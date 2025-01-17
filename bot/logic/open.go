package logic

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	database "github.com/jadevelopmentgrp/Tickets-Database"
	permcache "github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	worker "github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/metrics/prometheus"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/metrics/statsd"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/redis"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/channel"
	"github.com/rxdn/gdl/objects/channel/message"
	"github.com/rxdn/gdl/objects/interaction/component"
	"github.com/rxdn/gdl/objects/member"
	"github.com/rxdn/gdl/objects/user"
	"github.com/rxdn/gdl/permission"
	"github.com/rxdn/gdl/rest"
	"github.com/rxdn/gdl/rest/request"
	"golang.org/x/sync/errgroup"
)

func OpenTicket(ctx context.Context, cmd registry.InteractionContext, panel *database.Panel, subject string, formData map[database.FormInput]string) (database.Ticket, error) {
	lockCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	mu, err := redis.TakeTicketOpenLock(lockCtx, cmd.GuildId())
	if err != nil {
		cmd.HandleError(err)
		return database.Ticket{}, err
	}

	unlocked := false
	defer func() {
		if !unlocked {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			if _, err := mu.UnlockContext(ctx); err != nil {
				cmd.HandleError(err)
			}
		}
	}()

	// Make sure ticket count is within ticket limit
	// Check ticket limit before ratelimit token to prevent 1 person from stopping everyone opening tickets
	violatesTicketLimit, limit := getTicketLimit(ctx, cmd)
	if violatesTicketLimit {
		// Notify the user
		ticketsPluralised := "ticket"
		if limit > 1 {
			ticketsPluralised += "s"
		}

		// TODO: Use translation of tickets
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageTicketLimitReached, limit, ticketsPluralised)
		return database.Ticket{}, fmt.Errorf("ticket limit reached")
	}

	ok, err := redis.TakeTicketRateLimitToken(redis.Client, cmd.GuildId())
	if err != nil {
		cmd.HandleError(err)
		return database.Ticket{}, err
	}

	if !ok {
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageOpenRatelimited)
		return database.Ticket{}, nil
	}

	// Ensure that the panel isn't disabled
	if panel != nil && panel.ForceDisabled {
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageOpenPanelForceDisabled)
		return database.Ticket{}, nil
	}

	if panel != nil && panel.Disabled {
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageOpenPanelDisabled)
		return database.Ticket{}, nil
	}

	if panel != nil {
		member, err := cmd.Member()
		if err != nil {
			cmd.HandleError(err)
			return database.Ticket{}, err
		}

		matchedRole, action, err := dbclient.Client.PanelAccessControlRules.GetFirstMatched(
			ctx,
			panel.PanelId,
			append(member.Roles, cmd.GuildId()),
		)

		if err != nil {
			cmd.HandleError(err)
			return database.Ticket{}, err
		}

		if action == database.AccessControlActionDeny {
			if err := sendAccessControlDeniedMessage(ctx, cmd, panel.PanelId, matchedRole); err != nil {
				cmd.HandleError(err)
				return database.Ticket{}, err
			}

			return database.Ticket{}, nil
		} else if action != database.AccessControlActionAllow {
			cmd.HandleError(fmt.Errorf("invalid access control action %s", action))
			return database.Ticket{}, err
		}
	}

	settings, err := cmd.Settings()
	if err != nil {
		cmd.HandleError(err)
		return database.Ticket{}, err
	}

	isThread := settings.UseThreads

	// Check if the parent channel is an announcement channel
	if isThread {
		panelChannel, err := cmd.Channel()
		if err != nil {
			cmd.HandleError(err)
			return database.Ticket{}, err
		}

		if panelChannel.Type != channel.ChannelTypeGuildText {
			cmd.Reply(customisation.Red, i18n.Error, i18n.MessageOpenThreadAnnouncementChannel)
			return database.Ticket{}, nil
		}
	}

	// Check if the user has Send Messages in Threads
	if isThread && cmd.InteractionMetadata().Member != nil {
		member := cmd.InteractionMetadata().Member
		if member.Permissions > 0 && !permission.HasPermissionRaw(member.Permissions, permission.SendMessagesInThreads) {
			cmd.Reply(customisation.Red, i18n.Error, i18n.MessageOpenCantMessageInThreads)
			return database.Ticket{}, nil
		}
	}

	// If we're using a panel, then we need to create the ticket in the specified category
	var category uint64
	if panel != nil && panel.TargetCategory != 0 {
		category = panel.TargetCategory
	} else { // else we can just use the default category
		var err error
		category, err = dbclient.Client.ChannelCategory.Get(ctx, cmd.GuildId())
		if err != nil {
			cmd.HandleError(err)
			return database.Ticket{}, err
		}
	}

	useCategory := category != 0 && !isThread
	if useCategory {
		// Check if the category still exists
		_, err := cmd.Worker().GetChannel(category)
		if err != nil {
			useCategory = false

			if restError, ok := err.(request.RestError); ok && restError.StatusCode == 404 {
				if panel == nil {
					if err := dbclient.Client.ChannelCategory.Delete(ctx, cmd.GuildId()); err != nil {
						cmd.HandleError(err)
					}
				} // TODO: Else, set panel category to 0
			}
		}
	}

	// Generate subject
	if panel != nil && panel.Title != "" { // If we're using a panel, use the panel title as the subject
		subject = panel.Title
	} else { // Else, take command args as the subject
		if subject == "" {
			subject = "No subject given"
		}

		if len(subject) > 256 {
			subject = subject[0:255]
		}
	}

	// Channel count checks
	if !isThread {
		newCategoryId, err := checkChannelLimitAndDetermineParentId(ctx, cmd.Worker(), cmd.GuildId(), category, settings, true)
		if err != nil {
			if errors.Is(err, errGuildChannelLimitReached) {
				cmd.Reply(customisation.Red, i18n.Error, i18n.MessageGuildChannelLimitReached)
			} else if errors.Is(err, errCategoryChannelLimitReached) {
				cmd.Reply(customisation.Red, i18n.Error, i18n.MessageTooManyTickets)
			} else {
				cmd.HandleError(err)
			}

			return database.Ticket{}, err
		}

		category = newCategoryId
	}

	var panelId *int
	if panel != nil {
		panelId = &panel.PanelId
	}

	// Create channel
	ticketId, err := dbclient.Client.Tickets.Create(ctx, cmd.GuildId(), cmd.UserId(), isThread, panelId)
	if err != nil {
		cmd.HandleError(err)
		return database.Ticket{}, err
	}

	unlocked = true
	if _, err := mu.UnlockContext(ctx); err != nil && !errors.Is(err, redis.ErrLockExpired) {
		cmd.HandleError(err)
		return database.Ticket{}, err
	}

	name, err := GenerateChannelName(ctx, cmd, panel, ticketId, cmd.UserId(), nil)
	if err != nil {
		cmd.HandleError(err)
		return database.Ticket{}, err
	}

	var ch channel.Channel
	var joinMessageId *uint64
	if isThread {
		ch, err = cmd.Worker().CreatePrivateThread(cmd.ChannelId(), name, uint16(settings.ThreadArchiveDuration), false)
		if err != nil {
			cmd.HandleError(err)

			// To prevent tickets getting in a glitched state, we should mark it as closed (or delete it completely?)
			if err := dbclient.Client.Tickets.Close(ctx, ticketId, cmd.GuildId()); err != nil {
				cmd.HandleError(err)
			}

			return database.Ticket{}, err
		}

		// Join ticket
		if err := cmd.Worker().AddThreadMember(ch.Id, cmd.UserId()); err != nil {
			cmd.HandleError(err)
		}

		if settings.TicketNotificationChannel != nil {

			data := BuildJoinThreadMessage(ctx, cmd.Worker(), cmd.GuildId(), cmd.UserId(), ticketId, panel, nil)

			// TODO: Check if channel exists
			if msg, err := cmd.Worker().CreateMessageComplex(*settings.TicketNotificationChannel, data.IntoCreateMessageData()); err == nil {
				joinMessageId = &msg.Id
			} else {
				cmd.HandleError(err)
			}
		}
	} else {
		overwrites, err := CreateOverwrites(ctx, cmd, cmd.UserId(), panel)
		if err != nil {
			cmd.HandleError(err)
			return database.Ticket{}, err
		}

		data := rest.CreateChannelData{
			Name:                 name,
			Type:                 channel.ChannelTypeGuildText,
			Topic:                subject,
			PermissionOverwrites: overwrites,
		}

		if useCategory {
			data.ParentId = category
		}

		tmp, err := cmd.Worker().CreateGuildChannel(cmd.GuildId(), data)
		if err != nil { // Bot likely doesn't have permission
			// To prevent tickets getting in a glitched state, we should mark it as closed (or delete it completely?)
			if err := dbclient.Client.Tickets.Close(ctx, ticketId, cmd.GuildId()); err != nil {
				cmd.HandleError(err)
			}

			cmd.HandleError(err)

			var restError request.RestError
			if errors.As(err, &restError) && restError.ApiError.FirstErrorCode() == "CHANNEL_PARENT_MAX_CHANNELS" {
				canRefresh, err := redis.TakeChannelRefetchToken(ctx, cmd.GuildId())
				if err != nil {
					cmd.HandleError(err)
					return database.Ticket{}, err
				}

				if canRefresh {
					if err := refreshCachedChannels(ctx, cmd.Worker(), cmd.GuildId()); err != nil {
						cmd.HandleError(err)
						return database.Ticket{}, err
					}
				}
			}

			return database.Ticket{}, err
		}

		// TODO: Remove
		if tmp.Id == 0 {
			cmd.HandleError(fmt.Errorf("channel id is 0"))
			return database.Ticket{}, fmt.Errorf("channel id is 0")
		}

		ch = tmp
	}

	if err := dbclient.Client.Tickets.SetChannelId(ctx, cmd.GuildId(), ticketId, ch.Id); err != nil {
		cmd.HandleError(err)
		return database.Ticket{}, err
	}

	prometheus.TicketsCreated.Inc()

	// Parallelise as much as possible
	group, _ := errgroup.WithContext(ctx)

	// Let the user know the ticket has been opened
	group.Go(func() error {
		cmd.Reply(customisation.Green, i18n.Ticket, i18n.MessageTicketOpened, ch.Mention())
		return nil
	})

	// WelcomeMessageId is modified in the welcome message goroutine
	ticket := database.Ticket{
		Id:               ticketId,
		GuildId:          cmd.GuildId(),
		ChannelId:        &ch.Id,
		UserId:           cmd.UserId(),
		Open:             true,
		OpenTime:         time.Now(), // will be a bit off, but not used
		WelcomeMessageId: nil,
		PanelId:          panelId,
		IsThread:         isThread,
		JoinMessageId:    joinMessageId,
	}

	// Welcome message
	group.Go(func() error {

		externalPlaceholderCtx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()

		additionalPlaceholders, err := fetchCustomIntegrationPlaceholders(externalPlaceholderCtx, ticket, formAnswersToMap(formData))
		if err != nil {
			// TODO: Log for integration author and server owner on the dashboard, rather than spitting out a message.
			// A failing integration should not block the ticket creation process.
			cmd.HandleError(err)
		}

		welcomeMessageId, err := SendWelcomeMessage(ctx, cmd, ticket, subject, panel, formData, additionalPlaceholders)
		if err != nil {
			return err
		}

		// Update message IDs in DB
		if err := dbclient.Client.Tickets.SetMessageIds(ctx, cmd.GuildId(), ticketId, welcomeMessageId, joinMessageId); err != nil {
			return err
		}

		return nil
	})

	// Send mentions
	group.Go(func() error {
		metadata, err := dbclient.Client.GuildMetadata.Get(ctx, cmd.GuildId())
		if err != nil {
			return err
		}

		// mentions
		var content string

		// Append on-call role pings
		if isThread {
			if panel == nil {
				if metadata.OnCallRole != nil {
					content += fmt.Sprintf("<@&%d>", *metadata.OnCallRole)
				}
			} else {
				if panel.WithDefaultTeam && metadata.OnCallRole != nil {
					content += fmt.Sprintf("<@&%d>", *metadata.OnCallRole)
				}

				teams, err := dbclient.Client.PanelTeams.GetTeams(ctx, panel.PanelId)
				if err != nil {
					return err
				} else {
					for _, team := range teams {
						if team.OnCallRole != nil {
							content += fmt.Sprintf("<@&%d>", *team.OnCallRole)
						}
					}
				}
			}
		}

		if panel != nil {
			// roles
			roles, err := dbclient.Client.PanelRoleMentions.GetRoles(ctx, panel.PanelId)
			if err != nil {
				return err
			} else {
				for _, roleId := range roles {
					if roleId == cmd.GuildId() {
						content += "@everyone"
					} else {
						content += fmt.Sprintf("<@&%d>", roleId)
					}
				}
			}

			// user
			shouldMentionUser, err := dbclient.Client.PanelUserMention.ShouldMentionUser(ctx, panel.PanelId)
			if err != nil {
				return err
			} else {
				if shouldMentionUser {
					content += fmt.Sprintf("<@%d>", cmd.UserId())
				}
			}
		}

		if content != "" {
			if len(content) > 2000 {
				content = content[:2000]
			}

			pingMessage, err := cmd.Worker().CreateMessageComplex(ch.Id, rest.CreateMessageData{
				Content: content,
				AllowedMentions: message.AllowedMention{
					Parse: []message.AllowedMentionType{
						message.EVERYONE,
						message.USERS,
						message.ROLES,
					},
				},
			})

			if err != nil {
				return err
			}

			// error is likely to be a permission error
			_ = cmd.Worker().DeleteMessage(ch.Id, pingMessage.Id)
		}

		return nil
	})

	// Create webhook
	// TODO: Create webhook on use, rather than on ticket creation.
	// TODO: Webhooks for threads should be created on the parent channel.
	if !ticket.IsThread {
		group.Go(func() error {
			return createWebhook(ctx, cmd, ticketId, cmd.GuildId(), ch.Id)
		})
	}

	if err := group.Wait(); err != nil {
		cmd.HandleError(err)
		return database.Ticket{}, err
	}

	statsd.Client.IncrementKey(statsd.KeyTickets)
	if panel == nil {
		statsd.Client.IncrementKey(statsd.KeyOpenCommand)
	}

	return ticket, nil
}

var (
	errGuildChannelLimitReached    = errors.New("guild channel limit reached")
	errCategoryChannelLimitReached = errors.New("category channel limit reached")
)

func checkChannelLimitAndDetermineParentId(
	ctx context.Context,
	worker *worker.Context,
	guildId uint64,
	categoryId uint64,
	settings database.Settings,
	canRetry bool,
) (uint64, error) {
	channels, _ := worker.GetGuildChannels(guildId)

	// 500 guild limit check
	if countRealChannels(channels, 0) >= 500 {
		if !canRetry {
			return 0, errGuildChannelLimitReached
		} else {
			canRefresh, err := redis.TakeChannelRefetchToken(ctx, guildId)
			if err != nil {
				return 0, err
			}

			if canRefresh {
				if err := refreshCachedChannels(ctx, worker, guildId); err != nil {
					return 0, err
				}

				return checkChannelLimitAndDetermineParentId(ctx, worker, guildId, categoryId, settings, false)
			} else {
				return 0, errGuildChannelLimitReached
			}
		}
	}

	// Make sure there's not > 50 channels in a category
	if categoryId != 0 {
		categoryChildrenCount := countRealChannels(channels, categoryId)

		if categoryChildrenCount >= 50 {
			if canRetry {
				canRefresh, err := redis.TakeChannelRefetchToken(ctx, guildId)
				if err != nil {
					return 0, err
				}

				if canRefresh {
					if err := refreshCachedChannels(ctx, worker, guildId); err != nil {
						return 0, err
					}

					return checkChannelLimitAndDetermineParentId(ctx, worker, guildId, categoryId, settings, false)
				} else {
					return 0, errCategoryChannelLimitReached
				}
			}

			// Try to use the overflow category if there is one
			if settings.OverflowEnabled {
				// If overflow is enabled, and the category id is nil, then use the root of the server
				if settings.OverflowCategoryId == nil {
					categoryId = 0
				} else {
					categoryId = *settings.OverflowCategoryId

					// Verify that the overflow category still exists
					if !utils.ContainsFunc(channels, func(c channel.Channel) bool {
						return c.Id == categoryId
					}) {
						if err := dbclient.Client.Settings.SetOverflow(ctx, guildId, false, nil); err != nil {
							return 0, err
						}

						return 0, errCategoryChannelLimitReached
					}

					// Check that the overflow category still has space
					overflowCategoryChildrenCount := countRealChannels(channels, *settings.OverflowCategoryId)
					if overflowCategoryChildrenCount >= 50 {
						return 0, errCategoryChannelLimitReached
					}

				}
			} else {
				return 0, errCategoryChannelLimitReached
			}
		}
	}

	return categoryId, nil
}

func refreshCachedChannels(ctx context.Context, worker *worker.Context, guildId uint64) error {
	channels, err := rest.GetGuildChannels(ctx, worker.Token, worker.RateLimiter, guildId)
	if err != nil {
		return err
	}

	return worker.Cache.ReplaceChannels(ctx, guildId, channels)
}

// has hit ticket limit, ticket limit
func getTicketLimit(ctx context.Context, cmd registry.CommandContext) (bool, int) {
	isStaff, err := cmd.UserPermissionLevel(ctx)
	if err != nil {
		fmt.Print(err, cmd.ToErrorContext())
		return true, 1 // TODO: Stop flow
	}

	if isStaff >= permcache.Support {
		return false, 50
	}

	var openedTickets []database.Ticket
	var ticketLimit uint8

	group, _ := errgroup.WithContext(ctx)

	// get ticket limit
	group.Go(func() (err error) {
		ticketLimit, err = dbclient.Client.TicketLimit.Get(ctx, cmd.GuildId())
		return
	})

	group.Go(func() (err error) {
		openedTickets, err = dbclient.Client.Tickets.GetOpenByUser(ctx, cmd.GuildId(), cmd.UserId())
		return
	})

	if err := group.Wait(); err != nil {
		fmt.Print(err, cmd.ToErrorContext())
		return true, 1
	}

	return len(openedTickets) >= int(ticketLimit), int(ticketLimit)
}

func createWebhook(ctx context.Context, c registry.CommandContext, ticketId int, guildId, channelId uint64) error {
	// TODO: Re-add permission check
	//if permission.HasPermissionsChannel(ctx.Shard, ctx.GuildId, ctx.Shard.SelfId(), channelId, permission.ManageWebhooks) { // Do we actually need this?

	self, err := c.Worker().Self()
	if err != nil {
		return err
	}

	data := rest.WebhookData{
		Username: self.Username,
		Avatar:   self.AvatarUrl(256),
	}

	webhook, err := c.Worker().CreateWebhook(channelId, data)
	if err != nil {
		fmt.Print(err, c.ToErrorContext())
		return nil // Silently fail
	}

	dbWebhook := database.Webhook{
		Id:    webhook.Id,
		Token: webhook.Token,
	}

	if err := dbclient.Client.Webhooks.Create(ctx, guildId, ticketId, dbWebhook); err != nil {
		return err
	}

	return nil
}

func CreateOverwrites(ctx context.Context, cmd registry.InteractionContext, userId uint64, panel *database.Panel, otherUsers ...uint64) ([]channel.PermissionOverwrite, error) {
	overwrites := []channel.PermissionOverwrite{ // @everyone
		{
			Id:    cmd.GuildId(),
			Type:  channel.PermissionTypeRole,
			Allow: 0,
			Deny:  permission.BuildPermissions(permission.ViewChannel),
		},
	}

	// Build permissions
	additionalPermissions, err := dbclient.Client.TicketPermissions.Get(ctx, cmd.GuildId())
	if err != nil {
		return nil, err
	}

	// Separate permissions apply
	for _, snowflake := range append(otherUsers, userId) {
		overwrites = append(overwrites, BuildUserOverwrite(snowflake, additionalPermissions))
	}

	// Add the bot to the overwrites
	selfAllow := make([]permission.Permission, len(StandardPermissions), len(StandardPermissions)+1)
	copy(selfAllow, StandardPermissions[:]) // Do not append to StandardPermissions

	if permission.HasPermissionRaw(cmd.InteractionMetadata().AppPermissions, permission.ManageWebhooks) {
		selfAllow = append(selfAllow, permission.ManageWebhooks)
	}

	integrationRoleId, err := GetIntegrationRoleId(ctx, cmd.Worker(), cmd.GuildId())
	if err != nil {
		return nil, err
	}

	if integrationRoleId == nil {
		overwrites = append(overwrites, channel.PermissionOverwrite{
			Id:    cmd.Worker().BotId,
			Type:  channel.PermissionTypeMember,
			Allow: permission.BuildPermissions(selfAllow[:]...),
			Deny:  0,
		})
	} else {
		overwrites = append(overwrites, channel.PermissionOverwrite{
			Id:    *integrationRoleId,
			Type:  channel.PermissionTypeRole,
			Allow: permission.BuildPermissions(selfAllow[:]...),
			Deny:  0,
		})
	}

	// Create list of members & roles who should be added to the ticket
	allowedUsers, allowedRoles, err := GetAllowedStaffUsersAndRoles(ctx, cmd.GuildId(), panel)
	if err != nil {
		return nil, err
	}

	for _, member := range allowedUsers {
		allow := make([]permission.Permission, len(StandardPermissions))
		copy(allow, StandardPermissions[:]) // Do not append to StandardPermissions

		if member == cmd.Worker().BotId {
			continue // Already added overwrite above
		}

		overwrites = append(overwrites, channel.PermissionOverwrite{
			Id:    member,
			Type:  channel.PermissionTypeMember,
			Allow: permission.BuildPermissions(allow...),
			Deny:  0,
		})
	}

	for _, role := range allowedRoles {
		overwrites = append(overwrites, channel.PermissionOverwrite{
			Id:    role,
			Type:  channel.PermissionTypeRole,
			Allow: permission.BuildPermissions(StandardPermissions[:]...),
			Deny:  0,
		})
	}

	return overwrites, nil
}

func GetAllowedStaffUsersAndRoles(ctx context.Context, guildId uint64, panel *database.Panel) ([]uint64, []uint64, error) {
	// Create list of members & roles who should be added to the ticket
	// Add the sender & self
	allowedUsers := make([]uint64, 0)
	allowedRoles := make([]uint64, 0)

	// Should we add the default team
	if panel == nil || panel.WithDefaultTeam {
		// Get support reps & admins
		supportUsers, err := dbclient.Client.Permissions.GetSupport(ctx, guildId)
		if err != nil {
			return nil, nil, err
		}

		allowedUsers = append(allowedUsers, supportUsers...)

		// Get support roles & admin roles
		supportRoles, err := dbclient.Client.RolePermissions.GetSupportRoles(ctx, guildId)
		if err != nil {
			return nil, nil, err
		}

		allowedRoles = append(allowedUsers, supportRoles...)
	}

	// Add other support teams
	if panel != nil {
		group, _ := errgroup.WithContext(ctx)

		// Get users for support teams of panel
		group.Go(func() error {
			userIds, err := dbclient.Client.SupportTeamMembers.GetAllSupportMembersForPanel(ctx, panel.PanelId)
			if err != nil {
				return err
			}

			allowedUsers = append(allowedUsers, userIds...) // No mutex needed
			return nil
		})

		// Get roles for support teams of panel
		group.Go(func() error {
			roleIds, err := dbclient.Client.SupportTeamRoles.GetAllSupportRolesForPanel(ctx, panel.PanelId)
			if err != nil {
				return err
			}

			allowedRoles = append(allowedRoles, roleIds...) // No mutex needed
			return nil
		})

		if err := group.Wait(); err != nil {
			return nil, nil, err
		}
	}

	return allowedUsers, allowedRoles, nil
}

func GetIntegrationRoleId(rootCtx context.Context, worker *worker.Context, guildId uint64) (*uint64, error) {
	ctx, cancel := context.WithTimeout(rootCtx, time.Second*3)
	defer cancel()

	cachedId, err := redis.GetIntegrationRole(ctx, guildId, worker.BotId)
	if err == nil {
		return &cachedId, nil
	} else if !errors.Is(err, redis.ErrIntegrationRoleNotCached) {
		return nil, err
	}

	roles, err := worker.GetGuildRoles(guildId)
	if err != nil {
		return nil, err
	}

	for _, role := range roles {
		if role.Tags.BotId != nil && *role.Tags.BotId == worker.BotId {
			ctx, cancel := context.WithTimeout(rootCtx, time.Second*3)
			defer cancel() // defer is okay here as we return in every case

			if err := redis.SetIntegrationRole(ctx, guildId, worker.BotId, role.Id); err != nil {
				return nil, err
			}

			return &role.Id, nil
		}
	}

	return nil, nil
}

func GenerateChannelName(ctx context.Context, cmd registry.CommandContext, panel *database.Panel, ticketId int, openerId uint64, claimer *uint64) (string, error) {
	// Create ticket name
	var name string

	// Use server default naming scheme
	if panel == nil || panel.NamingScheme == nil {
		namingScheme, err := dbclient.Client.NamingScheme.Get(ctx, cmd.GuildId())
		if err != nil {
			return "", err
		}

		strTicket := strings.ToLower(cmd.GetMessage(i18n.Ticket))
		if namingScheme == database.Username {
			var user user.User
			if cmd.UserId() == openerId {
				user, err = cmd.User()
			} else {
				user, err = cmd.Worker().GetUser(openerId)
			}

			if err != nil {
				return "", err
			}

			name = fmt.Sprintf("%s-%s", strTicket, user.Username)
		} else {
			name = fmt.Sprintf("%s-%d", strTicket, ticketId)
		}
	} else {
		var err error
		name, err = doSubstitutions(cmd, *panel.NamingScheme, openerId, []Substitutor{
			// %id%
			NewSubstitutor("id", false, false, func(user user.User, member member.Member) string {
				return strconv.Itoa(ticketId)
			}),
			// %id_padded%
			NewSubstitutor("id_padded", false, false, func(user user.User, member member.Member) string {
				return fmt.Sprintf("%04d", ticketId)
			}),
			// %claimed%
			NewSubstitutor("claimed", false, false, func(user user.User, member member.Member) string {
				if claimer == nil {
					return "unclaimed"
				} else {
					return "claimed"
				}
			}),
			// %username%
			NewSubstitutor("username", true, false, func(user user.User, member member.Member) string {
				return user.Username
			}),
			// %nickname%
			NewSubstitutor("nickname", false, true, func(user user.User, member member.Member) string {
				nickname := member.Nick
				if len(nickname) == 0 {
					nickname = member.User.Username
				}

				return nickname
			}),
		})

		if err != nil {
			return "", err
		}
	}

	// Cap length after substitutions
	if len(name) > 100 {
		name = name[:100]
	}

	return name, nil
}

func countRealChannels(channels []channel.Channel, parentId uint64) int {
	var count int

	for _, ch := range channels {
		// Ignore threads
		if ch.Type == channel.ChannelTypeGuildPublicThread || ch.Type == channel.ChannelTypeGuildPrivateThread || ch.Type == channel.ChannelTypeGuildNewsThread {
			continue
		}

		if parentId == 0 || ch.ParentId.Value == parentId {
			count++
		}
	}

	return count
}

func BuildJoinThreadMessage(
	ctx context.Context,
	worker *worker.Context,
	guildId, openerId uint64,
	ticketId int,
	panel *database.Panel,
	staffMembers []uint64,
) command.MessageResponse {
	return buildJoinThreadMessage(ctx, worker, guildId, openerId, ticketId, panel, staffMembers, false)
}

func BuildThreadReopenMessage(
	ctx context.Context,
	worker *worker.Context,
	guildId, openerId uint64,
	ticketId int,
	panel *database.Panel,
	staffMembers []uint64,
) command.MessageResponse {
	return buildJoinThreadMessage(ctx, worker, guildId, openerId, ticketId, panel, staffMembers, true)
}

// TODO: Translations
func buildJoinThreadMessage(
	ctx context.Context,
	worker *worker.Context,
	guildId, openerId uint64,
	ticketId int,
	panel *database.Panel,
	staffMembers []uint64,
	fromReopen bool,
) command.MessageResponse {
	var colour customisation.Colour
	if len(staffMembers) > 0 {
		colour = customisation.Green
	} else {
		colour = customisation.Red
	}

	panelName := "None"
	if panel != nil {
		panelName = panel.ButtonLabel
	}

	title := "Join Ticket"
	if fromReopen {
		title = "Ticket Reopened"
	}

	e := utils.BuildEmbedRaw(customisation.GetColourOrDefault(ctx, guildId, colour), title, "A ticket has been opened. Press the button below to join it.", nil)
	e.AddField(customisation.PrefixWithEmoji("Opened By", customisation.EmojiOpen, !worker.IsWhitelabel), customisation.PrefixWithEmoji(fmt.Sprintf("<@%d>", openerId), customisation.EmojiBulletLine, !worker.IsWhitelabel), true)
	e.AddField(customisation.PrefixWithEmoji("Panel", customisation.EmojiPanel, !worker.IsWhitelabel), customisation.PrefixWithEmoji(panelName, customisation.EmojiBulletLine, !worker.IsWhitelabel), true)
	e.AddField(customisation.PrefixWithEmoji("Staff In Ticket", customisation.EmojiStaff, !worker.IsWhitelabel), customisation.PrefixWithEmoji(strconv.Itoa(len(staffMembers)), customisation.EmojiBulletLine, !worker.IsWhitelabel), true)

	if len(staffMembers) > 0 {
		var mentions []string // dynamic length
		charCount := len(customisation.EmojiBulletLine.String()) + 1
		for _, staffMember := range staffMembers {
			mention := fmt.Sprintf("<@%d>", staffMember)

			if charCount+len(mention)+1 > 1024 {
				break
			}

			mentions = append(mentions, mention)
			charCount += len(mention) + 1 // +1 for space
		}

		e.AddField(customisation.PrefixWithEmoji("Staff Members", customisation.EmojiStaff, !worker.IsWhitelabel), customisation.PrefixWithEmoji(strings.Join(mentions, " "), customisation.EmojiBulletLine, !worker.IsWhitelabel), false)
	}

	return command.MessageResponse{
		Embeds: utils.Slice(e),
		Components: utils.Slice(component.BuildActionRow(
			component.BuildButton(component.Button{
				Label:    "Join Ticket",
				CustomId: fmt.Sprintf("join_thread_%d", ticketId),
				Style:    component.ButtonStylePrimary,
				Emoji:    utils.BuildEmoji("➕"),
			}),
		)),
	}
}

func sendAccessControlDeniedMessage(ctx context.Context, cmd registry.InteractionContext, panelId int, matchedRole uint64) error {
	rules, err := dbclient.Client.PanelAccessControlRules.GetAll(ctx, panelId)
	if err != nil {
		return err
	}

	allowedRoleIds := make([]uint64, 0, len(rules))
	for _, rule := range rules {
		if rule.Action == database.AccessControlActionAllow {
			allowedRoleIds = append(allowedRoleIds, rule.RoleId)
		}
	}

	if len(allowedRoleIds) == 0 {
		cmd.Reply(customisation.Red, i18n.MessageNoPermission, i18n.MessageOpenAclNoAllowRules)
		return nil
	}

	if matchedRole == cmd.GuildId() {
		mentions := make([]string, 0, len(allowedRoleIds))
		for _, roleId := range allowedRoleIds {
			mentions = append(mentions, fmt.Sprintf("<@&%d>", roleId))
		}

		if len(allowedRoleIds) == 1 {
			cmd.Reply(customisation.Red, i18n.MessageNoPermission, i18n.MessageOpenAclNotAllowListedSingle, strings.Join(mentions, ", "))
		} else {
			cmd.Reply(customisation.Red, i18n.MessageNoPermission, i18n.MessageOpenAclNotAllowListedMultiple, strings.Join(mentions, ", "))
		}
	} else {
		cmd.Reply(customisation.Red, i18n.MessageNoPermission, i18n.MessageOpenAclDenyListed, matchedRole)
	}

	return nil
}

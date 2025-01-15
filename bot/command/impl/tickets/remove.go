package tickets

import (
	"time"

	permcache "github.com/TicketsBot/common/permission"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/logic"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/channel"
	"github.com/rxdn/gdl/objects/interaction"
	"github.com/rxdn/gdl/permission"
)

type RemoveCommand struct {
}

func (RemoveCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "remove",
		Description:     i18n.HelpRemove,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permcache.Everyone,
		Category:        command.Tickets,
		Arguments: command.Arguments(
			command.NewRequiredArgument("user", "User to remove from the current ticket", interaction.OptionTypeUser, i18n.MessageRemoveAdminNoMembers),
		),
		Timeout: time.Second * 8,
	}
}

func (c RemoveCommand) GetExecutor() interface{} {
	return c.Execute
}

func (RemoveCommand) Execute(ctx registry.CommandContext, userId uint64) {
	// Get ticket struct
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Verify that the current channel is a real ticket
	if ticket.UserId == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	selfPermissionLevel, err := ctx.UserPermissionLevel(ctx)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Verify that the user is allowed to modify the ticket
	if selfPermissionLevel == permcache.Everyone && ticket.UserId != ctx.UserId() {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveNoPermission)
		return
	}

	// verify that the user isn't trying to remove staff
	member, err := ctx.Worker().GetGuildMember(ctx.GuildId(), userId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	permissionLevel, err := permcache.GetPermissionLevel(ctx, utils.ToRetriever(ctx.Worker()), member, ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if permissionLevel > permcache.Everyone {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveCannotRemoveStaff)
		return
	}

	// Remove user from ticket in DB
	if err := dbclient.Client.TicketMembers.Delete(ctx, ctx.GuildId(), ticket.Id, userId); err != nil {
		ctx.HandleError(err)
		return
	}

	// Remove user from ticket
	if ticket.IsThread {
		if err := ctx.Worker().RemoveThreadMember(ctx.ChannelId(), userId); err != nil {
			ctx.HandleError(err)
			return
		}
	} else {
		data := channel.PermissionOverwrite{
			Id:    userId,
			Type:  channel.PermissionTypeMember,
			Allow: 0,
			Deny:  permission.BuildPermissions(logic.StandardPermissions[:]...),
		}

		if err := ctx.Worker().EditChannelPermissions(ctx.ChannelId(), data); err != nil {
			ctx.HandleError(err)
			return
		}
	}

	ctx.ReplyPermanent(customisation.Green, i18n.TitleRemove, i18n.MessageRemoveSuccess, userId, ctx.ChannelId())
}

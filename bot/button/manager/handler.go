package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	worker "github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/blacklist"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry"
	cmdcontext "github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	cmdregistry "github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/interaction"
	"github.com/rxdn/gdl/objects/interaction/component"
)

// Returns whether the handler may edit the message
func HandleInteraction(ctx context.Context, manager *ComponentInteractionManager, worker *worker.Context, data interaction.MessageComponentInteraction, responseCh chan button.Response) bool {
	// Safety checks - guild interactions only
	if data.GuildId.Value != 0 && data.Member == nil {
		return false
	}

	if data.GuildId.Value == 0 && data.User == nil {
		return false
	}

	lookupCtx, cancelLookupCtx := context.WithTimeout(ctx, time.Second*2)
	defer cancelLookupCtx()

	var cc cmdregistry.InteractionContext
	switch data.Data.Type() {
	case component.ComponentButton:
		cc = cmdcontext.NewButtonContext(ctx, worker, data, responseCh)
	case component.ComponentSelectMenu:
		cc = cmdcontext.NewSelectMenuContext(ctx, worker, data, responseCh)
	default:
		fmt.Errorf("invalid message component type: %d", data.Data.ComponentType)
		return false
	}

	// Check for guild-wide blacklist
	if data.GuildId.Value != 0 && blacklist.IsGuildBlacklisted(data.GuildId.Value) {
		cc.Reply(customisation.Red, i18n.TitleBlacklisted, i18n.MessageBlacklisted)
		return false
	}

	// Check not if the context has been cancelled
	if err := lookupCtx.Err(); err != nil {
		fmt.Print(err, data.GuildId.Value, data.ChannelId)

		cc.ReplyRaw(customisation.Red, "Error", fmt.Sprintf("An error occurred while processing this request."))
		return false
	}

	// Check if the user is blacklisted at guild / global level
	userBlacklisted, err := cc.IsBlacklisted(lookupCtx)
	if err != nil {
		fmt.Print(err, data.GuildId.Value, data.ChannelId)

		cc.ReplyRaw(customisation.Red, "Error", fmt.Sprintf("An error occurred while processing this request."))
		return false
	}

	if userBlacklisted {
		cc.Reply(customisation.Red, i18n.TitleBlacklisted, i18n.MessageBlacklisted)
		return false
	}

	checkCtx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	switch data.Data.Type() {
	case component.ComponentButton:
		handler := manager.MatchButton(data.Data.AsButton().CustomId)
		if handler == nil {
			return false
		}

		shouldExecute, canEdit := doPropertiesChecks(checkCtx, data.GuildId.Value, cc, handler.Properties())
		if shouldExecute {
			go func() {
				defer close(responseCh)

				cc := cc.(*cmdcontext.ButtonContext)

				var cancel context.CancelFunc
				cc.Context, cancel = context.WithTimeout(cc.Context, handler.Properties().Timeout)
				defer cancel()

				handler.Execute(cc)
			}()
		}

		return canEdit
	case component.ComponentSelectMenu:
		handler := manager.MatchSelect(data.Data.AsSelectMenu().CustomId)
		if handler == nil {
			return false
		}

		shouldExecute, canEdit := doPropertiesChecks(checkCtx, data.GuildId.Value, cc, handler.Properties())
		if shouldExecute {
			go func() {
				defer close(responseCh)

				cc := cc.(*cmdcontext.SelectMenuContext)

				var cancel context.CancelFunc
				cc.Context, cancel = context.WithTimeout(cc.Context, handler.Properties().Timeout)
				defer cancel()

				handler.Execute(cc)
			}()
		}

		return canEdit
	default:
		fmt.Errorf("invalid message component type: %d", data.Data.ComponentType)
		return false
	}
}

func doPropertiesChecks(ctx context.Context, guildId uint64, cmd cmdregistry.CommandContext, properties registry.Properties) (shouldExecute, canEdit bool) {
	if properties.PermissionLevel > permission.Everyone {
		permLevel, err := cmd.UserPermissionLevel(ctx)
		if err != nil {
			fmt.Print(err, cmd.ToErrorContext())
			return false, false
		}

		if permLevel < properties.PermissionLevel {
			cmd.Reply(customisation.Red, i18n.Error, i18n.MessageNoPermission)
			return false, false
		}
	}

	if guildId == 0 && !properties.HasFlag(registry.DMsAllowed) {
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageButtonGuildOnly)
		return false, false
	}

	if guildId != 0 && !properties.HasFlag(registry.GuildAllowed) {
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageButtonDMOnly)
		return false, false
	}

	return true, properties.HasFlag(registry.CanEdit)
}

package handlers

import (
	"fmt"
	"github.com/TicketsBot/common/permission"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry/matcher"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/constants"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/logic"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/interaction/component"
)

type ClaimHandler struct{}

func (h *ClaimHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "claim",
	}
}

func (h *ClaimHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout: constants.TimeoutOpenTicket,
	}
}

func (h *ClaimHandler) Execute(ctx *context.ButtonContext) {
	// Get permission level
	permissionLevel, err := ctx.UserPermissionLevel(ctx)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if permissionLevel < permission.Support {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageClaimNoPermission)
		return
	}

	// Get ticket struct
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Verify this is a ticket channel
	if ticket.UserId == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	if err := logic.ClaimTicket(ctx.Context, ctx, ticket, ctx.UserId()); err != nil {
		ctx.HandleError(err)
		return
	}

	res := command.MessageIntoMessageResponse(ctx.Interaction.Message)
	if len(res.Components) > 0 && res.Components[0].Type == component.ComponentActionRow {
		row := res.Components[0].ComponentData.(component.ActionRow)
		if len(row.Components) > 1 {
			row.Components = row.Components[:len(row.Components)-1]
		}

		res.Components[0] = component.Component{
			Type:          component.ComponentActionRow,
			ComponentData: row,
		}
	}

	ctx.Edit(res)
	ctx.ReplyPermanent(customisation.Green, i18n.TitleClaimed, i18n.MessageClaimed, fmt.Sprintf("<@%d>", ctx.UserId()))
}

package handlers

import (
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry/matcher"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/constants"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/logic"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
)

type CloseRequestAcceptHandler struct{}

func (h *CloseRequestAcceptHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "close_request_accept",
	}
}

func (h *CloseRequestAcceptHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: constants.TimeoutCloseTicket,
	}
}

func (h *CloseRequestAcceptHandler) Execute(ctx *context.ButtonContext) {
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if ticket.Id == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	if ctx.UserId() != ticket.UserId {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageCloseRequestNoPermission)
		return
	}

	closeRequest, ok, err := dbclient.Client.CloseRequest.Get(ctx, ticket.GuildId, ticket.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Infallible, unless malicious
	if !ok {
		return
	}

	ctx.Edit(command.MessageResponse{
		Embeds: utils.Slice(utils.BuildEmbedRaw(customisation.DefaultColours[customisation.Green], "Close Request", "Closing ticket...", nil)),
	})

	// Avoid users cant close issue
	// Allow members to close too, for context menu tickets
	logic.CloseTicket(ctx.Context, ctx, closeRequest.Reason, true)
}

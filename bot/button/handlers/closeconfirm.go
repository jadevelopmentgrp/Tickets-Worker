package handlers

import (
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry/matcher"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/constants"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/logic"
)

type CloseConfirmHandler struct{}

func (h *CloseConfirmHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "close_confirm",
	}
}

func (h *CloseConfirmHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: constants.TimeoutCloseTicket,
	}
}

func (h *CloseConfirmHandler) Execute(ctx *context.ButtonContext) {
	// TODO: IntoPanelContext()?
	logic.CloseTicket(ctx.Context, ctx, nil, false)
}

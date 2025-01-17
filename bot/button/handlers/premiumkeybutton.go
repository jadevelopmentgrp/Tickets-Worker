package handlers

import (
	"time"

	"github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry/matcher"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	prem "github.com/jadevelopmentgrp/Tickets-Worker/bot/premium"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
)

type PremiumKeyButtonHandler struct{}

func (h *PremiumKeyButtonHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "open_premium_key_modal",
	}
}

func (h *PremiumKeyButtonHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: time.Second * 3,
	}
}

func (h *PremiumKeyButtonHandler) Execute(ctx *context.ButtonContext) {
	// Get permission level
	permissionLevel, err := ctx.UserPermissionLevel(ctx)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if permissionLevel < permission.Admin {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNoPermission)
		return
	}

	ctx.Modal(button.ResponseModal{
		Data: prem.BuildKeyModal(ctx.GuildId()),
	})
}

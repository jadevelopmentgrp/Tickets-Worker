package handlers

import (
	"time"

	"github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry/matcher"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
)

type PremiumCheckAgain struct{}

func (h *PremiumCheckAgain) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "premium_check_again",
	}
}

func (h *PremiumCheckAgain) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: time.Second * 5,
	}
}

func (h *PremiumCheckAgain) Execute(ctx *context.ButtonContext) {
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

	ctx.EditWith(customisation.Green, i18n.MessagePremiumChecking, i18n.MessagePremiumPleaseWait)

	if err := utils.PremiumClient.DeleteCachedTier(ctx, ctx.GuildId()); err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.EditWith(customisation.Green, i18n.Success, i18n.MessagePremiumSuccessAfterCheck)

	// Re-enable panels
	if err := dbclient.Client.Panel.EnableAll(ctx, ctx.GuildId()); err != nil {
		ctx.HandleError(err)
		return
	}
}

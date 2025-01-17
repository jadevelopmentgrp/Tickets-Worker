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
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/interaction/component"
)

type PremiumKeyOpenHandler struct{}

func (h *PremiumKeyOpenHandler) Matcher() matcher.Matcher {
	return matcher.NewSimpleMatcher("premium_purchase_method")
}

func (h *PremiumKeyOpenHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout: time.Second * 5,
	}
}

func (h *PremiumKeyOpenHandler) Execute(ctx *context.SelectMenuContext) {
	permLevel, err := ctx.UserPermissionLevel(ctx)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if permLevel < permission.Admin {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNoPermission)
		return
	}

	if len(ctx.InteractionData.Values) == 0 {
		return
	}

	ctx.Modal(button.ResponseModal{
		Data: prem.BuildKeyModal(ctx.GuildId()),
	})

	components := utils.Slice(component.BuildActionRow(component.BuildButton(component.Button{
		Label:    ctx.GetMessage(i18n.MessagePremiumOpenForm),
		CustomId: "open_premium_key_modal",
		Style:    component.ButtonStylePrimary,
		Emoji:    utils.BuildEmoji("🔑"),
	})))

	ctx.EditWithComponents(customisation.Green, i18n.TitlePremium, i18n.MessagePremiumOpenFormDescription, components)
}

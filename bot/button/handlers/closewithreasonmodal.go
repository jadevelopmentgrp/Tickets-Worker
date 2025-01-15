package handlers

import (
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry/matcher"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/interaction"
	"github.com/rxdn/gdl/objects/interaction/component"
	"time"
)

type CloseWithReasonModalHandler struct{}

func (h *CloseWithReasonModalHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "close_with_reason",
	}
}

func (h *CloseWithReasonModalHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: time.Second * 3,
	}
}

func (h *CloseWithReasonModalHandler) Execute(ctx *context.ButtonContext) {
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if ticket.Id == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	if !utils.CanClose(ctx.Context, ctx, ticket) {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageCloseNoPermission)
		return
	}

	ctx.Modal(button.ResponseModal{
		Data: interaction.ModalResponseData{
			CustomId: "close_with_reason_submit",
			Title:    i18n.TitleClose.GetFromGuild(ctx.GuildId()),
			Components: []component.Component{
				component.BuildActionRow(component.BuildInputText(component.InputText{
					Style:       component.TextStyleParagraph,
					CustomId:    "reason",
					Label:       i18n.Reason.GetFromGuild(ctx.GuildId()),
					Placeholder: utils.Ptr(i18n.MessageCloseReasonPlaceholder.GetFromGuild(ctx.GuildId())),
					MinLength:   nil,
					MaxLength:   utils.Ptr(uint32(1024)),
				})),
			},
		},
	})
}

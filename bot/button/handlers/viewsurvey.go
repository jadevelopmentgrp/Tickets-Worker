package handlers

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button/registry/matcher"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/logic"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/rxdn/gdl/objects/channel/embed"
	"github.com/rxdn/gdl/objects/interaction/component"
)

type ViewSurveyHandler struct{}

func (h *ViewSurveyHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "view-survey-")
	})
}

func (h *ViewSurveyHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed),
		PermissionLevel: permission.Support,
		Timeout:         time.Second * 5,
	}
}

var viewSurveyPattern = regexp.MustCompile(`view-survey-(\d+)-(\d+)`)

func (h *ViewSurveyHandler) Execute(ctx *context.ButtonContext) {
	groups := viewSurveyPattern.FindStringSubmatch(ctx.InteractionData.CustomId)
	if len(groups) != 3 {
		return
	}

	// Error may occur if guild ID in custom ID > max u64 size
	guildId, err := strconv.ParseUint(groups[1], 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	ticketId, err := strconv.Atoi(groups[2])
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Get ticket
	ticket, err := dbclient.Client.Tickets.Get(ctx, ticketId, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if ticket.GuildId != guildId || ticket.Id != ticketId {
		return
	}

	surveyResponse, err := dbclient.Client.ExitSurveyResponses.GetResponses(ctx, ticket.GuildId, ticket.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if len(surveyResponse.Responses) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "No survey surveyResponse has been recorded for this ticket.") // TODO: i18n
		return
	}

	opener, err := ctx.Worker().GetUser(ticket.UserId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	e := embed.NewEmbed().
		SetTitle("Exit Survey"). // TODO: i18n
		SetAuthor(opener.Username, "", opener.AvatarUrl(256)).
		SetColor(ctx.GetColour(customisation.Green))

	for _, answer := range surveyResponse.Responses {
		var title string
		if answer.Question == nil {
			title = "Unknown Question"
		} else {
			title = *answer.Question
		}

		var response string
		if len(answer.Response) > 0 {
			response = answer.Response
		} else {
			response = "No response"
		}

		e.AddField(title, response, false)
	}

	var buttons []component.Component
	buttons = append(buttons, logic.TranscriptLinkElement(ticket.HasTranscript)(ctx.Worker(), ticket)...)
	buttons = append(buttons, logic.ThreadLinkElement(ticket.ChannelId != nil && ticket.IsThread)(ctx.Worker(), ticket)...)

	if len(buttons) > 0 {
		ctx.ReplyWithEmbedAndComponents(e, utils.Slice(component.BuildActionRow(buttons...)))
	} else {
		ctx.ReplyWithEmbed(e)
	}
}

package utils

import (
	"context"

	"github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/channel/embed"
	"github.com/rxdn/gdl/objects/guild/emoji"
)

func BuildEmbed(
	ctx registry.CommandContext,
	colour customisation.Colour, titleId, contentId i18n.MessageId, fields []embed.EmbedField,
	format ...interface{},
) *embed.Embed {
	title := i18n.GetMessageFromGuild(ctx.GuildId(), titleId)
	content := i18n.GetMessageFromGuild(ctx.GuildId(), contentId, format...)

	msgEmbed := embed.NewEmbed().
		SetColor(ctx.GetColour(colour)).
		SetTitle(title).
		SetDescription(content)

	for _, field := range fields {
		msgEmbed.AddField(field.Name, field.Value, field.Inline)
	}

	msgEmbed.SetFooter("Tickets by jaDevelopment", "https://avatars.githubusercontent.com/u/142818403")

	return msgEmbed
}

func BuildEmbedRaw(
	colourHex int, title, content string, fields []embed.EmbedField,
) *embed.Embed {
	msgEmbed := embed.NewEmbed().
		SetColor(colourHex).
		SetTitle(title).
		SetDescription(content)

	for _, field := range fields {
		msgEmbed.AddField(field.Name, field.Value, field.Inline)
	}

	msgEmbed.SetFooter("Tickets by jaDevelopment", "https://avatars.githubusercontent.com/u/142818403")

	return msgEmbed
}

func GetColourForGuild(ctx context.Context, worker *worker.Context, colour customisation.Colour, guildId uint64) (int, error) {
	colourCode, ok, err := dbclient.Client.CustomColours.Get(ctx, guildId, colour.Int16())
	if err != nil {
		return 0, err
	} else if !ok {
		return colour.Default(), nil
	} else {
		return colourCode, nil
	}
}

func EmbedFieldRaw(name, value string, inline bool) embed.EmbedField {
	return embed.EmbedField{
		Name:   name,
		Value:  value,
		Inline: inline,
	}
}

func EmbedField(guildId uint64, name string, value i18n.MessageId, inline bool, format ...interface{}) embed.EmbedField {
	return embed.EmbedField{
		Name:   name,
		Value:  i18n.GetMessageFromGuild(guildId, value, format...),
		Inline: inline,
	}
}

func BuildEmoji(emote string) *emoji.Emoji {
	return &emoji.Emoji{
		Name: emote,
	}
}

func Embeds(embeds ...*embed.Embed) []*embed.Embed {
	return embeds
}

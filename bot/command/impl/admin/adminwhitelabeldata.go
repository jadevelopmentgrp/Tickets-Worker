package admin

import (
	"fmt"
	"strings"
	"time"

	"github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/channel/embed"
	"github.com/rxdn/gdl/objects/interaction"
	"github.com/rxdn/gdl/rest"
)

type AdminWhitelabelDataCommand struct {
}

func (AdminWhitelabelDataCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "whitelabel-data",
		Description:     i18n.HelpAdmin,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.Settings,
		HelperOnly:      true,
		Arguments: command.Arguments(
			command.NewRequiredArgument("user_id", "ID of the user who has the whitelabel subscription", interaction.OptionTypeUser, i18n.MessageInvalidArgument),
		),
		Timeout: time.Second * 10,
	}
}

func (c AdminWhitelabelDataCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AdminWhitelabelDataCommand) Execute(ctx registry.CommandContext, userId uint64) {
	data, err := dbclient.Client.Whitelabel.GetByUserId(ctx, userId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var botIdFormatted = "Bot not found"
	var publicKeyFormatted = "Not set"
	if data.BotId != 0 {
		botIdFormatted = fmt.Sprintf("%d (<@%d>)", data.BotId, data.BotId)

		application, err := rest.GetCurrentApplication(ctx, data.Token, nil)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if application.VerifyKey == data.PublicKey {
			publicKeyFormatted = "Matches"
		} else {
			publicKeyFormatted = "Does not match(!)"
		}
	}

	errors, err := dbclient.Client.WhitelabelErrors.GetRecent(ctx, userId, 3)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var errorsFormatted string
	if len(errors) == 0 {
		errorsFormatted = "No errors found"
	} else {
		strs := make([]string, len(errors))
		for i, botError := range errors {
			strs[i] = fmt.Sprintf("[<t:%d:f>] `%s`", botError.Time.Unix(), botError.Message)
		}

		errorsFormatted = strings.Join(strs, "\n")
	}

	guilds, err := dbclient.Client.WhitelabelGuilds.GetGuilds(ctx, data.BotId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var guildsFormatted string
	if len(guilds) == 0 {
		guildsFormatted = "No Guilds"
	} else {
		for _, guild := range guilds {
			guildsFormatted += fmt.Sprintf("%d\n", guild)
		}

		guildsFormatted = strings.TrimSuffix(guildsFormatted, "\n")
	}

	fields := []embed.EmbedField{
		utils.EmbedFieldRaw("Bot ID", botIdFormatted, true),
		utils.EmbedFieldRaw("Public Key", publicKeyFormatted, true),
		utils.EmbedFieldRaw("Guilds", guildsFormatted, true),
		utils.EmbedFieldRaw("Last 3 Errors", errorsFormatted, true),
	}

	ctx.ReplyWithEmbed(utils.BuildEmbedRaw(ctx.GetColour(customisation.Green), "Whitelabel", "", fields))
}

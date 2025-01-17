package admin

import (
	"fmt"
	"strconv"
	"time"

	"github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/interaction"
)

type AdminCheckBlacklistCommand struct {
}

func (AdminCheckBlacklistCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "check-blacklist",
		Description:     i18n.HelpAdmin,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.Settings,
		HelperOnly:      true,
		Arguments: command.Arguments(
			command.NewRequiredArgument("guild_id", "ID of the guild to unblacklist", interaction.OptionTypeString, i18n.MessageInvalidArgument),
		),
		Timeout: time.Second * 10,
	}
}

func (c AdminCheckBlacklistCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AdminCheckBlacklistCommand) Execute(ctx registry.CommandContext, raw string) {
	guildId, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, ctx.GetMessage(i18n.Error), "Invalid guild ID provided")
		return
	}

	isBlacklisted, reason, err := dbclient.Client.ServerBlacklist.IsBlacklisted(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if isBlacklisted {
		reasonFormatted := utils.ValueOrDefault(reason, "No reason provided")
		ctx.ReplyRaw(customisation.Orange, "Blacklist Check", fmt.Sprintf("This guild is blacklisted.\n```%s```", reasonFormatted))
	} else {
		ctx.ReplyRaw(customisation.Green, "Blacklist Check", "This guild is not blacklisted")
	}
}

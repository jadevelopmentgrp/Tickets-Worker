package admin

import (
	"fmt"
	"strconv"
	"time"

	"github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/interaction"
)

type AdminCheckPremiumCommand struct {
}

func (AdminCheckPremiumCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "checkpremium",
		Description:     i18n.HelpAdminCheckPremium,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.Settings,
		HelperOnly:      true,
		Arguments: command.Arguments(
			command.NewRequiredArgument("guild_id", "ID of the guild to check premium status for", interaction.OptionTypeString, i18n.MessageInvalidArgument),
		),
		Timeout: time.Second * 10,
	}
}

func (c AdminCheckPremiumCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AdminCheckPremiumCommand) Execute(ctx registry.CommandContext, raw string) {
	guildId, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, ctx.GetMessage(i18n.Error), "Invalid guild ID provided")
		return
	}

	guild, err := ctx.Worker().GetGuild(guildId)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, ctx.GetMessage(i18n.Error), err.Error())
		return
	}

	tier, src, err := utils.PremiumClient.GetTierByGuild(ctx, guild)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.ReplyRaw(customisation.Green, ctx.GetMessage(i18n.Admin), fmt.Sprintf("`%s` (owner <@%d> %d) has premium tier %d (src %s)", guild.Name, guild.OwnerId, guild.OwnerId, tier, src))
}

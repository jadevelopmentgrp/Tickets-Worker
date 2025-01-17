package settings

import (
	"time"

	"github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/interaction"
)

type AutoCloseConfigureCommand struct {
}

func (AutoCloseConfigureCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "configure",
		Description:      i18n.HelpAutoCloseConfigure,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permission.Admin,
		Category:         command.Settings,
		DefaultEphemeral: true,
		Timeout:          time.Second * 3,
	}
}

func (c AutoCloseConfigureCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AutoCloseConfigureCommand) Execute(ctx registry.CommandContext) {
	ctx.Reply(customisation.Green, i18n.TitleAutoclose, i18n.MessageAutoCloseConfigure, ctx.GuildId())
}

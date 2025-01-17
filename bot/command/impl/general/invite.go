package general

import (
	"time"

	"github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/interaction"
)

type InviteCommand struct {
}

func (InviteCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "invite",
		Description:      i18n.MessageHelpInvite,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permission.Everyone,
		Category:         command.General,
		MainBotOnly:      true,
		DefaultEphemeral: true,
		Timeout:          time.Second * 3,
	}
}

func (c InviteCommand) GetExecutor() interface{} {
	return c.Execute
}

func (InviteCommand) Execute(ctx registry.CommandContext) {
	ctx.Reply(customisation.Green, i18n.TitleInvite, i18n.MessageInvite)
}

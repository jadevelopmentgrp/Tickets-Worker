package statistics

import (
	"github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/interaction"
)

type StatsCommand struct {
}

func (StatsCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "stats",
		Description:     i18n.HelpStats,
		Type:            interaction.ApplicationCommandTypeChatInput,
		Aliases:         []string{"statistics"},
		PermissionLevel: permission.Support,
		Children: []registry.Command{
			StatsUserCommand{},
			StatsServerCommand{},
		},
		Category: command.Statistics,
	}
}

func (c StatsCommand) GetExecutor() interface{} {
	return c.Execute
}

func (StatsCommand) Execute(ctx registry.CommandContext) {
	// Cannot call parent command
}

package general

import (
	"github.com/TicketsBot/common/permission"
	"github.com/TicketsBot/worker/bot/command"
	"github.com/TicketsBot/worker/bot/command/registry"
	"github.com/TicketsBot/worker/bot/utils"
	"github.com/TicketsBot/worker/i18n"
)

type VoteCommand struct {
}

func (VoteCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "vote",
		Description:      i18n.HelpVote,
		PermissionLevel:  permission.Everyone,
		Category:         command.General,
		DefaultEphemeral: true,
	}
}

func (c VoteCommand) GetExecutor() interface{} {
	return c.Execute
}

func (VoteCommand) Execute(ctx registry.CommandContext) {
	ctx.Reply(utils.Green, "Vote", i18n.MessageVote)
	ctx.Accept()
}

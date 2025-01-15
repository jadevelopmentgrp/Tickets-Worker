package tags

import (
	"fmt"
	"strings"
	"time"

	"github.com/TicketsBot/common/permission"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/interaction"
)

type ManageTagsListCommand struct {
}

func (ManageTagsListCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "list",
		Description:      i18n.HelpTagList,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permission.Support,
		Category:         command.Tags,
		DefaultEphemeral: true,
		Timeout:          time.Second * 3,
	}
}

func (c ManageTagsListCommand) GetExecutor() interface{} {
	return c.Execute
}

func (ManageTagsListCommand) Execute(ctx registry.CommandContext) {
	ids, err := dbclient.Client.Tag.GetTagIds(ctx, ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var joined string
	for _, id := range ids {
		joined += fmt.Sprintf("• `%s`\n", id)
	}
	joined = strings.TrimSuffix(joined, "\n")

	ctx.Reply(customisation.Green, i18n.TitleTags, i18n.MessageTagList, joined, "/")
}

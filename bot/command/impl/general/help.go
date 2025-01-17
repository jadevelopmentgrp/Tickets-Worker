package general

import (
	"sort"
	"strings"
	"time"

	"github.com/elliotchance/orderedmap"
	"github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/channel/embed"
	"github.com/rxdn/gdl/objects/interaction"
)

type HelpCommand struct {
	Registry registry.Registry
}

func (HelpCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "help",
		Description:      i18n.HelpHelp,
		Type:             interaction.ApplicationCommandTypeChatInput,
		Aliases:          []string{"h"},
		PermissionLevel:  permission.Everyone,
		Category:         command.General,
		DefaultEphemeral: true,
		Timeout:          time.Second * 5,
	}
}

func (c HelpCommand) GetExecutor() interface{} {
	return c.Execute
}

func (c HelpCommand) Execute(ctx registry.CommandContext) {
	commandCategories := orderedmap.NewOrderedMap()

	// initialise map with the correct order of categories
	for _, category := range command.Categories {
		commandCategories.Set(category, nil)
	}

	permLevel, err := ctx.UserPermissionLevel(ctx)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	commandIds, err := command.LoadCommandIds(ctx.Worker(), ctx.Worker().BotId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	for _, cmd := range c.Registry {
		properties := cmd.Properties()

		// check bot admin / helper only commands
		if (properties.AdminOnly && !utils.IsBotAdmin(ctx.UserId())) || (properties.HelperOnly && !utils.IsBotHelper(ctx.UserId())) {
			continue
		}

		// Show slash commands only
		if properties.Type != interaction.ApplicationCommandTypeChatInput {
			continue
		}

		if permLevel >= cmd.Properties().PermissionLevel { // only send commands the user has permissions for
			var current []registry.Command
			if commands, ok := commandCategories.Get(properties.Category); ok {
				if commands == nil {
					current = make([]registry.Command, 0)
				} else {
					current = commands.([]registry.Command)
				}
			}
			current = append(current, cmd)

			commandCategories.Set(properties.Category, current)
		}
	}

	embed := embed.NewEmbed().
		SetColor(ctx.GetColour(customisation.Green)).
		SetTitle(ctx.GetMessage(i18n.TitleHelp))

	for _, category := range commandCategories.Keys() {
		var commands []registry.Command
		if retrieved, ok := commandCategories.Get(category.(command.Category)); ok {
			if retrieved == nil {
				commands = make([]registry.Command, 0)
			} else {
				commands = retrieved.([]registry.Command)
			}
		}

		sort.Slice(commands, func(i, j int) bool {
			return commands[i].Properties().Name < commands[j].Properties().Name
		})

		if len(commands) > 0 {
			formatted := make([]string, 0)
			for _, cmd := range commands {
				var commandId *uint64
				if tmp, ok := commandIds[cmd.Properties().Name]; ok {
					commandId = &tmp
				}

				formatted = append(formatted, registry.FormatHelp(cmd, ctx.GuildId(), commandId))
			}

			embed.AddField(string(category.(command.Category)), strings.Join(formatted, "\n"), false)
		}
	}

	embed.SetFooter("Tickets by jaDevelopment", "https://avatars.githubusercontent.com/u/142818403")

	// Explicitly ignore error to fix 403 (Cannot send messages to this user)
	_, _ = ctx.ReplyWith(command.NewEphemeralEmbedMessageResponse(embed))
}

package event

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	worker "github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/blacklist"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	cmdcontext "github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/impl/tags"
	cmdregistry "github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/customisation"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/metrics/prometheus"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/metrics/statsd"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/interaction"
	"golang.org/x/sync/errgroup"
)

// TODO: Command not found messages
// (defaultDefer, error)
func executeCommand(
	ctx context.Context,
	worker *worker.Context,
	registry cmdregistry.Registry,
	data interaction.ApplicationCommandInteraction,
	responseCh chan interaction.ApplicationCommandCallbackData,
) (bool, error) {
	// data.Member is needed for permission level lookup
	if data.GuildId.Value == 0 || data.Member == nil {
		responseCh <- interaction.ApplicationCommandCallbackData{
			Content: "Commands in DMs are not currently supported. Please run this command in a server.",
		}
		return false, nil
	}

	cmd, ok := registry[data.Data.Name]
	if !ok {
		// If a registered command is not found, check for a tag alias
		tag, exists, err := dbclient.Client.Tag.GetByApplicationCommandId(ctx, data.GuildId.Value, data.Data.Id)
		if err != nil {
			return false, err
		}

		if !exists {
			return false, fmt.Errorf("command %s does not exist", data.Data.Name)
		}

		// Execute tag
		cmd = tags.NewTagAliasCommand(tag)
		ok = true
	}

	options := data.Data.Options
	for len(options) > 0 && options[0].Value == nil { // Value and Options are mutually exclusive, value is never present on subcommands
		subCommand := options[0]

		var found bool
		for _, child := range cmd.Properties().Children {
			if child.Properties().Name == subCommand.Name {
				cmd = child
				found = true
				break
			}
		}

		if !found {
			return false, fmt.Errorf("subcommand %s does not exist for command %s", subCommand.Name, cmd.Properties().Name)
		}

		options = subCommand.Options
	}

	properties := cmd.Properties()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovering panicking goroutine while executing command %s: %v\n", properties.Name, r)
				debug.PrintStack()
			}
		}()

		lookupCtx, cancelLookupCtx := context.WithTimeout(ctx, time.Second*2)
		defer cancelLookupCtx()

		// Parallelise queries
		group, _ := errgroup.WithContext(lookupCtx)

		// Get permission level
		var permLevel = permission.Everyone
		group.Go(func() error {
			res, err := permission.GetPermissionLevel(lookupCtx, utils.ToRetriever(worker), *data.Member, data.GuildId.Value)
			if err != nil {
				return err
			}

			permLevel = res
			return nil
		})

		if err := group.Wait(); err != nil {
			fmt.Print(err)
			responseCh <- interaction.ApplicationCommandCallbackData{
				Content: fmt.Sprintf("An error occurred while processing this request."),
			}
			return
		}

		ctx, cancel := context.WithTimeout(ctx, properties.Timeout)
		defer cancel()

		interactionContext := cmdcontext.NewSlashCommandContext(ctx, worker, data, 0, responseCh)

		// Check if the guild is globally blacklisted
		if blacklist.IsGuildBlacklisted(data.GuildId.Value) {
			interactionContext.Reply(customisation.Red, i18n.TitleBlacklisted, i18n.MessageBlacklisted)
			return
		}

		if properties.PermissionLevel > permLevel {
			interactionContext.Reply(customisation.Red, i18n.Error, i18n.MessageNoPermission)
			return
		}

		if properties.AdminOnly && !utils.IsBotAdmin(interactionContext.UserId()) {
			interactionContext.Reply(customisation.Red, i18n.Error, i18n.MessageOwnerOnly)
			return
		}

		if properties.HelperOnly && !utils.IsBotHelper(interactionContext.UserId()) {
			interactionContext.Reply(customisation.Red, i18n.Error, i18n.MessageNoPermission)
			return
		}

		// Check for user blacklist - cannot parallelise as relies on permission level
		// If data.Member is nil, it does not matter, as it is not checked if the command is not executed in a guild
		blacklisted, err := interactionContext.IsBlacklisted(lookupCtx)
		cancelLookupCtx()
		if err != nil {
			interactionContext.HandleError(err)
			return
		}

		if blacklisted {
			interactionContext.Reply(customisation.Red, i18n.TitleBlacklisted, i18n.MessageBlacklisted)
			return
		}

		statsd.Client.IncrementKey(statsd.KeySlashCommands)
		statsd.Client.IncrementKey(statsd.KeyCommands)
		prometheus.LogCommand(data.Data.Name)

		defer close(responseCh)

		if err := callCommand(cmd, &interactionContext, options); err != nil {
			if errors.Is(err, ErrArgumentNotFound) {
				if worker.IsWhitelabel {
					content := `This command registration is outdated. Please ask the server administrators to visit the whitelabel dashboard and press "Create Slash Commands" again.`
					embed := utils.BuildEmbedRaw(customisation.GetDefaultColour(customisation.Red), "Outdated Command", content, nil)
					res := command.NewEphemeralEmbedMessageResponse(embed)
					responseCh <- res.IntoApplicationCommandData()

					return
				} else {
					res := command.NewEphemeralTextMessageResponse("argument is missing")
					responseCh <- res.IntoApplicationCommandData()
				}
			} else {
				interactionContext.HandleError(err)
				return
			}
		}
	}()

	return properties.DefaultEphemeral, nil
}

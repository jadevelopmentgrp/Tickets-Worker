package manager

import (
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/impl/general"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/impl/settings"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/impl/settings/setup"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/impl/statistics"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/impl/tags"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/impl/tickets"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/i18n"
	"github.com/rxdn/gdl/objects/interaction"
	"github.com/rxdn/gdl/rest"
)

type CommandManager struct {
	registry registry.Registry
}

func (cm *CommandManager) GetCommands() map[string]registry.Command {
	return cm.registry
}

func (cm *CommandManager) RegisterCommands() {
	cm.registry = make(map[string]registry.Command)

	cm.registry["help"] = general.HelpCommand{Registry: cm.registry}

	cm.registry["about"] = general.AboutCommand{}
	cm.registry["jumptotop"] = general.JumpToTopCommand{}

	cm.registry["addadmin"] = settings.AddAdminCommand{}
	cm.registry["addsupport"] = settings.AddSupportCommand{}
	cm.registry["autoclose"] = settings.AutoCloseCommand{}
	cm.registry["blacklist"] = settings.BlacklistCommand{}
	cm.registry["language"] = settings.LanguageCommand{}
	cm.registry["panel"] = settings.PanelCommand{}
	cm.registry["removeadmin"] = settings.RemoveAdminCommand{}
	cm.registry["removesupport"] = settings.RemoveSupportCommand{}
	cm.registry["setup"] = setup.SetupCommand{}
	cm.registry["viewstaff"] = settings.ViewStaffCommand{}

	cm.registry["stats"] = statistics.StatsCommand{}

	cm.registry["managetags"] = tags.ManageTagsCommand{}
	cm.registry["tag"] = tags.TagCommand{}

	cm.registry["add"] = tickets.AddCommand{}
	cm.registry["claim"] = tickets.ClaimCommand{}
	cm.registry["close"] = tickets.CloseCommand{}
	cm.registry["closerequest"] = tickets.CloseRequestCommand{}
	cm.registry["notes"] = tickets.NotesCommand{}
	cm.registry["on-call"] = tickets.OnCallCommand{}
	cm.registry["open"] = tickets.OpenCommand{}
	cm.registry["Start Ticket"] = tickets.StartTicketCommand{}
	cm.registry["remove"] = tickets.RemoveCommand{}
	cm.registry["rename"] = tickets.RenameCommand{}
	cm.registry["reopen"] = tickets.ReopenCommand{}
	cm.registry["switchpanel"] = tickets.SwitchPanelCommand{}
	cm.registry["transfer"] = tickets.TransferCommand{}
	cm.registry["unclaim"] = tickets.UnclaimCommand{}
}

func (cm *CommandManager) RunSetupFuncs() {
	for _, command := range cm.registry {
		if command.Properties().SetupFunc != nil {
			command.Properties().SetupFunc()
		}
	}
}

func (cm *CommandManager) BuildCreatePayload() (data []rest.CreateCommandData) {
	for _, cmd := range cm.GetCommands() {
		properties := cmd.Properties()

		if properties.MessageOnly {
			continue
		}

		option := buildOption(cmd)

		var description string
		if properties.Type == interaction.ApplicationCommandTypeChatInput {
			description = option.Description
		}

		cmdData := rest.CreateCommandData{
			Name:        option.Name,
			Description: description,
			Options:     option.Options,
			Type:        properties.Type,
		}

		data = append(data, cmdData)
	}

	return data
}

func buildOption(cmd registry.Command) interaction.ApplicationCommandOption {
	properties := cmd.Properties()

	// Required args must come before optional args
	var required []interaction.ApplicationCommandOption
	var optional []interaction.ApplicationCommandOption

	for _, child := range properties.Children {
		if child.Properties().MessageOnly {
			continue
		}

		option := buildOption(child)

		if option.Required {
			required = append(required, option)
		} else {
			optional = append(optional, option)
		}
	}

	for _, argument := range properties.Arguments {
		option := interaction.ApplicationCommandOption{
			Type:         argument.Type,
			Name:         argument.Name,
			Description:  argument.Description,
			Default:      false,
			Required:     argument.Required,
			Choices:      nil,
			Autocomplete: argument.AutoCompleteHandler != nil,
			Options:      nil,
		}

		if option.Required {
			required = append(required, option)
		} else {
			optional = append(optional, option)
		}
	}

	options := append(required, optional...)

	return interaction.ApplicationCommandOption{
		Type:        interaction.OptionTypeSubCommand,
		Name:        properties.Name,
		Description: i18n.GetMessage(i18n.LocaleEnglish, properties.Description),
		Default:     false,
		Required:    false,
		Choices:     nil,
		Options:     options,
	}
}

package context

import (
	"context"
	"errors"
	"fmt"

	permcache "github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	worker "github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/command/registry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/errorcontext"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/rxdn/gdl/objects/channel"
	"github.com/rxdn/gdl/objects/channel/message"
	"github.com/rxdn/gdl/objects/guild"
	"github.com/rxdn/gdl/objects/interaction"
	"github.com/rxdn/gdl/objects/member"
	"github.com/rxdn/gdl/objects/user"
	"go.uber.org/atomic"
)

type SlashCommandContext struct {
	context.Context
	*Replyable
	*ReplyCounter
	*StateCache
	InteractionExtension
	worker      *worker.Context
	Interaction interaction.ApplicationCommandInteraction

	hasReplied *atomic.Bool
	responseCh chan interaction.ApplicationCommandCallbackData
}

var _ registry.CommandContext = (*SlashCommandContext)(nil)

func NewSlashCommandContext(
	ctx context.Context,
	worker *worker.Context,
	interaction interaction.ApplicationCommandInteraction,
	responseCh chan interaction.ApplicationCommandCallbackData,
) SlashCommandContext {
	c := SlashCommandContext{
		Context: ctx,

		ReplyCounter: NewReplyCounter(),

		InteractionExtension: NewInteractionExtension(interaction),

		worker:      worker,
		Interaction: interaction,

		hasReplied: atomic.NewBool(false),
		responseCh: responseCh,
	}

	c.Replyable = NewReplyable(&c)
	c.StateCache = NewStateCache(&c)
	return c
}

func (c *SlashCommandContext) Worker() *worker.Context {
	return c.worker
}

func (c *SlashCommandContext) GuildId() uint64 {
	return c.Interaction.GuildId.Value // TODO: Null check
}

func (c *SlashCommandContext) ChannelId() uint64 {
	return c.Interaction.ChannelId
}

func (c *SlashCommandContext) UserId() uint64 {
	if c.Interaction.Member != nil {
		return c.Interaction.Member.User.Id
	} else if c.Interaction.User != nil {
		return c.Interaction.User.Id
	} else {
		fmt.Errorf("infallible: interaction.member and interaction.user are both null")
		return 0
	}
}

func (c *SlashCommandContext) UserPermissionLevel(ctx context.Context) (permcache.PermissionLevel, error) {
	if c.Interaction.Member == nil {
		return permcache.Everyone, errors.New("member was nil")
	}

	return permcache.GetPermissionLevel(ctx, utils.ToRetriever(c.worker), *c.Interaction.Member, c.GuildId())
}

func (c *SlashCommandContext) IsInteraction() bool {
	return true
}

func (c *SlashCommandContext) Source() registry.Source {
	return registry.SourceDiscord
}

func (c *SlashCommandContext) ToErrorContext() errorcontext.WorkerErrorContext {
	return errorcontext.WorkerErrorContext{
		Guild:   c.GuildId(),
		User:    c.Interaction.Member.User.Id,
		Channel: c.ChannelId(),
	}
}

func (c *SlashCommandContext) ReplyWith(response command.MessageResponse) (message.Message, error) {
	//hasReplied := c.hasReplied.Swap(true)

	if err := c.ReplyCounter.Try(); err != nil {
		return message.Message{}, err
	}

	c.responseCh <- response.IntoApplicationCommandData()

	return message.Message{}, nil
}

func (c *SlashCommandContext) Channel() (channel.PartialChannel, error) {
	return c.Interaction.Channel, nil
}

func (c *SlashCommandContext) Guild() (guild.Guild, error) {
	return c.Worker().GetGuild(c.GuildId())
}

func (c *SlashCommandContext) Member() (member.Member, error) {
	if c.Interaction.Member != nil {
		return *c.Interaction.Member, nil
	} else {
		return c.Worker().GetGuildMember(c.GuildId(), c.UserId())
	}
}

func (c *SlashCommandContext) User() (user.User, error) {
	return c.Worker().GetUser(c.UserId())
}

func (c *SlashCommandContext) IsBlacklisted(ctx context.Context) (bool, error) {
	permLevel, err := c.UserPermissionLevel(ctx)
	if err != nil {
		return false, err
	}

	// if interaction.Member is nil, it does not matter, as the member's roles are not checked
	// if the command is not executed in a guild
	return utils.IsBlacklisted(ctx, c.GuildId(), c.UserId(), utils.ValueOrZero(c.Interaction.Member), permLevel)
}

/// InteractionContext functions

func (c *SlashCommandContext) InteractionMetadata() interaction.InteractionMetadata {
	return c.Interaction.InteractionMetadata
}

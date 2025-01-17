package context

import (
	"context"
	"errors"
	"fmt"

	permcache "github.com/jadevelopmentgrp/Tickets-Utilities/permission"
	worker "github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button"
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

type SelectMenuContext struct {
	context.Context
	*Replyable
	*ReplyCounter
	*MessageComponentExtensions
	*StateCache
	worker          *worker.Context
	Interaction     interaction.MessageComponentInteraction
	InteractionData interaction.SelectMenuInteractionData
	hasReplied      *atomic.Bool
	responseChannel chan button.Response
}

var _ registry.CommandContext = (*SelectMenuContext)(nil)

func NewSelectMenuContext(
	ctx context.Context,
	worker *worker.Context,
	interaction interaction.MessageComponentInteraction,
	responseChannel chan button.Response,
) *SelectMenuContext {
	c := SelectMenuContext{
		Context:         ctx,
		ReplyCounter:    NewReplyCounter(),
		worker:          worker,
		Interaction:     interaction,
		InteractionData: interaction.Data.AsSelectMenu(),
		hasReplied:      atomic.NewBool(false),
		responseChannel: responseChannel,
	}

	c.Replyable = NewReplyable(&c)
	c.MessageComponentExtensions = NewMessageComponentExtensions(&c, interaction.InteractionMetadata, responseChannel, c.hasReplied)
	c.StateCache = NewStateCache(&c)
	return &c
}

func (c *SelectMenuContext) Worker() *worker.Context {
	return c.worker
}

func (c *SelectMenuContext) GuildId() uint64 {
	return c.Interaction.GuildId.Value // TODO: Null check
}

func (c *SelectMenuContext) ChannelId() uint64 {
	return c.Interaction.ChannelId
}

func (c *SelectMenuContext) UserId() uint64 {
	return c.InteractionUser().Id
}

func (c *SelectMenuContext) UserPermissionLevel(ctx context.Context) (permcache.PermissionLevel, error) {
	if c.Interaction.Member == nil {
		return permcache.Everyone, errors.New("member was nil")
	}

	return permcache.GetPermissionLevel(ctx, utils.ToRetriever(c.worker), *c.Interaction.Member, c.GuildId())
}

func (c *SelectMenuContext) IsInteraction() bool {
	return true
}

func (c *SelectMenuContext) Source() registry.Source {
	return registry.SourceDiscord
}

func (c *SelectMenuContext) ToErrorContext() errorcontext.WorkerErrorContext {
	return errorcontext.WorkerErrorContext{
		Guild:   c.GuildId(),
		User:    c.UserId(),
		Channel: c.ChannelId(),
	}
}

func (c *SelectMenuContext) ReplyWith(response command.MessageResponse) (msg message.Message, err error) {
	//hasReplied := c.hasReplied.Swap(true)

	if err := c.ReplyCounter.Try(); err != nil {
		return message.Message{}, err
	}

	c.responseChannel <- button.ResponseMessage{
		Data: response,
	}

	return
}

func (c *SelectMenuContext) Channel() (channel.PartialChannel, error) {
	return c.Interaction.Channel, nil
}

func (c *SelectMenuContext) Guild() (guild.Guild, error) {
	return c.Worker().GetGuild(c.GuildId())
}

func (c *SelectMenuContext) Member() (member.Member, error) {
	if c.GuildId() == 0 {
		return member.Member{}, fmt.Errorf("button was not clicked in a guild")
	}

	if c.Interaction.Member != nil {
		return *c.Interaction.Member, nil
	} else {
		return c.Worker().GetGuildMember(c.GuildId(), c.UserId())
	}
}

func (c *SelectMenuContext) InteractionMember() member.Member {
	if c.Interaction.Member != nil {
		return *c.Interaction.Member
	} else {
		fmt.Print(fmt.Errorf("SelectMenuContext.InteractionMember was called when Member is nil"), c.ToErrorContext())
		return member.Member{}
	}
}

func (c *SelectMenuContext) User() (user.User, error) {
	return c.InteractionUser(), nil
}

func (c *SelectMenuContext) InteractionUser() user.User {
	if c.Interaction.Member != nil {
		return c.Interaction.Member.User
	} else if c.Interaction.User != nil {
		return *c.Interaction.User
	} else { // Infallible
		fmt.Print(fmt.Errorf("infallible: SelectMenuContext.InteractionUser was called when User is nil"), c.ToErrorContext())
		return user.User{}
	}
}

func (c *SelectMenuContext) IntoPanelContext() PanelContext {
	return NewPanelContext(c.Context, c.worker, c.GuildId(), c.ChannelId(), c.InteractionUser().Id)
}

func (c *SelectMenuContext) IsBlacklisted(ctx context.Context) (bool, error) {
	// TODO: Check user blacklist
	if c.GuildId() == 0 {
		return false, nil
	}

	permLevel, err := c.UserPermissionLevel(ctx)
	if err != nil {
		return false, err
	}

	// if interaction.Member is nil, it does not matter, as the member's roles are not checked
	// if the command is not executed in a guild
	return utils.IsBlacklisted(ctx, c.GuildId(), c.UserId(), utils.ValueOrZero(c.Interaction.Member), permLevel)
}

/// InteractionContext functions

func (c *SelectMenuContext) InteractionMetadata() interaction.InteractionMetadata {
	return c.Interaction.InteractionMetadata
}

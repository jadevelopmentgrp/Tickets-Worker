package listeners

import (
	"context"
	"errors"
	"fmt"
	"time"

	database "github.com/jadevelopmentgrp/Tickets-Database"
	"github.com/jadevelopmentgrp/Tickets-Utilities/chatrelay"
	"github.com/jadevelopmentgrp/Tickets-Utilities/model"
	worker "github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/metrics/prometheus"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/metrics/statsd"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/redis"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/rxdn/gdl/gateway/payloads/events"
)

// proxy messages to web UI + set last message id
func OnMessage(worker *worker.Context, e events.MessageCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*7) // TODO: Propagate context
	defer cancel()

	statsd.Client.IncrementKey(statsd.KeyMessages)

	// ignore DMs
	if e.GuildId == 0 {
		return
	}

	ticket, isTicket, err := getTicket(ctx, e.ChannelId)
	if err != nil {
		fmt.Print(err, utils.MessageCreateErrorContext(e))
		return
	}

	// ensure valid ticket channel
	if !isTicket || ticket.Id == 0 {
		return
	}

	var isStaffCached *bool

	// ignore our own messages
	if e.Author.Id != worker.BotId && !e.Author.Bot {
		// set participants, for logging
		if err := dbclient.Client.Participants.Set(ctx, e.GuildId, ticket.Id, e.Author.Id); err != nil {
			fmt.Print(err, utils.MessageCreateErrorContext(e))
		}

		isStaffCached, err := isStaff(ctx, e, ticket)

		if err != nil {
			fmt.Print(err, utils.MessageCreateErrorContext(e))
		} else {
			// set ticket last message, for autoclose
			// isStaffCached cannot be nil at this point
			if err := updateLastMessage(ctx, e, ticket, isStaffCached); err != nil {
				fmt.Print(err, utils.MessageCreateErrorContext(e))
			}

			if isStaffCached { // check the user is staff
				// We don't have to check for previous responses due to ON CONFLICT DO NOTHING
				if err := dbclient.Client.FirstResponseTime.Set(ctx, e.GuildId, e.Author.Id, ticket.Id, time.Now().Sub(ticket.OpenTime)); err != nil {
					fmt.Print(err, utils.MessageCreateErrorContext(e))
				}
			}
		}
	}
	// proxy msg to web UI
	if err := chatrelay.PublishMessage(redis.Client, chatrelay.MessageData{
		Ticket:  ticket,
		Message: e.Message,
	}); err != nil {
		fmt.Print(err, utils.MessageCreateErrorContext(e))
	}

	// Ignore the welcome message and ping message
	if e.Author.Id != worker.BotId {
		var userIsStaff bool
		if isStaffCached != nil {
			userIsStaff = *isStaffCached
		} else {
			tmp, err := isStaff(ctx, e, ticket)
			if err != nil {
				fmt.Print(err, utils.MessageCreateErrorContext(e))
				return
			}

			userIsStaff = tmp
		}

		var newStatus model.TicketStatus
		if userIsStaff {
			newStatus = model.TicketStatusPending
		} else {
			newStatus = model.TicketStatusOpen
		}

		if ticket.Status != newStatus {
			if err := dbclient.Client.Tickets.SetStatus(ctx, e.GuildId, ticket.Id, newStatus); err != nil {
				fmt.Print(err, utils.MessageCreateErrorContext(e))
			}

			if !ticket.IsThread {
				if err := dbclient.Client.CategoryUpdateQueue.Add(ctx, e.GuildId, ticket.Id, newStatus); err != nil {
					fmt.Print(err, utils.MessageCreateErrorContext(e))
				}
			}
		}
	}
}

func updateLastMessage(ctx context.Context, msg events.MessageCreate, ticket database.Ticket, isStaff bool) error {
	// If last message was sent by staff, don't reset the timer
	lastMessage, err := dbclient.Client.TicketLastMessage.Get(ctx, ticket.GuildId, ticket.Id)
	if err != nil {
		return err
	}

	// No last message, or last message was before we started storing user IDs
	if lastMessage.UserId == nil {
		return dbclient.Client.TicketLastMessage.Set(ctx, ticket.GuildId, ticket.Id, msg.Id, msg.Author.Id, false)
	}

	// If the last message was sent by the ticket opener, we can skip the rest of the logic, and update straight away
	if ticket.UserId == msg.Author.Id {
		return dbclient.Client.TicketLastMessage.Set(ctx, ticket.GuildId, ticket.Id, msg.Id, msg.Author.Id, false)
	}

	// If the last message *and* this message were sent by staff members, then do not reset the timer
	if lastMessage.UserId != nil && *lastMessage.UserIsStaff && isStaff {
		return nil
	}

	return dbclient.Client.TicketLastMessage.Set(ctx, ticket.GuildId, ticket.Id, msg.Id, msg.Author.Id, isStaff)
}

// This method should not be used for anything requiring elevated privileges
func isStaff(ctx context.Context, msg events.MessageCreate, ticket database.Ticket) (bool, error) {
	// If the user is the ticket opener, they are not staff
	if msg.Author.Id == ticket.UserId {
		return false, nil
	}

	members, err := dbclient.Client.TicketMembers.Get(ctx, ticket.GuildId, ticket.Id)
	if err != nil {
		return false, err
	}

	if utils.Contains(members, msg.Author.Id) {
		return false, nil
	}

	return true, nil
}

func getTicket(ctx context.Context, channelId uint64) (database.Ticket, bool, error) {
	isTicket, err := redis.IsTicketChannel(ctx, channelId)

	cacheHit := err == nil

	if err == nil && !isTicket {
		prometheus.LogOnMessageTicketLookup(false, cacheHit)
		return database.Ticket{}, false, nil
	} else if err != nil && !errors.Is(err, redis.ErrTicketStatusNotCached) {
		return database.Ticket{}, false, err
	}

	// Either cache miss or the ticket *does* exist, so we need to fetch the object from the database
	ticket, ok, err := dbclient.Client.Tickets.GetByChannel(ctx, channelId)
	if err != nil {
		return database.Ticket{}, false, err
	}

	if !ok {
		return database.Ticket{}, false, nil
	}

	if err := redis.SetTicketChannelStatus(ctx, channelId, ticket.Id != 0); err != nil {
		return database.Ticket{}, false, err
	}

	if ticket.Id == 0 {
		prometheus.LogOnMessageTicketLookup(false, cacheHit)
		return database.Ticket{}, false, nil
	}

	prometheus.LogOnMessageTicketLookup(true, cacheHit)

	return ticket, true, nil
}

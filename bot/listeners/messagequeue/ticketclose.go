package messagequeue

import (
	"context"
	"fmt"

	"github.com/jadevelopmentgrp/Tickets-Utilities/closerelay"
	worker "github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/cache"
	cmdcontext "github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/constants"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/errorcontext"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/logic"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/redis"
	"github.com/jadevelopmentgrp/Tickets-Worker/config"
)

// TODO: Make this good
func ListenTicketClose() {
	ch := make(chan closerelay.TicketClose)
	go closerelay.Listen(redis.Client, ch)

	for payload := range ch {
		payload := payload

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutCloseTicket)
			defer cancel()

			if payload.Reason == "" {
				payload.Reason = "No reason specified"
			}

			// Get the ticket struct
			ticket, err := dbclient.Client.Tickets.Get(ctx, payload.TicketId, payload.GuildId)
			if err != nil {
				fmt.Print(err)
				return
			}

			// Check that this is a valid ticket
			if ticket.GuildId == 0 {
				return
			}

			// Create error context for later
			errorContext := errorcontext.WorkerErrorContext{
				Guild: ticket.GuildId,
				User:  payload.UserId,
			}

			// Get bot token for guild
			var token string
			var botId uint64
			{
				whiteLabelBotId, isWhitelabel, err := dbclient.Client.WhitelabelGuilds.GetBotByGuild(ctx, payload.GuildId)
				if err != nil {
					fmt.Print(err, errorContext)
				}

				if isWhitelabel {
					bot, err := dbclient.Client.Whitelabel.GetByBotId(ctx, whiteLabelBotId)
					if err != nil {
						fmt.Print(err, errorContext)
						return
					}

					if bot.Token == "" {
						token = config.Conf.Discord.Token
					} else {
						token = bot.Token
						botId = whiteLabelBotId
					}
				} else {
					token = config.Conf.Discord.Token
				}
			}

			// Create worker context
			workerCtx := &worker.Context{
				Token:        token,
				IsWhitelabel: botId != 0,
				Cache:        cache.Client, // TODO: Less hacky
				RateLimiter:  nil,          // Use http-proxy ratelimit functionality
			}

			// if ticket didnt open in the first place, no channel ID is assigned.
			// we should close it here, or it wont get closed at all
			if ticket.ChannelId == nil {
				if err := dbclient.Client.Tickets.Close(ctx, payload.TicketId, payload.GuildId); err != nil {
					fmt.Print(err, errorContext)
				}
				return
			}

			// ticket.ChannelId cannot be nil
			cc := cmdcontext.NewDashboardContext(ctx, workerCtx, ticket.GuildId, *ticket.ChannelId, payload.UserId)
			logic.CloseTicket(ctx, &cc, &payload.Reason, false)
		}()
	}
}

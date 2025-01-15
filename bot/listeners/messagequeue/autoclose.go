package messagequeue

import (
	"context"

	"github.com/TicketsBot/common/autoclose"
	"github.com/TicketsBot/common/sentry"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/cache"
	cmdcontext "github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/constants"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/logic"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/metrics/statsd"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/redis"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	gdlUtils "github.com/rxdn/gdl/utils"
)

const AutoCloseReason = "Automatically closed due to inactivity"

func ListenAutoClose() {
	ch := make(chan autoclose.Ticket)
	go autoclose.Listen(redis.Client, ch)

	for ticket := range ch {
		statsd.Client.IncrementKey(statsd.AutoClose)

		ticket := ticket
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutCloseTicket)
			defer cancel()

			// get ticket
			ticket, err := dbclient.Client.Tickets.Get(ctx, ticket.TicketId, ticket.GuildId)
			if err != nil {
				sentry.Error(err)
				return
			}

			// get worker
			worker, err := buildContext(ctx, ticket, cache.Client)
			if err != nil {
				sentry.Error(err)
				return
			}

			// query already checks, but just to be sure
			if ticket.ChannelId == nil {
				return
			}

			// get premium status
			premiumTier, err := utils.PremiumClient.GetTierByGuildId(ctx, ticket.GuildId, true, worker.Token, worker.RateLimiter)
			if err != nil {
				sentry.Error(err)
				return
			}

			cc := cmdcontext.NewAutoCloseContext(ctx, worker, ticket.GuildId, *ticket.ChannelId, worker.BotId, premiumTier)
			logic.CloseTicket(ctx, cc, gdlUtils.StrPtr(AutoCloseReason), true)
		}()
	}
}

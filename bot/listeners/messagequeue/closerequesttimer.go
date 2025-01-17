package messagequeue

import (
	"context"
	"fmt"

	database "github.com/jadevelopmentgrp/Tickets-Database"
	"github.com/jadevelopmentgrp/Tickets-Utilities/closerequest"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/cache"
	cmdcontext "github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/constants"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/logic"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/metrics/statsd"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/redis"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
)

func ListenCloseRequestTimer() {
	ch := make(chan database.CloseRequest)
	go closerequest.Listen(redis.Client, ch)

	for request := range ch {
		statsd.Client.IncrementKey(statsd.AutoClose)

		request := request
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutCloseTicket)
			defer cancel()

			// get ticket
			ticket, err := dbclient.Client.Tickets.Get(ctx, request.TicketId, request.GuildId)
			if err != nil {
				fmt.Print(err)
				return
			}

			// get worker
			worker, err := buildContext(ctx, ticket, cache.Client)
			if err != nil {
				fmt.Print(err)
				return
			}

			// query already checks, but just to be sure
			if ticket.ChannelId == nil {
				return
			}

			cc := cmdcontext.NewAutoCloseContext(ctx, worker, ticket.GuildId, *ticket.ChannelId, request.UserId)
			logic.CloseTicket(ctx, cc, request.Reason, true)
		}()
	}
}

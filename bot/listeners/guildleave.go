package listeners

import (
	"context"
	"fmt"
	"time"

	worker "github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/metrics/statsd"
	"github.com/rxdn/gdl/gateway/payloads/events"
)

/*
 * Sent when a guild becomes unavailable during a guild outage, or when the user leaves or is removed from a guild.
 * The inner payload is an unavailable guild object.
 * If the unavailable field is not set, the user was removed from the guild.
 */
func OnGuildLeave(worker *worker.Context, e events.GuildDelete) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3) // TODO: Propagate context
	defer cancel()

	if e.Unavailable == nil {
		statsd.Client.IncrementKey(statsd.KeyLeaves)

		if worker.IsWhitelabel {
			if err := dbclient.Client.WhitelabelGuilds.Delete(ctx, worker.BotId, e.Guild.Id); err != nil {
				fmt.Print(err)
			}
		}

		// Exclude from autoclose
		if err := dbclient.Client.AutoCloseExclude.ExcludeAll(ctx, e.Guild.Id); err != nil {
			fmt.Print(err)
		}

		if err := dbclient.Client.GuildLeaveTime.Set(ctx, e.Guild.Id); err != nil {
			fmt.Print(err)
		}
	}
}

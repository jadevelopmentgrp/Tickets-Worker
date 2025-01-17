package listeners

import (
	"context"
	"fmt"
	"time"

	worker "github.com/jadevelopmentgrp/Tickets-Worker"
	cmdcontext "github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/constants"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/listeners/messagequeue"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/logic"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/rxdn/gdl/gateway/payloads/events"
	gdlUtils "github.com/rxdn/gdl/utils"
)

// Remove user permissions when they leave
func OnMemberLeave(worker *worker.Context, e events.GuildMemberRemove) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3) // TODO: Propagate context
	defer cancel()

	if err := dbclient.Client.Permissions.RemoveSupport(ctx, e.GuildId, e.User.Id); err != nil {
		fmt.Print(err)
	}

	if err := utils.ToRetriever(worker).Cache().DeleteCachedPermissionLevel(ctx, e.GuildId, e.User.Id); err != nil {
		fmt.Print(err)
	}

	// auto close
	settings, err := dbclient.Client.AutoClose.Get(ctx, e.GuildId)
	if err != nil {
		fmt.Print(err)
	} else {
		// check setting is enabled
		if settings.Enabled && settings.OnUserLeave != nil && *settings.OnUserLeave {
			// get open tickets by user
			tickets, err := dbclient.Client.Tickets.GetOpenByUser(ctx, e.GuildId, e.User.Id)
			if err != nil {
				fmt.Print(err)
			} else {
				for _, ticket := range tickets {
					isExcluded, err := dbclient.Client.AutoCloseExclude.IsExcluded(ctx, e.GuildId, ticket.Id)
					if err != nil {
						fmt.Print(err)
						continue
					}

					if isExcluded {
						continue
					}

					// verify ticket exists + prevent potential panic
					if ticket.ChannelId == nil {
						return
					}

					ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutCloseTicket)

					cc := cmdcontext.NewAutoCloseContext(ctx, worker, e.GuildId, *ticket.ChannelId, worker.BotId)
					logic.CloseTicket(ctx, cc, gdlUtils.StrPtr(messagequeue.AutoCloseReason), true)

					cancel()
				}
			}
		}
	}
}

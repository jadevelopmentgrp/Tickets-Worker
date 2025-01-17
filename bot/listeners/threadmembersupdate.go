package listeners

import (
	"context"
	"fmt"
	"time"

	database "github.com/jadevelopmentgrp/Tickets-Database"
	worker "github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/errorcontext"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/logic"
	"github.com/rxdn/gdl/gateway/payloads/events"
)

func OnThreadMembersUpdate(worker *worker.Context, e events.ThreadMembersUpdate) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*6) // TODO: Propagate context
	defer cancel()

	settings, err := dbclient.Client.Settings.Get(ctx, e.GuildId)
	if err != nil {
		fmt.Print(err, errorcontext.WorkerErrorContext{Guild: e.GuildId})
		return
	}

	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, e.ThreadId, e.GuildId)
	if err != nil {
		fmt.Print(err, errorcontext.WorkerErrorContext{Guild: e.GuildId})
		return
	}

	if ticket.Id == 0 || ticket.GuildId != e.GuildId {
		return
	}

	if ticket.JoinMessageId != nil {
		var panel *database.Panel
		if ticket.PanelId != nil {
			tmp, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
			if err != nil {
				fmt.Print(err, errorcontext.WorkerErrorContext{Guild: e.GuildId})
				return
			}

			if tmp.PanelId != 0 && e.GuildId == tmp.GuildId {
				panel = &tmp
			}
		}

		threadStaff, err := logic.GetStaffInThread(ctx, worker, ticket, e.ThreadId)
		if err != nil {
			fmt.Print(err, errorcontext.WorkerErrorContext{Guild: e.GuildId})
			return
		}

		if settings.TicketNotificationChannel != nil {
			data := logic.BuildJoinThreadMessage(ctx, worker, ticket.GuildId, ticket.UserId, ticket.Id, panel, threadStaff)
			if _, err := worker.EditMessage(*settings.TicketNotificationChannel, *ticket.JoinMessageId, data.IntoEditMessageData()); err != nil {
				fmt.Print(err, errorcontext.WorkerErrorContext{Guild: e.GuildId})
			}
		}
	}
}

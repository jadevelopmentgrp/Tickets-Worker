package listeners

import (
	"context"
	"fmt"
	"time"

	worker "github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/dbclient"
	"github.com/rxdn/gdl/gateway/payloads/events"
)

func OnChannelDelete(worker *worker.Context, e events.ChannelDelete) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3) // TODO: Propagate context
	defer cancel()

	// If this is a ticket channel, close it

	if err := dbclient.Client.Tickets.CloseByChannel(ctx, e.Id); err != nil {
		fmt.Print(err)
	}

	// if this is a channel category, delete it
	if err := dbclient.Client.ChannelCategory.DeleteByChannel(ctx, e.Id); err != nil {
		fmt.Print(err)
	}

	// if this is an archive channel, delete it
	if err := dbclient.Client.ArchiveChannel.DeleteByChannel(ctx, e.Id); err != nil {
		fmt.Print(err)
	}
}

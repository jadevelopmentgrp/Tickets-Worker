package listeners

import (
	"context"
	"fmt"
	"time"

	worker "github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/utils"
	"github.com/rxdn/gdl/gateway/payloads/events"
)

// Remove user permissions when they leave
func OnMemberUpdate(worker *worker.Context, e events.GuildMemberUpdate) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3) // TODO: Propagate context
	defer cancel()

	if err := utils.ToRetriever(worker).Cache().DeleteCachedPermissionLevel(ctx, e.GuildId, e.User.Id); err != nil {
		fmt.Print(err)
	}
}

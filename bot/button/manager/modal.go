package manager

import (
	"context"
	"time"

	worker "github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/button"
	cmdcontext "github.com/jadevelopmentgrp/Tickets-Worker/bot/command/context"
	"github.com/rxdn/gdl/objects/interaction"
)

func HandleModalInteraction(ctx context.Context, manager *ComponentInteractionManager, worker *worker.Context, data interaction.ModalSubmitInteraction, responseCh chan button.Response) bool {
	// Safety checks
	if data.GuildId.Value != 0 && data.Member == nil {
		return false
	}

	if data.GuildId.Value == 0 && data.User == nil {
		return false
	}

	lookupCtx, cancelLookupCtx := context.WithTimeout(ctx, time.Second*2)
	defer cancelLookupCtx()

	handler := manager.MatchModal(data.Data.CustomId)
	if handler == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(ctx, handler.Properties().Timeout)

	cc := cmdcontext.NewModalContext(ctx, worker, data, responseCh)
	shouldExecute, canEdit := doPropertiesChecks(lookupCtx, data.GuildId.Value, cc, handler.Properties())
	if shouldExecute {
		go func() {
			defer cancel()
			handler.Execute(cc)
		}()
	} else {
		cancel()
	}

	return canEdit
}

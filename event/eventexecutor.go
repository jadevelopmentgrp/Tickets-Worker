package event

import (
	"errors"
	"fmt"

	"github.com/jadevelopmentgrp/Tickets-Worker"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/listeners"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/metrics/prometheus"
	"github.com/rxdn/gdl/gateway/payloads"
)

func execute(c *worker.Context, event []byte) error {
	var payload payloads.Payload
	if err := json.Unmarshal(event, &payload); err != nil {
		return errors.New(fmt.Sprintf("error whilst decoding event data: %s (data: %s)", err.Error(), string(event)))
	}

	prometheus.Events.WithLabelValues(payload.EventName).Inc()

	if err := listeners.HandleEvent(c, payload); err != nil {
		return err
	}

	return nil
}

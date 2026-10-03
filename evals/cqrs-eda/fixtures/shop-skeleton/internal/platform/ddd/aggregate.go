// Package ddd provides DDD building blocks.
package ddd

import (
	"uuid"

	"github.com/acme/shop/internal/platform/events"
)

// AggregateRoot records the domain events raised by an aggregate until the
// command handler pulls them. Embed it in aggregates; the zero value is usable.
//
// Aggregates are not safe for concurrent use: load, mutate and save one per
// command.
type AggregateRoot struct {
	id     uuid.UUID
	events []events.Event
}

// NewAggregateRoot returns an AggregateRoot with the given identity.
func NewAggregateRoot(id uuid.UUID) AggregateRoot {
	return AggregateRoot{id: id}
}

// ID returns the aggregate identity.
func (a *AggregateRoot) ID() uuid.UUID { return a.id }

// RecordEvent stores an event raised by a state change.
func (a *AggregateRoot) RecordEvent(e events.Event) {
	a.events = append(a.events, e)
}

// PullEvents returns the recorded events and clears them.
func (a *AggregateRoot) PullEvents() []events.Event {
	evs := a.events
	a.events = nil
	return evs
}

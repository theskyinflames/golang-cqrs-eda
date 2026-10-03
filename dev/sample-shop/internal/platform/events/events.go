// Package events defines domain events and how they are handled.
package events

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/acme/shop/internal/platform/bus"
)

// Event is a domain event: a fact that already happened, named in past tense.
type Event interface {
	ID() uuid.UUID
	Name() string
	AggregateID() uuid.UUID
	OccurredAt() time.Time
}

// Base implements Event. Embed it in concrete event types:
//
//	type OrderPlaced struct {
//		events.Base
//		Total int64
//	}
type Base struct {
	id          uuid.UUID
	name        string
	aggregateID uuid.UUID
	occurredAt  time.Time
}

// NewBase returns a Base with a time-ordered ID and the current UTC time.
func NewBase(name string, aggregateID uuid.UUID) Base {
	return Base{
		id:          uuid.NewV7(),
		name:        name,
		aggregateID: aggregateID,
		occurredAt:  time.Now().UTC(),
	}
}

func (b Base) ID() uuid.UUID          { return b.id }
func (b Base) Name() string           { return b.name }
func (b Base) AggregateID() uuid.UUID { return b.aggregateID }
func (b Base) OccurredAt() time.Time  { return b.occurredAt }

// Handler reacts to an event of type E. It usually maps the event to a
// command and dispatches it.
type Handler[E Event] interface {
	Handle(ctx context.Context, e E) error
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc[E Event] func(ctx context.Context, e E) error

// Handle implements Handler.
func (f HandlerFunc[E]) Handle(ctx context.Context, e E) error { return f(ctx, e) }

// ErrUnexpectedEvent is returned when a handler receives an event of another type.
var ErrUnexpectedEvent = errors.New("unexpected event")

// Register subscribes handlers to the event called name on r. All handlers
// run in order; their errors are joined. Each event name is registered once.
func Register[E Event](r bus.Registry, name string, hs ...Handler[E]) error {
	return r.Register(name, func(ctx context.Context, d bus.Dispatchable) (any, error) {
		e, ok := d.(E)
		if !ok {
			return nil, fmt.Errorf("%w: %s got %T", ErrUnexpectedEvent, name, d)
		}
		var errs []error
		for _, h := range hs {
			if err := h.Handle(ctx, e); err != nil {
				errs = append(errs, err)
			}
		}
		return nil, errors.Join(errs...)
	})
}

// dispatch sends e to its handlers. An event nobody subscribes to is not an
// error: publishers must not depend on who listens.
func dispatch(ctx context.Context, d bus.Dispatcher, e Event) error {
	if _, err := d.Dispatch(ctx, e); err != nil && !errors.Is(err, bus.ErrNotDispatchable) {
		return err
	}
	return nil
}

// Publish dispatches each event to d and joins the errors. Events without
// subscribers are skipped.
func Publish(ctx context.Context, d bus.Dispatcher, evs ...Event) error {
	var errs []error
	for _, e := range evs {
		if err := dispatch(ctx, d, e); err != nil {
			errs = append(errs, fmt.Errorf("publish %s %s: %w", e.Name(), e.ID(), err))
		}
	}
	return errors.Join(errs...)
}

// Listen dispatches every event received on ch to d until ctx is done or ch
// is closed. Use it to bridge an external source (a broker subscription) to
// the in-process event bus. Events without subscribers are skipped. onErr may
// be nil.
func Listen(ctx context.Context, ch <-chan Event, d bus.Dispatcher, onErr func(error)) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			if err := dispatch(ctx, d, e); err != nil && onErr != nil {
				onErr(fmt.Errorf("listen %s %s: %w", e.Name(), e.ID(), err))
			}
		}
	}
}

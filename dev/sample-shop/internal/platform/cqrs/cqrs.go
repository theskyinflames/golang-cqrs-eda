// Package cqrs separates operations that change state (commands) from those
// that read it (queries), and wires their handlers to a bus.
package cqrs

import (
	"context"
	"errors"
	"fmt"

	"github.com/acme/shop/internal/platform/bus"
	"github.com/acme/shop/internal/platform/events"
)

// Command is an intent to change state, named in imperative form.
// Name must return a constant and not depend on fields, because
// RegisterCommand calls it on the zero value.
type Command interface {
	Name() string
}

// CommandHandler executes a command and returns the domain events it raised.
type CommandHandler[C Command] interface {
	Handle(ctx context.Context, cmd C) ([]events.Event, error)
}

// CommandHandlerFunc adapts a function to CommandHandler.
type CommandHandlerFunc[C Command] func(ctx context.Context, cmd C) ([]events.Event, error)

// Handle implements CommandHandler.
func (f CommandHandlerFunc[C]) Handle(ctx context.Context, cmd C) ([]events.Event, error) {
	return f(ctx, cmd)
}

// Query is a request to read state without changing it.
// Name must return a constant and not depend on fields, because
// RegisterQuery calls it on the zero value.
type Query interface {
	Name() string
}

// QueryHandler answers a query with a result of type R.
type QueryHandler[Q Query, R any] interface {
	Handle(ctx context.Context, q Q) (R, error)
}

// QueryHandlerFunc adapts a function to QueryHandler.
type QueryHandlerFunc[Q Query, R any] func(ctx context.Context, q Q) (R, error)

// Handle implements QueryHandler.
func (f QueryHandlerFunc[Q, R]) Handle(ctx context.Context, q Q) (R, error) { return f(ctx, q) }

var (
	// ErrUnexpectedMessage is returned when a handler receives a message of another type.
	ErrUnexpectedMessage = errors.New("unexpected message")
	// ErrUnexpectedResult is returned by Ask when the result is not of the requested type.
	ErrUnexpectedResult = errors.New("unexpected result")
)

// RegisterCommand registers h on r under the command's name.
func RegisterCommand[C Command](r bus.Registry, h CommandHandler[C]) error {
	var zero C
	name := zero.Name()
	return r.Register(name, func(ctx context.Context, d bus.Dispatchable) (any, error) {
		cmd, ok := d.(C)
		if !ok {
			return nil, fmt.Errorf("%w: %s got %T", ErrUnexpectedMessage, name, d)
		}
		return h.Handle(ctx, cmd)
	})
}

// RegisterQuery registers h on r under the query's name.
func RegisterQuery[Q Query, R any](r bus.Registry, h QueryHandler[Q, R]) error {
	var zero Q
	name := zero.Name()
	return r.Register(name, func(ctx context.Context, d bus.Dispatchable) (any, error) {
		q, ok := d.(Q)
		if !ok {
			return nil, fmt.Errorf("%w: %s got %T", ErrUnexpectedMessage, name, d)
		}
		return h.Handle(ctx, q)
	})
}

// Send dispatches a command and returns the events it raised.
func Send(ctx context.Context, d bus.Dispatcher, cmd Command) ([]events.Event, error) {
	v, err := d.Dispatch(ctx, cmd)
	evs, _ := v.([]events.Event)
	return evs, err
}

// Ask dispatches a query and returns its typed result.
func Ask[R any](ctx context.Context, d bus.Dispatcher, q Query) (R, error) {
	var zero R
	v, err := d.Dispatch(ctx, q)
	if err != nil || v == nil {
		return zero, err
	}
	r, ok := v.(R)
	if !ok {
		return zero, fmt.Errorf("%w: %s want %T got %T", ErrUnexpectedResult, q.Name(), zero, v)
	}
	return r, nil
}

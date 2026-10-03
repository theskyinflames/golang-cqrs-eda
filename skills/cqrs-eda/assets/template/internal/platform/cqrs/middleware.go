package cqrs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"example.com/app/internal/platform/bus"
	"example.com/app/internal/platform/events"
)

// CommandFunc is the untyped form of a command handler that middlewares wrap.
type CommandFunc func(ctx context.Context, cmd Command) ([]events.Event, error)

// CommandMiddleware decorates command handling with a cross-cutting concern.
type CommandMiddleware func(next CommandFunc) CommandFunc

// WrapCommand applies mws to h. The first middleware is the outermost:
// WrapCommand(h, a, b) runs a → b → h.
func WrapCommand[C Command](h CommandHandler[C], mws ...CommandMiddleware) CommandHandler[C] {
	next := CommandFunc(func(ctx context.Context, cmd Command) ([]events.Event, error) {
		c, ok := cmd.(C)
		if !ok {
			return nil, fmt.Errorf("%w: %s got %T", ErrUnexpectedMessage, cmd.Name(), cmd)
		}
		return h.Handle(ctx, c)
	})
	for _, mw := range slices.Backward(mws) {
		next = mw(next)
	}
	return CommandHandlerFunc[C](func(ctx context.Context, cmd C) ([]events.Event, error) {
		return next(ctx, cmd)
	})
}

// QueryFunc is the untyped form of a query handler that middlewares wrap.
type QueryFunc func(ctx context.Context, q Query) (any, error)

// QueryMiddleware decorates query handling with a cross-cutting concern.
type QueryMiddleware func(next QueryFunc) QueryFunc

// WrapQuery applies mws to h. The first middleware is the outermost.
func WrapQuery[Q Query, R any](h QueryHandler[Q, R], mws ...QueryMiddleware) QueryHandler[Q, R] {
	next := QueryFunc(func(ctx context.Context, q Query) (any, error) {
		qq, ok := q.(Q)
		if !ok {
			return nil, fmt.Errorf("%w: %s got %T", ErrUnexpectedMessage, q.Name(), q)
		}
		return h.Handle(ctx, qq)
	})
	for _, mw := range slices.Backward(mws) {
		next = mw(next)
	}
	return QueryHandlerFunc[Q, R](func(ctx context.Context, q Q) (R, error) {
		var zero R
		v, err := next(ctx, q)
		if err != nil || v == nil {
			return zero, err
		}
		r, ok := v.(R)
		if !ok {
			return zero, fmt.Errorf("%w: %s want %T got %T", ErrUnexpectedResult, q.Name(), zero, v)
		}
		return r, nil
	})
}

// LogCommandErrors logs failed commands by name, never the payload, to keep
// personal data out of logs. Errors matching one of expected (errors.Is) are
// normal rejections — not found, rule violations, invalid input — and are
// logged at Info as "command rejected"; anything else is logged at Error.
func LogCommandErrors(l *slog.Logger, expected ...error) CommandMiddleware {
	return func(next CommandFunc) CommandFunc {
		return func(ctx context.Context, cmd Command) ([]events.Event, error) {
			evs, err := next(ctx, cmd)
			if err != nil {
				logErr(ctx, l, "command", cmd.Name(), err, expected)
			}
			return evs, err
		}
	}
}

// LogQueryErrors logs failed queries by name. expected works as in
// LogCommandErrors.
func LogQueryErrors(l *slog.Logger, expected ...error) QueryMiddleware {
	return func(next QueryFunc) QueryFunc {
		return func(ctx context.Context, q Query) (any, error) {
			v, err := next(ctx, q)
			if err != nil {
				logErr(ctx, l, "query", q.Name(), err, expected)
			}
			return v, err
		}
	}
}

func logErr(ctx context.Context, l *slog.Logger, kind, name string, err error, expected []error) {
	for _, e := range expected {
		if errors.Is(err, e) {
			l.InfoContext(ctx, kind+" rejected", kind, name, "error", err)
			return
		}
	}
	l.ErrorContext(ctx, kind+" failed", kind, name, "error", err)
}

// ErrEventsNotPublished means the command succeeded (state is changed) but
// some of its events could not be published. Check it with errors.Is.
var ErrEventsNotPublished = errors.New("events not published")

// PublishEvents dispatches the events of a successful command to d.
// Place it outside WithUnitOfWork so events leave only after commit.
func PublishEvents(d bus.Dispatcher) CommandMiddleware {
	return func(next CommandFunc) CommandFunc {
		return func(ctx context.Context, cmd Command) ([]events.Event, error) {
			evs, err := next(ctx, cmd)
			if err != nil {
				return evs, err
			}
			if err := events.Publish(ctx, d, evs...); err != nil {
				return evs, fmt.Errorf("%s: %w: %w", cmd.Name(), ErrEventsNotPublished, err)
			}
			return evs, nil
		}
	}
}

// UnitOfWork runs fn atomically: it commits if fn returns nil and rolls back
// otherwise. Implementations usually begin a DB transaction and store it in
// the context passed to fn, where repositories pick it up.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// WithUnitOfWork runs each command inside a unit of work.
func WithUnitOfWork(u UnitOfWork) CommandMiddleware {
	return func(next CommandFunc) CommandFunc {
		return func(ctx context.Context, cmd Command) ([]events.Event, error) {
			var evs []events.Event
			err := u.Do(ctx, func(ctx context.Context) error {
				var err error
				evs, err = next(ctx, cmd)
				return err
			})
			if err != nil {
				return nil, err
			}
			return evs, nil
		}
	}
}

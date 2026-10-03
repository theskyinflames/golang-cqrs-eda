package cqrs_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/acme/shop/internal/platform/bus"
	"github.com/acme/shop/internal/platform/cqrs"
	"github.com/acme/shop/internal/platform/events"
)

type cmd struct{}

func (cmd) Name() string { return "cmd" }

type qry struct{}

func (qry) Name() string { return "qry" }

func raise(context.Context, cmd) ([]events.Event, error) {
	return []events.Event{events.NewBase("evt", uuid.Nil())}, nil
}

func trace(log *[]string, name string) cqrs.CommandMiddleware {
	return func(next cqrs.CommandFunc) cqrs.CommandFunc {
		return func(ctx context.Context, c cqrs.Command) ([]events.Event, error) {
			*log = append(*log, name+">")
			evs, err := next(ctx, c)
			*log = append(*log, "<"+name)
			return evs, err
		}
	}
}

func TestWrapCommandOrder(t *testing.T) {
	var log []string
	h := cqrs.WrapCommand(cqrs.CommandHandlerFunc[cmd](raise), trace(&log, "a"), trace(&log, "b"))
	if _, err := h.Handle(t.Context(), cmd{}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(log, " "); got != "a> b> <b <a" {
		t.Fatalf("order %q", got)
	}
}

type recordingUoW struct{ committed, rolledBack bool }

func (u *recordingUoW) Do(ctx context.Context, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		u.rolledBack = true
		return err
	}
	u.committed = true
	return nil
}

func TestWithUnitOfWork(t *testing.T) {
	errBoom := errors.New("boom")
	fail := cqrs.CommandHandlerFunc[cmd](func(context.Context, cmd) ([]events.Event, error) {
		return nil, errBoom
	})

	u := &recordingUoW{}
	if _, err := cqrs.WrapCommand(fail, cqrs.WithUnitOfWork(u)).Handle(t.Context(), cmd{}); !errors.Is(err, errBoom) || !u.rolledBack {
		t.Fatalf("err %v, rolled back %v", err, u.rolledBack)
	}

	u = &recordingUoW{}
	evs, err := cqrs.WrapCommand(cqrs.CommandHandlerFunc[cmd](raise), cqrs.WithUnitOfWork(u)).Handle(t.Context(), cmd{})
	if err != nil || !u.committed || len(evs) != 1 {
		t.Fatalf("err %v, committed %v, events %d", err, u.committed, len(evs))
	}
}

func TestPublishEvents(t *testing.T) {
	var noSubscribers bus.Bus
	h := cqrs.WrapCommand(cqrs.CommandHandlerFunc[cmd](raise), cqrs.PublishEvents(&noSubscribers))
	if _, err := h.Handle(t.Context(), cmd{}); err != nil {
		t.Fatalf("events without subscribers must not fail, got %v", err)
	}

	var eventBus bus.Bus
	errBoom := errors.New("boom")
	_ = eventBus.Register("evt", func(context.Context, bus.Dispatchable) (any, error) { return nil, errBoom })
	h = cqrs.WrapCommand(cqrs.CommandHandlerFunc[cmd](raise), cqrs.PublishEvents(&eventBus))
	evs, err := h.Handle(t.Context(), cmd{})
	if !errors.Is(err, cqrs.ErrEventsNotPublished) || !errors.Is(err, errBoom) {
		t.Fatalf("got %v", err)
	}
	if len(evs) != 1 {
		t.Fatal("events must still be returned")
	}
}

func TestRegisterAndDispatch(t *testing.T) {
	var b bus.Bus
	if err := cqrs.RegisterCommand(&b, cqrs.CommandHandlerFunc[cmd](raise)); err != nil {
		t.Fatal(err)
	}
	if err := cqrs.RegisterQuery(&b, cqrs.QueryHandlerFunc[qry, string](
		func(context.Context, qry) (string, error) { return "ok", nil })); err != nil {
		t.Fatal(err)
	}

	if evs, err := cqrs.Send(t.Context(), &b, cmd{}); err != nil || len(evs) != 1 {
		t.Fatalf("send: %v, %d events", err, len(evs))
	}
	if r, err := cqrs.Ask[string](t.Context(), &b, qry{}); err != nil || r != "ok" {
		t.Fatalf("ask: %q, %v", r, err)
	}
	if _, err := cqrs.Ask[int](t.Context(), &b, qry{}); !errors.Is(err, cqrs.ErrUnexpectedResult) {
		t.Fatalf("want ErrUnexpectedResult, got %v", err)
	}
}

package events_test

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/acme/shop/internal/platform/bus"
	"github.com/acme/shop/internal/platform/events"
)

const userCreatedName = "user.created"

type userCreated struct {
	events.Base
	Email string
}

func newUserCreated(email string) userCreated {
	return userCreated{Base: events.NewBase(userCreatedName, uuid.NewV7()), Email: email}
}

type other struct{ events.Base }

func TestRegisterAndPublish(t *testing.T) {
	var b bus.Bus
	var got []string
	errBoom := errors.New("boom")
	err := events.Register(&b, userCreatedName,
		events.HandlerFunc[userCreated](func(_ context.Context, e userCreated) error {
			got = append(got, "h1:"+e.Email)
			return nil
		}),
		events.HandlerFunc[userCreated](func(_ context.Context, e userCreated) error {
			got = append(got, "h2:"+e.Email)
			return errBoom
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = events.Publish(t.Context(), &b, newUserCreated("a@b.c"))
	if !errors.Is(err, errBoom) {
		t.Fatalf("want errBoom, got %v", err)
	}
	if len(got) != 2 || got[0] != "h1:a@b.c" || got[1] != "h2:a@b.c" {
		t.Fatalf("handlers ran %v", got)
	}

	wrongType := other{events.NewBase(userCreatedName, uuid.Nil())}
	if err := events.Publish(t.Context(), &b, wrongType); !errors.Is(err, events.ErrUnexpectedEvent) {
		t.Fatalf("want ErrUnexpectedEvent, got %v", err)
	}
}

func TestPublishWithoutSubscribers(t *testing.T) {
	var b bus.Bus
	if err := events.Publish(t.Context(), &b, newUserCreated("a@b.c")); err != nil {
		t.Fatalf("an event nobody listens to must not fail, got %v", err)
	}
}

func TestListen(t *testing.T) {
	var b bus.Bus
	errBoom := errors.New("boom")
	received := 0
	_ = events.Register(&b, userCreatedName, events.HandlerFunc[userCreated](
		func(_ context.Context, e userCreated) error {
			received++
			if e.Email == "fail" {
				return errBoom
			}
			return nil
		}))

	ch := make(chan events.Event, 3)
	ch <- newUserCreated("a@b.c")
	ch <- other{events.NewBase("unknown", uuid.Nil())} // no subscribers: skipped
	ch <- newUserCreated("fail")
	close(ch)

	var errs []error
	events.Listen(t.Context(), ch, &b, func(err error) { errs = append(errs, err) })

	if received != 2 {
		t.Fatalf("received %d", received)
	}
	if len(errs) != 1 || !errors.Is(errs[0], errBoom) {
		t.Fatalf("errors %v", errs)
	}
}

package ddd_test

import (
	"testing"
	"uuid"

	"example.com/app/internal/platform/ddd"
	"example.com/app/internal/platform/events"
)

func TestAggregateRoot(t *testing.T) {
	var zero ddd.AggregateRoot // zero value is usable
	zero.RecordEvent(events.NewBase("x", uuid.Nil()))
	if n := len(zero.PullEvents()); n != 1 {
		t.Fatalf("got %d events", n)
	}

	id := uuid.NewV7()
	a := ddd.NewAggregateRoot(id)
	if a.ID() != id {
		t.Fatal("wrong id")
	}
	a.RecordEvent(events.NewBase("a", id))
	a.RecordEvent(events.NewBase("b", id))
	evs := a.PullEvents()
	if len(evs) != 2 || evs[0].Name() != "a" || evs[1].Name() != "b" {
		t.Fatalf("got %v", evs)
	}
	if len(a.PullEvents()) != 0 {
		t.Fatal("events not cleared")
	}
}

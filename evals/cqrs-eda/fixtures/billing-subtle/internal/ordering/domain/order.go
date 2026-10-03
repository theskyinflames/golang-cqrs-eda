package domain

import (
	"context"
	"errors"
	"uuid"

	"github.com/acme/shop/internal/platform/ddd"
	"github.com/acme/shop/internal/platform/events"
)

// Event names are constants in past tense, prefixed by the bounded context.
const OrderPlacedName = "ordering.order_placed"

// OrderPlaced is raised when an order is accepted.
type OrderPlaced struct {
	events.Base
	Total int64
}

var (
	ErrInvalidTotal = errors.New("total must be positive")
	ErrNotFound     = errors.New("order not found")
)

// Order is an aggregate: it guards its invariants and records what happened.
type Order struct {
	ddd.AggregateRoot
	total int64
}

// PlaceOrder is the only way to create a valid Order.
func PlaceOrder(id uuid.UUID, total int64) (*Order, error) {
	if total <= 0 {
		return nil, ErrInvalidTotal
	}
	o := &Order{AggregateRoot: ddd.NewAggregateRoot(id), total: total}
	o.RecordEvent(OrderPlaced{Base: events.NewBase(OrderPlacedName, id), Total: total})
	return o, nil
}

func (o *Order) Total() int64 { return o.total }

// OrderRepository is a port: the domain defines it, infra implements it.
type OrderRepository interface {
	Save(ctx context.Context, o *Order) error
}

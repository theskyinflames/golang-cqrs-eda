package app

import (
	"context"
	"fmt"
	"uuid"

	"github.com/acme/shop/internal/ordering/domain"
	"github.com/acme/shop/internal/platform/events"
)

// PlaceOrder is a command: imperative name, plain data, no behavior.
type PlaceOrder struct {
	OrderID uuid.UUID
	Total   int64
}

func (PlaceOrder) Name() string { return "ordering.place_order" }

// PlaceOrderHandler loads/creates one aggregate, saves it and returns its events.
type PlaceOrderHandler struct {
	Orders domain.OrderRepository
}

func (h PlaceOrderHandler) Handle(ctx context.Context, cmd PlaceOrder) ([]events.Event, error) {
	o, err := domain.PlaceOrder(cmd.OrderID, cmd.Total)
	if err != nil {
		return nil, err
	}
	if err := h.Orders.Save(ctx, o); err != nil {
		return nil, fmt.Errorf("save order %s: %w", o.ID(), err)
	}
	return o.PullEvents(), nil
}

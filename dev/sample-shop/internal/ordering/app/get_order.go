package app

import (
	"context"
	"uuid"
)

// GetOrder is a query. Its result is a read model (DTO), never the aggregate.
type GetOrder struct{ OrderID uuid.UUID }

func (GetOrder) Name() string { return "ordering.get_order" }

type OrderView struct {
	ID    uuid.UUID `json:"id"`
	Total int64     `json:"total"`
}

// OrderReader is the read-side port. It may query the DB directly and
// skip the aggregate: queries don't need invariants.
type OrderReader interface {
	OrderByID(ctx context.Context, id uuid.UUID) (OrderView, error)
}

type GetOrderHandler struct{ Orders OrderReader }

func (h GetOrderHandler) Handle(ctx context.Context, q GetOrder) (OrderView, error) {
	return h.Orders.OrderByID(ctx, q.OrderID)
}

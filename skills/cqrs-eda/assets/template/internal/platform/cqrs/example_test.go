package cqrs_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"uuid"

	"example.com/app/internal/platform/bus"
	"example.com/app/internal/platform/cqrs"
	"example.com/app/internal/platform/ddd"
	"example.com/app/internal/platform/events"
)

// --- domain ---

const orderPlacedName = "order.placed"

type OrderPlaced struct {
	events.Base
	Total int64
}

type Order struct {
	ddd.AggregateRoot
	total int64
}

func PlaceOrder(id uuid.UUID, total int64) (*Order, error) {
	if total <= 0 {
		return nil, errors.New("total must be positive")
	}
	o := &Order{AggregateRoot: ddd.NewAggregateRoot(id), total: total}
	o.RecordEvent(OrderPlaced{Base: events.NewBase(orderPlacedName, id), Total: total})
	return o, nil
}

// --- application ---

type PlaceOrderCmd struct {
	ID    uuid.UUID
	Total int64
}

func (PlaceOrderCmd) Name() string { return "order.place" }

type GetOrderTotal struct{ ID uuid.UUID }

func (GetOrderTotal) Name() string { return "order.get_total" }

type OrderRepo map[uuid.UUID]*Order

type PlaceOrderHandler struct{ repo OrderRepo }

func (h PlaceOrderHandler) Handle(_ context.Context, cmd PlaceOrderCmd) ([]events.Event, error) {
	o, err := PlaceOrder(cmd.ID, cmd.Total)
	if err != nil {
		return nil, err
	}
	h.repo[o.ID()] = o
	return o.PullEvents(), nil
}

type GetOrderTotalHandler struct{ repo OrderRepo }

func (h GetOrderTotalHandler) Handle(_ context.Context, q GetOrderTotal) (int64, error) {
	o, ok := h.repo[q.ID]
	if !ok {
		return 0, errors.New("order not found")
	}
	return o.total, nil
}

// noTx is a UnitOfWork stand-in; production code wraps a DB transaction.
type noTx struct{}

func (noTx) Do(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// Example wires a command bus, a query bus and an event bus the way
// cmd/<service>/main.go would.
func Example() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}))
	repo := OrderRepo{}

	// Event bus: events usually become commands; here we just print them.
	eventBus := bus.New()
	_ = events.Register(eventBus, orderPlacedName,
		events.HandlerFunc[OrderPlaced](func(_ context.Context, e OrderPlaced) error {
			fmt.Println("event:", e.Name(), e.Total)
			return nil
		}))

	// Command bus: log → publish events (after commit) → transaction → handler.
	commandBus := bus.New()
	_ = cqrs.RegisterCommand(commandBus, cqrs.WrapCommand(
		PlaceOrderHandler{repo},
		cqrs.LogCommandErrors(logger),
		cqrs.PublishEvents(eventBus),
		cqrs.WithUnitOfWork(noTx{}),
	))

	queryBus := bus.New()
	_ = cqrs.RegisterQuery(queryBus, cqrs.WrapQuery(
		GetOrderTotalHandler{repo},
		cqrs.LogQueryErrors(logger),
	))

	id := uuid.NewV7()
	if _, err := cqrs.Send(ctx, commandBus, PlaceOrderCmd{ID: id, Total: 42}); err != nil {
		fmt.Println(err)
	}
	total, err := cqrs.Ask[int64](ctx, queryBus, GetOrderTotal{ID: id})
	fmt.Println("total:", total, err)

	_, _ = cqrs.Send(ctx, commandBus, PlaceOrderCmd{ID: uuid.NewV7(), Total: 0})

	// Output:
	// event: order.placed 42
	// total: 42 <nil>
	// level=ERROR msg="command failed" command=order.place error="total must be positive"
}

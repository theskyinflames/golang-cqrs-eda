package app

import (
	"context"
	"uuid"

	"github.com/acme/shop/internal/ordering/domain"
	"github.com/acme/shop/internal/platform/bus"
	"github.com/acme/shop/internal/platform/cqrs"
	"github.com/acme/shop/internal/platform/events"
)

type ScheduleShipment struct{ OrderID uuid.UUID }

func (ScheduleShipment) Name() string { return "shipping.schedule_shipment" }

// ShipWhenOrderPlaced is a policy: "when OrderPlaced, then ScheduleShipment".
// It translates the event into a command and sends it; it holds no logic.
type ShipWhenOrderPlaced struct {
	Commands bus.Dispatcher
}

func (p ShipWhenOrderPlaced) Handle(ctx context.Context, e domain.OrderPlaced) error {
	_, err := cqrs.Send(ctx, p.Commands, ScheduleShipment{OrderID: e.AggregateID()})
	return err
}

var _ events.Handler[domain.OrderPlaced] = ShipWhenOrderPlaced{}

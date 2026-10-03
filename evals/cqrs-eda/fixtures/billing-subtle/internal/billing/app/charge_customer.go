package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/acme/shop/internal/billing/domain"
	"github.com/acme/shop/internal/platform/bus"
	"github.com/acme/shop/internal/platform/cqrs"
	"github.com/acme/shop/internal/platform/events"
)

type ChargeCustomer struct {
	CustomerID uuid.UUID
	Amount     int64
}

func (ChargeCustomer) Name() string { return "billing.charge_customer" }

type ChargeCustomerHandler struct{ Customers domain.CustomerRepository }

func (h ChargeCustomerHandler) Handle(ctx context.Context, cmd ChargeCustomer) ([]events.Event, error) {
	c, err := h.Customers.ByID(ctx, cmd.CustomerID)
	if err != nil {
		return nil, err
	}
	c.Charge(cmd.Amount)
	if err := h.Customers.Save(ctx, c); err != nil {
		return nil, err
	}
	return c.PullEvents(), nil
}

// ChargeCustomerWhenInvoiceIssued is a policy.
type ChargeCustomerWhenInvoiceIssued struct{ Commands bus.Dispatcher }

func (p ChargeCustomerWhenInvoiceIssued) Handle(ctx context.Context, e domain.IssueInvoiceEvent) error {
	_, err := cqrs.Send(ctx, p.Commands, ChargeCustomer{CustomerID: e.CustomerID, Amount: e.Amount})
	return err
}

// NotifyAccountingWhenInvoiceIssued tells accounting about new invoices.
type NotifyAccountingWhenInvoiceIssued struct{ Log *slog.Logger }

func (p NotifyAccountingWhenInvoiceIssued) Handle(ctx context.Context, e domain.IssueInvoiceEvent) error {
	p.Log.InfoContext(ctx, "notify accounting", "customer", e.CustomerID, "amount", e.Amount)
	return nil
}

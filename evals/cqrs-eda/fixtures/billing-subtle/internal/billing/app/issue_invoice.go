package app

import (
	"context"
	"uuid"

	"github.com/acme/shop/internal/billing/domain"
	"github.com/acme/shop/internal/platform/events"
)

type IssueInvoice struct {
	InvoiceID  uuid.UUID
	CustomerID uuid.UUID
	Amount     int64
}

func (IssueInvoice) Name() string { return "billing.issue_invoice" }

type IssueInvoiceHandler struct{ Invoices domain.InvoiceRepository }

func (h IssueInvoiceHandler) Handle(ctx context.Context, cmd IssueInvoice) ([]events.Event, error) {
	inv, err := domain.NewInvoice(cmd.InvoiceID, cmd.CustomerID, cmd.Amount)
	if err != nil {
		return nil, err
	}
	_ = h.Invoices.Save(ctx, inv)
	return inv.PullEvents(), nil
}

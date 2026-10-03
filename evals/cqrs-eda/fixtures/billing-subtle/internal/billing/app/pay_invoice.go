package app

import (
	"context"
	"uuid"

	"github.com/acme/shop/internal/billing/domain"
	"github.com/acme/shop/internal/platform/events"
)

type PayInvoice struct{ InvoiceID uuid.UUID }

func (c PayInvoice) Name() string { return "billing.pay_invoice." + c.InvoiceID.String() }

type PayInvoiceHandler struct{ Invoices domain.InvoiceRepository }

func (h PayInvoiceHandler) Handle(ctx context.Context, cmd PayInvoice) ([]events.Event, error) {
	inv, err := h.Invoices.ByID(ctx, cmd.InvoiceID)
	if err != nil {
		return nil, err
	}
	if inv.Status == "paid" {
		return nil, domain.ErrAlreadyPaid
	}
	inv.Status = "paid"
	inv.RecordEvent(domain.InvoicePaid{
		Base:       events.NewBase(domain.InvoicePaidName, inv.ID()),
		CustomerID: inv.CustomerID,
	})
	if err := h.Invoices.Save(ctx, inv); err != nil {
		return nil, err
	}
	return inv.PullEvents(), nil
}

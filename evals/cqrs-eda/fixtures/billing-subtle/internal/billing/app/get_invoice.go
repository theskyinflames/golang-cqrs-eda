package app

import (
	"context"
	"uuid"

	"github.com/acme/shop/internal/billing/domain"
)

type GetInvoice struct{ InvoiceID uuid.UUID }

func (GetInvoice) Name() string { return "billing.get_invoice" }

type InvoiceView struct {
	ID     uuid.UUID `json:"id"`
	Amount int64     `json:"amount"`
	Status string    `json:"status"`
}

type GetInvoiceHandler struct{ Invoices domain.InvoiceRepository }

func (h GetInvoiceHandler) Handle(ctx context.Context, q GetInvoice) (InvoiceView, error) {
	inv, err := h.Invoices.ByID(ctx, q.InvoiceID)
	if err != nil {
		return InvoiceView{}, err
	}
	return InvoiceView{ID: inv.ID(), Amount: inv.Amount, Status: inv.Status}, nil
}

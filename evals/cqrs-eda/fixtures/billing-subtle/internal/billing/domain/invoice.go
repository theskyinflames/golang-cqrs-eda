package domain

import (
	"context"
	"errors"
	"uuid"

	"github.com/acme/shop/internal/platform/ddd"
	"github.com/acme/shop/internal/platform/events"
)

const (
	InvoiceIssuedName = "billing.issue_invoice"
	InvoicePaidName   = "billing.invoice_paid"
)

type IssueInvoiceEvent struct {
	events.Base
	CustomerID uuid.UUID
	Amount     int64
}

type InvoicePaid struct {
	events.Base
	CustomerID uuid.UUID
}

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidAmount = errors.New("amount must be positive")
	ErrAlreadyPaid   = errors.New("invoice already paid")
)

type Invoice struct {
	ddd.AggregateRoot
	CustomerID uuid.UUID
	Amount     int64
	Status     string
}

func NewInvoice(id, customerID uuid.UUID, amount int64) (*Invoice, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	inv := &Invoice{AggregateRoot: ddd.NewAggregateRoot(id), CustomerID: customerID, Amount: amount, Status: "issued"}
	inv.RecordEvent(IssueInvoiceEvent{
		Base:       events.NewBase(InvoiceIssuedName, uuid.Nil()),
		CustomerID: customerID,
		Amount:     amount,
	})
	return inv, nil
}

type Customer struct {
	ddd.AggregateRoot
	balance int64
}

func (c *Customer) Charge(amount int64) { c.balance += amount }

func (c *Customer) Balance() int64 { return c.balance }

type InvoiceRepository interface {
	Save(ctx context.Context, inv *Invoice) error
	ByID(ctx context.Context, id uuid.UUID) (*Invoice, error)
}

type CustomerRepository interface {
	Save(ctx context.Context, c *Customer) error
	ByID(ctx context.Context, id uuid.UUID) (*Customer, error)
}

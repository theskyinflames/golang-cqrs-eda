// Package domain holds the ordering bounded context's domain model.
package domain

import (
	"uuid"

	"github.com/acme/shop/internal/platform/ddd"
)

// Order is a customer order.
type Order struct {
	ddd.AggregateRoot
	customerID uuid.UUID
	total      int64 // cents
}

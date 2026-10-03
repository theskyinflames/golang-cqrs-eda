// Package memory is an in-memory adapter for tests and prototypes.
package memory

import (
	"context"
	"sync"
	"uuid"

	"github.com/acme/shop/internal/ordering/app"
	"github.com/acme/shop/internal/ordering/domain"
)

// record is the stored state. Never store *domain.Order itself: it would keep
// pending events (republished on the next load) and share pointers.
type record struct {
	total int64
}

// Orders implements domain.OrderRepository and app.OrderReader in memory.
type Orders struct {
	mu sync.RWMutex
	m  map[uuid.UUID]record
}

func (r *Orders) Save(_ context.Context, o *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.m == nil {
		r.m = make(map[uuid.UUID]record)
	}
	r.m[o.ID()] = record{total: o.Total()}
	return nil
}

// ByID loads the aggregate for a command.
func (r *Orders) ByID(_ context.Context, id uuid.UUID) (*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.m[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return domain.RestoreOrder(id, rec.total), nil
}

// OrderByID serves the read side directly from the record.
func (r *Orders) OrderByID(_ context.Context, id uuid.UUID) (app.OrderView, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.m[id]
	if !ok {
		return app.OrderView{}, domain.ErrNotFound
	}
	return app.OrderView{ID: id, Total: rec.total}, nil
}

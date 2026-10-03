package memory

import (
	"context"
	"sync"
	"uuid"

	"github.com/acme/shop/internal/billing/domain"
)

type Invoices struct {
	mu sync.Mutex
	m  map[uuid.UUID]domain.Invoice
}

func (s *Invoices) Save(_ context.Context, inv *domain.Invoice) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[uuid.UUID]domain.Invoice{}
	}
	s.m[inv.ID()] = *inv
	return nil
}

func (s *Invoices) ByID(_ context.Context, id uuid.UUID) (*domain.Invoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.m[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &inv, nil
}

type Customers struct {
	mu sync.Mutex
	m  map[uuid.UUID]domain.Customer
}

func (s *Customers) Save(_ context.Context, c *domain.Customer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[uuid.UUID]domain.Customer{}
	}
	s.m[c.ID()] = *c
	return nil
}

func (s *Customers) ByID(_ context.Context, id uuid.UUID) (*domain.Customer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.m[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &c, nil
}

package app_test

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/acme/shop/internal/ordering/app"
	"github.com/acme/shop/internal/ordering/domain"
)

// fakeOrders is a hand-written fake; no mocking library needed.
type fakeOrders struct {
	saved []*domain.Order
	err   error
}

func (f *fakeOrders) Save(_ context.Context, o *domain.Order) error {
	f.saved = append(f.saved, o)
	return f.err
}

func TestPlaceOrderHandler(t *testing.T) {
	errDB := errors.New("db down")
	tests := []struct {
		name       string
		total      int64
		repoErr    error
		wantErr    error
		wantEvents int
	}{
		{name: "places order", total: 10, wantEvents: 1},
		{name: "rejects invalid total", total: 0, wantErr: domain.ErrInvalidTotal},
		{name: "fails when repo fails", total: 10, repoErr: errDB, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := app.PlaceOrderHandler{Orders: &fakeOrders{err: tt.repoErr}}

			evs, err := h.Handle(t.Context(), app.PlaceOrder{OrderID: uuid.NewV7(), Total: tt.total})

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if len(evs) != tt.wantEvents {
				t.Fatalf("got %d events, want %d", len(evs), tt.wantEvents)
			}
			if tt.wantEvents > 0 {
				if _, ok := evs[0].(domain.OrderPlaced); !ok {
					t.Fatalf("got %T, want OrderPlaced", evs[0])
				}
			}
		})
	}
}

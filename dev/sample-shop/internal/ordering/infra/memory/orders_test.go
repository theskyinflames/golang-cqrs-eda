package memory_test

import (
	"testing"
	"uuid"

	"github.com/acme/shop/internal/ordering/domain"
	"github.com/acme/shop/internal/ordering/infra/memory"
)

func TestLoadedOrderHasNoPendingEvents(t *testing.T) {
	var repo memory.Orders
	o, err := domain.PlaceOrder(uuid.NewV7(), 10) // records OrderPlaced
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(t.Context(), o); err != nil {
		t.Fatal(err)
	}

	loaded, err := repo.ByID(t.Context(), o.ID())
	if err != nil {
		t.Fatal(err)
	}
	if evs := loaded.PullEvents(); len(evs) != 0 {
		t.Fatalf("loaded order replays %d events", len(evs))
	}
}

package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"uuid"

	"github.com/acme/shop/internal/ordering/app"
	"github.com/acme/shop/internal/ordering/domain"
	"github.com/acme/shop/internal/platform/bus"
	"github.com/acme/shop/internal/platform/cqrs"
)

// Routes is a driving adapter: decode → Send/Ask → encode. No business logic.
func Routes(mux *http.ServeMux, commands, queries bus.Dispatcher) {
	mux.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Total int64 `json:"total"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id := uuid.NewV7()
		_, err := cqrs.Send(r.Context(), commands, app.PlaceOrder{OrderID: id, Total: in.Total})
		switch {
		case errors.Is(err, domain.ErrInvalidTotal):
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		case errors.Is(err, cqrs.ErrEventsNotPublished):
			// State changed; only side effects failed. Still a success for the client.
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Location", "/orders/"+id.String())
		w.WriteHeader(http.StatusCreated)
	})

	mux.HandleFunc("GET /orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		v, err := cqrs.Ask[app.OrderView](r.Context(), queries, app.GetOrder{OrderID: id})
		switch {
		case errors.Is(err, domain.ErrNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(v)
	})
}

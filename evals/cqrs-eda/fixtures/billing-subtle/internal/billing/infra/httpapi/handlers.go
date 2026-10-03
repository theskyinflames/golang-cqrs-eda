package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"uuid"

	"github.com/acme/shop/internal/billing/app"
	"github.com/acme/shop/internal/billing/domain"
	"github.com/acme/shop/internal/platform/bus"
	"github.com/acme/shop/internal/platform/cqrs"
)

func Routes(mux *http.ServeMux, commands, queries bus.Dispatcher) {
	mux.HandleFunc("POST /invoices", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			CustomerID uuid.UUID `json:"customer_id"`
			Amount     int64     `json:"amount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id := uuid.NewV7()
		_, err := cqrs.Send(r.Context(), commands, app.IssueInvoice{InvoiceID: id, CustomerID: in.CustomerID, Amount: in.Amount})
		if err != nil && !errors.Is(err, cqrs.ErrEventsNotPublished) {
			writeErr(w, err)
			return
		}
		w.Header().Set("Location", "/invoices/"+id.String())
		w.WriteHeader(http.StatusCreated)
	})

	mux.HandleFunc("POST /invoices/{id}/pay", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, err = cqrs.Send(r.Context(), commands, app.PayInvoice{InvoiceID: id})
		if err != nil && !errors.Is(err, cqrs.ErrEventsNotPublished) {
			writeErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /invoices/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		v, err := cqrs.Ask[*app.InvoiceView](r.Context(), queries, app.GetInvoice{InvoiceID: id})
		if err != nil {
			writeErr(w, err)
			return
		}
		_ = json.NewEncoder(w).Encode(v)
	})
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, domain.ErrInvalidAmount):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrAlreadyPaid):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

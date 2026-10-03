# CQRS/EDA patterns — worked example

A complete, compiling slice of a service (module `github.com/acme/shop`, Go 1.27)
built on the platform packages. Copy the shape, not the names. Every snippet was
verified with `go vet` and `go test`. On Go < 1.27 the import is
`github.com/google/uuid`; write `uuid.Must(uuid.NewV7())` instead of `uuid.NewV7()`.

## Contents

1. Domain: aggregate, event, errors, repository port
2. Command and handler
3. Query, read model and handler
4. Policy: event → command (another bounded context)
5. Unit of work and repositories (database/sql, in-memory)
6. HTTP driving adapter
7. Composition root (`cmd/<service>/main.go`)
8. Testing a command handler

## 1. Domain: aggregate, event, errors, repository port

Pure Go: no SQL, HTTP or bus imports. Invariants and domain errors live here.

`internal/ordering/domain/order.go`

```go
package domain

import (
	"context"
	"errors"
	"uuid"

	"github.com/acme/shop/internal/platform/ddd"
	"github.com/acme/shop/internal/platform/events"
)

// Event names are constants in past tense, prefixed by the bounded context.
const OrderPlacedName = "ordering.order_placed"

// OrderPlaced is raised when an order is accepted.
type OrderPlaced struct {
	events.Base
	Total int64
}

var (
	ErrInvalidTotal = errors.New("total must be positive")
	ErrNotFound     = errors.New("order not found")
)

// Order is an aggregate: it guards its invariants and records what happened.
type Order struct {
	ddd.AggregateRoot
	total int64
}

// PlaceOrder is the only way to create a valid Order.
func PlaceOrder(id uuid.UUID, total int64) (*Order, error) {
	if total <= 0 {
		return nil, ErrInvalidTotal
	}
	o := &Order{AggregateRoot: ddd.NewAggregateRoot(id), total: total}
	o.RecordEvent(OrderPlaced{Base: events.NewBase(OrderPlacedName, id), Total: total})
	return o, nil
}

// RestoreOrder rebuilds an Order from stored state. It records no events:
// loading is not a state change.
func RestoreOrder(id uuid.UUID, total int64) *Order {
	return &Order{AggregateRoot: ddd.NewAggregateRoot(id), total: total}
}

func (o *Order) Total() int64 { return o.total }

// OrderRepository is a port: the domain defines it, infra implements it.
type OrderRepository interface {
	Save(ctx context.Context, o *Order) error
}
```

## 2. Command and handler

One command, one handler, one aggregate. The handler orchestrates; the aggregate decides.

`internal/ordering/app/place_order.go`

```go
package app

import (
	"context"
	"fmt"
	"uuid"

	"github.com/acme/shop/internal/ordering/domain"
	"github.com/acme/shop/internal/platform/events"
)

// PlaceOrder is a command: imperative name, plain data, no behavior.
type PlaceOrder struct {
	OrderID uuid.UUID
	Total   int64
}

func (PlaceOrder) Name() string { return "ordering.place_order" }

// PlaceOrderHandler loads/creates one aggregate, saves it and returns its events.
type PlaceOrderHandler struct {
	Orders domain.OrderRepository
}

func (h PlaceOrderHandler) Handle(ctx context.Context, cmd PlaceOrder) ([]events.Event, error) {
	o, err := domain.PlaceOrder(cmd.OrderID, cmd.Total)
	if err != nil {
		return nil, err
	}
	if err := h.Orders.Save(ctx, o); err != nil {
		return nil, fmt.Errorf("save order %s: %w", o.ID(), err)
	}
	return o.PullEvents(), nil
}
```

## 3. Query, read model and handler

Queries return views shaped for the caller and may bypass the aggregate.

`internal/ordering/app/get_order.go`

```go
package app

import (
	"context"
	"uuid"
)

// GetOrder is a query. Its result is a read model (DTO), never the aggregate.
type GetOrder struct{ OrderID uuid.UUID }

func (GetOrder) Name() string { return "ordering.get_order" }

type OrderView struct {
	ID    uuid.UUID `json:"id"`
	Total int64     `json:"total"`
}

// OrderReader is the read-side port. It may query the DB directly and
// skip the aggregate: queries don't need invariants.
type OrderReader interface {
	OrderByID(ctx context.Context, id uuid.UUID) (OrderView, error)
}

type GetOrderHandler struct{ Orders OrderReader }

func (h GetOrderHandler) Handle(ctx context.Context, q GetOrder) (OrderView, error) {
	return h.Orders.OrderByID(ctx, q.OrderID)
}
```

## 4. Policy: event → command

Lives in the reacting context. Cross-aggregate effects go through events, so they are eventually consistent. Policies may receive the same event twice: the command they send must be idempotent.

`internal/shipping/app/policy.go`

```go
package app

import (
	"context"
	"uuid"

	"github.com/acme/shop/internal/ordering/domain"
	"github.com/acme/shop/internal/platform/bus"
	"github.com/acme/shop/internal/platform/cqrs"
	"github.com/acme/shop/internal/platform/events"
)

type ScheduleShipment struct{ OrderID uuid.UUID }

func (ScheduleShipment) Name() string { return "shipping.schedule_shipment" }

// ShipWhenOrderPlaced is a policy: "when OrderPlaced, then ScheduleShipment".
// It translates the event into a command and sends it; it holds no logic.
type ShipWhenOrderPlaced struct {
	Commands bus.Dispatcher
}

func (p ShipWhenOrderPlaced) Handle(ctx context.Context, e domain.OrderPlaced) error {
	_, err := cqrs.Send(ctx, p.Commands, ScheduleShipment{OrderID: e.AggregateID()})
	return err
}

var _ events.Handler[domain.OrderPlaced] = ShipWhenOrderPlaced{}
```

## 5a. Unit of work (database/sql)

Puts the transaction in the context; repositories pick it up via `conn`. Adapt to pgx by swapping the types.

`internal/ordering/infra/postgres/uow.go`

```go
package postgres

import (
	"context"
	"database/sql"
	"errors"
)

type txKey struct{}

// UnitOfWork implements cqrs.UnitOfWork with a database/sql transaction.
type UnitOfWork struct{ DB *sql.DB }

func (u UnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	tx, err := u.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	if err = fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	return tx.Commit()
}

// querier is satisfied by both *sql.DB and *sql.Tx.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// conn returns the transaction in ctx, or db when there is none.
func conn(ctx context.Context, db *sql.DB) querier {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return db
}
```

## 5b. Repository

Implements the write port (domain) and the read port (app). Translates storage errors into domain errors.

`internal/ordering/infra/postgres/orders.go`

```go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"uuid"

	"github.com/acme/shop/internal/ordering/app"
	"github.com/acme/shop/internal/ordering/domain"
)

// Orders implements domain.OrderRepository and app.OrderReader.
type Orders struct{ DB *sql.DB }

func (r Orders) Save(ctx context.Context, o *domain.Order) error {
	_, err := conn(ctx, r.DB).ExecContext(ctx,
		`INSERT INTO orders (id, total) VALUES ($1, $2)`, o.ID().String(), o.Total())
	return err
}

func (r Orders) OrderByID(ctx context.Context, id uuid.UUID) (app.OrderView, error) {
	v := app.OrderView{ID: id}
	err := conn(ctx, r.DB).QueryRowContext(ctx,
		`SELECT total FROM orders WHERE id = $1`, id.String()).Scan(&v.Total)
	if errors.Is(err, sql.ErrNoRows) {
		return v, domain.ErrNotFound // adapters must not know about sql
	}
	return v, err
}
```

## 5c. In-memory repository

Stores a record, not the aggregate, and rebuilds it with a domain constructor that records no events.

`internal/ordering/infra/memory/orders.go`

```go
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
```

## 6. HTTP driving adapter

Decode → `cqrs.Send`/`cqrs.Ask` → map domain errors to status codes → encode.

`internal/ordering/infra/httpapi/handlers.go`

```go
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
```

## 7. Composition root

The only place that knows every layer. Register the SQL driver (e.g. `_ "github.com/jackc/pgx/v5/stdlib"`) in real code.

`cmd/shop/main.go`

```go
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/acme/shop/internal/ordering/app"
	"github.com/acme/shop/internal/ordering/domain"
	"github.com/acme/shop/internal/ordering/infra/httpapi"
	"github.com/acme/shop/internal/ordering/infra/postgres"
	"github.com/acme/shop/internal/platform/bus"
	"github.com/acme/shop/internal/platform/cqrs"
	"github.com/acme/shop/internal/platform/events"
	shipping "github.com/acme/shop/internal/shipping/app"
)

func main() {
	if err := run(); err != nil {
		slog.Error("shop stopped", "error", err)
		os.Exit(1)
	}
}

// run is the composition root: it is the only place that knows every layer.
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer db.Close()

	var (
		commands = bus.New()
		queries  = bus.New()
		evts     = bus.New()
		orders   = postgres.Orders{DB: db}
		uow      = postgres.UnitOfWork{DB: db}
	)

	// Normal rejections log at Info; anything else at Error.
	expected := []error{domain.ErrInvalidTotal, domain.ErrNotFound}

	// Every command: log → publish events after commit → transaction → handler.
	commandMws := []cqrs.CommandMiddleware{
		cqrs.LogCommandErrors(log, expected...),
		cqrs.PublishEvents(evts),
		cqrs.WithUnitOfWork(uow),
	}

	err = errors.Join(
		cqrs.RegisterCommand(commands, cqrs.WrapCommand(app.PlaceOrderHandler{Orders: orders}, commandMws...)),
		cqrs.RegisterQuery(queries, cqrs.WrapQuery(app.GetOrderHandler{Orders: orders}, cqrs.LogQueryErrors(log, expected...))),
		events.Register(evts, domain.OrderPlacedName, events.Handler[domain.OrderPlaced](shipping.ShipWhenOrderPlaced{Commands: commands})),
	)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	httpapi.Routes(mux, commands, queries)
	srv := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
```

## 8. Testing a command handler

Table-driven, hand-written fakes for ports; assert returned events, not internals.

`internal/ordering/app/place_order_test.go`

```go
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
```

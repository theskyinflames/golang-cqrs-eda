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

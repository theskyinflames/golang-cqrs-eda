package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/acme/shop/internal/platform/bus"
	"github.com/acme/shop/internal/platform/cqrs"
)

func main() {
	if err := run(); err != nil {
		slog.Error("shop stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	var (
		commands = bus.New()
		queries  = bus.New()
		evts     = bus.New()
	)
	commandMws := []cqrs.CommandMiddleware{
		cqrs.LogCommandErrors(log),
		cqrs.PublishEvents(evts),
	}
	_, _, _ = commands, queries, commandMws

	// TODO: register use cases.

	mux := http.NewServeMux()
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

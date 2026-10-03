// Package bus routes dispatchable messages (commands, queries, events) to
// their handlers by name. It is the seam between the infrastructure layer
// (HTTP, gRPC, consumers) and the application layer (handlers).
package bus

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Dispatchable is any message that can travel through a bus.
type Dispatchable interface {
	Name() string
}

// Handler handles a dispatchable message.
type Handler func(ctx context.Context, d Dispatchable) (any, error)

// Registry registers handlers by message name.
type Registry interface {
	Register(name string, h Handler) error
}

// Dispatcher dispatches a message to its handler and waits for the result.
type Dispatcher interface {
	Dispatch(ctx context.Context, d Dispatchable) (any, error)
}

var (
	// ErrNotDispatchable is returned when no handler is registered for a message.
	ErrNotDispatchable = errors.New("not dispatchable")
	// ErrAlreadyRegistered is returned when a name already has a handler.
	ErrAlreadyRegistered = errors.New("handler already registered")
)

type registry struct {
	mu sync.RWMutex
	h  map[string]Handler
}

func (r *registry) Register(name string, h Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.h == nil {
		r.h = make(map[string]Handler)
	}
	if _, ok := r.h[name]; ok {
		return fmt.Errorf("%w: %s", ErrAlreadyRegistered, name)
	}
	r.h[name] = h
	return nil
}

func (r *registry) handler(name string) (Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.h[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotDispatchable, name)
	}
	return h, nil
}

// Bus dispatches messages sequentially, in the caller's goroutine.
// The zero value is ready to use. Prefer it unless profiling proves otherwise.
type Bus struct {
	registry
}

// New returns a sequential bus.
func New() *Bus { return &Bus{} }

// Dispatch runs the handler registered for d.Name().
func (b *Bus) Dispatch(ctx context.Context, d Dispatchable) (any, error) {
	h, err := b.handler(d.Name())
	if err != nil {
		return nil, err
	}
	return h(ctx, d)
}

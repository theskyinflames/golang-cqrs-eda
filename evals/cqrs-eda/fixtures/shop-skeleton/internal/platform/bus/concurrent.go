package bus

import (
	"context"
	"sync"
	"time"
)

// Response is the outcome of an asynchronous dispatch.
type Response struct {
	Value any
	Err   error
}

// ConcurrentBus runs each handler in its own goroutine, bounded by a
// concurrency limit and a per-dispatch timeout.
type ConcurrentBus struct {
	registry
	timeout time.Duration
	sem     chan struct{}
	wg      sync.WaitGroup
}

// NewConcurrent returns a bus that runs at most limit handlers at once,
// each cancelled after timeout.
func NewConcurrent(timeout time.Duration, limit int) *ConcurrentBus {
	return &ConcurrentBus{
		timeout: timeout,
		sem:     make(chan struct{}, max(limit, 1)),
	}
}

// DispatchAsync starts the handler and returns a channel that receives
// exactly one Response. It blocks while the concurrency limit is reached
// (back-pressure) and gives up if ctx is done first.
func (b *ConcurrentBus) DispatchAsync(ctx context.Context, d Dispatchable) <-chan Response {
	out := make(chan Response, 1)

	h, err := b.handler(d.Name())
	if err != nil {
		out <- Response{Err: err}
		return out
	}

	select {
	case b.sem <- struct{}{}:
	case <-ctx.Done():
		out <- Response{Err: ctx.Err()}
		return out
	}

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer func() { <-b.sem }()
		ctx, cancel := context.WithTimeout(ctx, b.timeout)
		defer cancel()
		v, err := h(ctx, d)
		out <- Response{Value: v, Err: err}
	}()
	return out
}

// Dispatch implements Dispatcher by waiting for DispatchAsync.
func (b *ConcurrentBus) Dispatch(ctx context.Context, d Dispatchable) (any, error) {
	rs := <-b.DispatchAsync(ctx, d)
	return rs.Value, rs.Err
}

// InFlight returns the number of handlers currently running.
func (b *ConcurrentBus) InFlight() int { return len(b.sem) }

// Wait blocks until every started handler has returned. Call it on shutdown.
func (b *ConcurrentBus) Wait() { b.wg.Wait() }

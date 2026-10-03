package bus_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/acme/shop/internal/platform/bus"
)

type msg string

func (m msg) Name() string { return string(m) }

func echo(_ context.Context, d bus.Dispatchable) (any, error) { return d.Name(), nil }

func TestBus(t *testing.T) {
	var b bus.Bus // zero value is usable
	if err := b.Register("a", echo); err != nil {
		t.Fatal(err)
	}
	if err := b.Register("a", echo); !errors.Is(err, bus.ErrAlreadyRegistered) {
		t.Fatalf("want ErrAlreadyRegistered, got %v", err)
	}
	if v, err := b.Dispatch(t.Context(), msg("a")); err != nil || v != "a" {
		t.Fatalf("got %v, %v", v, err)
	}
	if _, err := b.Dispatch(t.Context(), msg("b")); !errors.Is(err, bus.ErrNotDispatchable) {
		t.Fatalf("want ErrNotDispatchable, got %v", err)
	}
}

func TestConcurrentBus(t *testing.T) {
	t.Run("respects the concurrency limit", func(t *testing.T) {
		const limit = 3
		b := bus.NewConcurrent(time.Second, limit)
		var running, peak atomic.Int32
		release := make(chan struct{})
		_ = b.Register("a", func(context.Context, bus.Dispatchable) (any, error) {
			n := running.Add(1)
			for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
			}
			<-release
			running.Add(-1)
			return nil, nil
		})

		var rs []<-chan bus.Response
		for range limit {
			rs = append(rs, b.DispatchAsync(t.Context(), msg("a")))
		}
		// The next dispatch must block until a slot frees up.
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		if r := <-b.DispatchAsync(ctx, msg("a")); !errors.Is(r.Err, context.DeadlineExceeded) {
			t.Fatalf("want DeadlineExceeded, got %v", r.Err)
		}
		close(release)
		for _, r := range rs {
			if res := <-r; res.Err != nil {
				t.Fatal(res.Err)
			}
		}
		b.Wait()
		if peak.Load() > limit || b.InFlight() != 0 {
			t.Fatalf("peak %d, in flight %d", peak.Load(), b.InFlight())
		}
	})

	t.Run("cancels slow handlers", func(t *testing.T) {
		b := bus.NewConcurrent(10*time.Millisecond, 1)
		_ = b.Register("slow", func(ctx context.Context, _ bus.Dispatchable) (any, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		})
		if _, err := b.Dispatch(t.Context(), msg("slow")); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want DeadlineExceeded, got %v", err)
		}
	})

	t.Run("unknown message", func(t *testing.T) {
		b := bus.NewConcurrent(time.Second, 1)
		if _, err := b.Dispatch(t.Context(), msg("x")); !errors.Is(err, bus.ErrNotDispatchable) {
			t.Fatalf("want ErrNotDispatchable, got %v", err)
		}
	})
}

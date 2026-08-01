package provider

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

type counterProvider struct {
	calls int32
}

func (c *counterProvider) GetPower() (float64, float64, float64, float64, error) {
	val := float64(atomic.AddInt32(&c.calls, 1))
	return val, 0, 0, val, nil
}

func TestThrottledProvider(t *testing.T) {
	t.Run("Throttling enabled - blocking behavior", func(t *testing.T) {
		base := &counterProvider{}
		interval := 100 * time.Millisecond
		ctx := context.Background()

		tp := NewThrottledProvider(ctx, base, interval)

		// First call: Should be immediate as lastFetch is uninitialized
		start := time.Now()
		p1, _, _, _, err := tp.GetPower()
		elapsed1 := time.Since(start)

		if err != nil {
			t.Fatal(err)
		}
		if p1 != 1 {
			t.Errorf("expected power 1, got %f", p1)
		}
		if elapsed1 > 20*time.Millisecond {
			t.Errorf("expected first call to be fast, but took %v", elapsed1)
		}

		// Second call: Should block for roughly the interval duration
		start = time.Now()
		p2, _, _, _, err := tp.GetPower()
		elapsed2 := time.Since(start)

		if err != nil {
			t.Fatal(err)
		}
		// Value should be 2 because it fetches fresh data (no Read-and-Clear)
		if p2 != 2 {
			t.Errorf("expected power 2 (fresh fetch), got %f", p2)
		}
		if elapsed2 < interval {
			t.Errorf("expected second call to block for %v, but took %v", interval, elapsed2)
		}
	})

	t.Run("Throttling disabled - immediate pass-through", func(t *testing.T) {
		base := &counterProvider{}
		tp := NewThrottledProvider(context.Background(), base, 0)

		start := time.Now()
		tp.GetPower()
		tp.GetPower()
		elapsed := time.Since(start)

		if elapsed > 20*time.Millisecond {
			t.Errorf("expected calls to be immediate with 0 interval, but took %v", elapsed)
		}
		if atomic.LoadInt32(&base.calls) != 2 {
			t.Errorf("expected 2 calls to base provider, got %d", atomic.LoadInt32(&base.calls))
		}
	})

	t.Run("Context cancellation aborts pending wait", func(t *testing.T) {
		base := &counterProvider{}
		interval := 5 * time.Second
		ctx, cancel := context.WithCancel(t.Context())

		tp := NewThrottledProvider(ctx, base, interval)

		// First call is immediate and starts the throttle window
		if _, _, _, _, err := tp.GetPower(); err != nil {
			t.Fatal(err)
		}

		// Cancel while the second call would have to wait for the interval
		cancel()

		start := time.Now()
		_, _, _, _, err := tp.GetPower()
		elapsed := time.Since(start)

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
		if elapsed > time.Second {
			t.Errorf("expected canceled call to return quickly, but took %v", elapsed)
		}
		if atomic.LoadInt32(&base.calls) != 1 {
			t.Errorf("expected base provider not to be called after cancellation, got %d calls", atomic.LoadInt32(&base.calls))
		}
	})

	t.Run("Error propagation", func(t *testing.T) {
		mockErr := fmt.Errorf("provider failure")
		base := &customProvider{
			getPower: func() (float64, float64, float64, float64, error) {
				return 0, 0, 0, 0, mockErr
			},
		}

		tp := NewThrottledProvider(context.Background(), base, 10*time.Millisecond)

		_, _, _, _, err := tp.GetPower()
		if !errors.Is(err, mockErr) {
			t.Errorf("expected error %v, got %v", mockErr, err)
		}
	})
}

type customProvider struct {
	getPower func() (float64, float64, float64, float64, error)
}

func (c *customProvider) GetPower() (float64, float64, float64, float64, error) {
	return c.getPower()
}

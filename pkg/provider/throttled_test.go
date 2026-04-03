package provider

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

type counterProvider struct {
	calls int32
}

func (c *counterProvider) GetPower() (float64, float64, float64, float64, error) {
	atomic.AddInt32(&c.calls, 1)
	val := float64(atomic.LoadInt32(&c.calls))
	return val, 0, 0, val, nil
}

func TestThrottledProvider(t *testing.T) {
	t.Run("Throttling enabled - non-blocking and background updates", func(t *testing.T) {
		base := &counterProvider{}
		interval := 100 * time.Millisecond
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		tp := NewThrottledProvider(ctx, base, interval)

		// GetPower should return the initial fetch value immediately
		start := time.Now()
		p1, _, _, _, err := tp.GetPower()
		elapsed := time.Since(start)

		if err != nil {
			t.Fatal(err)
		}
		if p1 != 1 {
			t.Errorf("expected power 1, got %f", p1)
		}
		if elapsed > 10*time.Millisecond {
			t.Errorf("expected call to be fast, but took %v", elapsed)
		}

		// Second call should return 0 because of Read-and-Clear
		p2, _, _, _, _ := tp.GetPower()
		if p2 != 0 {
			t.Errorf("expected 0 on second call (Read-and-Clear), got %f", p2)
		}

		// Wait for background worker to tick (which performs 2nd fetch in background)
		time.Sleep(interval + 50*time.Millisecond)

		p3, _, _, _, _ := tp.GetPower()
		if p3 < 2 {
			t.Errorf("expected power to be updated (>=2), got %f", p3)
		}

		// Third call should be 0 again
		p4, _, _, _, _ := tp.GetPower()
		if p4 != 0 {
			t.Errorf("expected 0 on third call (Read-and-Clear), got %f", p4)
		}
	})

	t.Run("Default throttle interval - disabled if 0", func(t *testing.T) {
		base := &counterProvider{}
		tp := NewThrottledProvider(t.Context(), base, 0)

		if tp.interval != 0 {
			t.Errorf("expected 0 throttle interval, got %v", tp.interval)
		}

		tp.GetPower()
		tp.GetPower()
		if atomic.LoadInt32(&base.calls) != 2 {
			t.Errorf("expected 2 calls (no throttling), got %d", atomic.LoadInt32(&base.calls))
		}
	})

	t.Run("Zeroes cache if fetch fails", func(t *testing.T) {
		type failingProvider struct {
			calls int32
		}
		base := &failingProvider{}
		interval := 50 * time.Millisecond

		mock := func() (float64, float64, float64, float64, error) {
			c := atomic.AddInt32(&base.calls, 1)
			if c == 1 {
				return 100, 0, 0, 100, nil
			}
			return 0, 0, 0, 0, fmt.Errorf("provider error")
		}

		wrapped := &customProvider{getPower: mock}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		tp := NewThrottledProvider(ctx, wrapped, interval)

		// Initial fetch (during NewThrottledProvider) succeeded
		p1, _, _, _, _ := tp.GetPower()
		if p1 != 100 {
			t.Errorf("expected 100, got %f", p1)
		}

		// Wait for background worker to tick and fail
		time.Sleep(interval + 20*time.Millisecond)

		p2, _, _, _, _ := tp.GetPower()
		if p2 != 0 {
			t.Errorf("expected 0 after background failure, got %f", p2)
		}
	})
}

type customProvider struct {
	getPower func() (float64, float64, float64, float64, error)
}

func (c *customProvider) GetPower() (float64, float64, float64, float64, error) {
	return c.getPower()
}

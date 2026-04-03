package provider

import (
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
	t.Run("Throttling enabled - blocks until interval passed", func(t *testing.T) {
		base := &counterProvider{}
		interval := 100 * time.Millisecond
		tp := NewThrottledProvider(t.Context(), base, interval, 0)

		// Initial fetch happened in NewThrottledProvider
		if atomic.LoadInt32(&base.calls) != 1 {
			t.Errorf("expected 1 call (initial), got %d", atomic.LoadInt32(&base.calls))
		}

		start := time.Now()
		// This call should block to satisfy the throttle interval
		p2, _, _, _, err := tp.GetPower()
		elapsed := time.Since(start)

		if err != nil {
			t.Fatal(err)
		}
		if atomic.LoadInt32(&base.calls) != 2 {
			t.Errorf("expected 2 calls, got %d", atomic.LoadInt32(&base.calls))
		}
		if p2 != 2 {
			t.Errorf("expected power 2, got %f", p2)
		}
		// It should have blocked for approximately 'interval'
		if elapsed < 80*time.Millisecond {
			t.Errorf("expected call to block, but took only %v", elapsed)
		}
	})

	t.Run("Throttling enabled - no wait if enough time passed", func(t *testing.T) {
		base := &counterProvider{}
		interval := 50 * time.Millisecond
		tp := NewThrottledProvider(t.Context(), base, interval, 0)

		// Wait for interval to pass since initial fetch
		time.Sleep(interval + 10*time.Millisecond)

		start := time.Now()
		p, _, _, _, err := tp.GetPower()
		elapsed := time.Since(start)

		if err != nil {
			t.Fatal(err)
		}
		if p != 2 {
			t.Errorf("expected power 2, got %f", p)
		}
		if elapsed > 20*time.Millisecond {
			t.Errorf("expected call to be fast, but took %v", elapsed)
		}
	})

	t.Run("Default throttle interval - disabled if 0", func(t *testing.T) {
		base := &counterProvider{}
		tp := NewThrottledProvider(t.Context(), base, 0, 0) // Should not throttle if 0

		if tp.throttleInterval != 0 {
			t.Errorf("expected 0 throttle interval, got %v", tp.throttleInterval)
		}

		tp.GetPower()
		tp.GetPower()
		if atomic.LoadInt32(&base.calls) != 2 {
			t.Errorf("expected 2 calls (no throttling), got %d", atomic.LoadInt32(&base.calls))
		}
	})

	t.Run("Stale threshold - returns 0 if cache is too old and fetch fails", func(t *testing.T) {
		type failingProvider struct {
			calls int32
		}
		base := &failingProvider{}
		interval := 10 * time.Millisecond
		staleThreshold := 50 * time.Millisecond

		mock := func() (float64, float64, float64, float64, error) {
			c := atomic.AddInt32(&base.calls, 1)
			if c == 1 {
				// Initial fetch in NewThrottledProvider succeeds
				return 100, 0, 0, 100, nil
			}
			// Subsequent fetches fail
			return 0, 0, 0, 0, fmt.Errorf("provider error")
		}

		wrapped := &customProvider{getPower: mock}
		tp := NewThrottledProvider(t.Context(), wrapped, interval, staleThreshold)

		// Wait for stale threshold to pass
		time.Sleep(staleThreshold + 20*time.Millisecond)

		// This call will attempt to fetch, fail, and then see that the cache is stale
		p, _, _, _, err := tp.GetPower()
		if err != nil {
			t.Fatal(err)
		}
		if p != 0 {
			t.Errorf("expected 0 after stale threshold, got %f", p)
		}
	})
}

type customProvider struct {
	getPower func() (float64, float64, float64, float64, error)
}

func (c *customProvider) GetPower() (float64, float64, float64, float64, error) {
	return c.getPower()
}

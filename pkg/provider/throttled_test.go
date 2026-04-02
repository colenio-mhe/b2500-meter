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
	t.Run("Throttling enabled - returns cache instead of waiting", func(t *testing.T) {
		base := &counterProvider{}
		interval := 100 * time.Millisecond
		tp := NewThrottledProvider(t.Context(), base, interval, 0)

		// Wait for initial background fetch
		time.Sleep(20 * time.Millisecond)

		start := time.Now()
		// First call should return cached value from initial fetch
		p1, _, _, _, err := tp.GetPower()
		if err != nil {
			t.Fatal(err)
		}
		if atomic.LoadInt32(&base.calls) != 1 {
			t.Errorf("expected 1 call, got %d", atomic.LoadInt32(&base.calls))
		}
		if p1 != 1 {
			t.Errorf("expected power 1, got %f", p1)
		}

		// Second call should NOT wait and return cached value (1)
		p2, _, _, _, err := tp.GetPower()
		if err != nil {
			t.Fatal(err)
		}
		elapsed := time.Since(start)

		if atomic.LoadInt32(&base.calls) != 1 {
			t.Errorf("expected still 1 call (cached), got %d", atomic.LoadInt32(&base.calls))
		}
		if p2 != 1 {
			t.Errorf("expected power 1 (cached), got %f", p2)
		}
		if elapsed > 10*time.Millisecond {
			t.Errorf("expected call to be fast (cached), but took %v", elapsed)
		}
	})

	t.Run("Throttling enabled - fetch fresh after interval", func(t *testing.T) {
		base := &counterProvider{}
		interval := 50 * time.Millisecond
		tp := NewThrottledProvider(t.Context(), base, interval, 0)

		// Wait for initial fetch
		time.Sleep(10 * time.Millisecond)
		tp.GetPower() // should return 1

		// Wait for next background fetch
		time.Sleep(interval + 20*time.Millisecond)

		if atomic.LoadInt32(&base.calls) != 2 {
			t.Errorf("expected 2 calls after interval, got %d", atomic.LoadInt32(&base.calls))
		}

		p2, _, _, _, err := tp.GetPower() // should return cached 2
		if err != nil {
			t.Fatal(err)
		}
		if p2 != 2 {
			t.Errorf("expected power 2, got %f", p2)
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

	t.Run("Stale threshold - returns 0 if cache is too old", func(t *testing.T) {
		type failingProvider struct {
			calls int32
		}
		base := &failingProvider{}
		// First call succeeds, subsequent calls fail
		interval := 50 * time.Millisecond
		staleThreshold := 100 * time.Millisecond

		mock := func() (float64, float64, float64, float64, error) {
			c := atomic.AddInt32(&base.calls, 1)
			if c == 1 {
				return 100, 0, 0, 100, nil
			}
			return 0, 0, 0, 0, fmt.Errorf("provider error")
		}

		// Use a custom provider that uses the mock function
		wrapped := &customProvider{getPower: mock}
		tp := NewThrottledProvider(t.Context(), wrapped, interval, staleThreshold)

		// Wait for initial fetch
		time.Sleep(20 * time.Millisecond)
		p, _, _, _, err := tp.GetPower()
		if err != nil {
			t.Fatal(err)
		}
		if p != 100 {
			t.Errorf("expected 100, got %f", p)
		}

		// Wait for stale threshold to pass.
		// After initial fetch, background loop will try again after 50ms and fail.
		// lastFetchTime will stay at the time of the first fetch.
		time.Sleep(staleThreshold + 10*time.Millisecond)

		p, _, _, _, err = tp.GetPower()
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

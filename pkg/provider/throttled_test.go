package provider

import (
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
		tp := NewThrottledProvider(base, interval)

		start := time.Now()
		// First call should be immediate and fetch
		p1, _, _, _, err := tp.GetPower()
		if err != nil {
			t.Fatal(err)
		}
		if base.calls != 1 {
			t.Errorf("expected 1 call, got %d", base.calls)
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

		if base.calls != 1 {
			t.Errorf("expected still 1 call (cached), got %d", base.calls)
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
		tp := NewThrottledProvider(base, interval)

		tp.GetPower() // calls = 1
		time.Sleep(interval + 10*time.Millisecond)

		start := time.Now()
		p2, _, _, _, err := tp.GetPower() // should call base
		elapsed := time.Since(start)

		if err != nil {
			t.Fatal(err)
		}
		if base.calls != 2 {
			t.Errorf("expected 2 calls, got %d", base.calls)
		}
		if p2 != 2 {
			t.Errorf("expected power 2, got %f", p2)
		}
		// Should not have waited much since interval already passed
		if elapsed > 20*time.Millisecond {
			t.Errorf("expected second call to be fast, but took %v", elapsed)
		}
	})

	t.Run("Default throttle interval - disabled if 0", func(t *testing.T) {
		base := &counterProvider{}
		tp := NewThrottledProvider(base, 0) // Should not throttle if 0

		if tp.throttleInterval != 0 {
			t.Errorf("expected 0 throttle interval, got %v", tp.throttleInterval)
		}

		tp.GetPower()
		tp.GetPower()
		if base.calls != 2 {
			t.Errorf("expected 2 calls (no throttling), got %d", base.calls)
		}
	})
}

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
	t.Run("Throttling disabled", func(t *testing.T) {
		base := &counterProvider{}
		tp := NewThrottledProvider(base, 0)

		for i := 1; i <= 5; i++ {
			tp.GetPower()
		}

		if base.calls != 5 {
			t.Errorf("expected 5 calls, got %d", base.calls)
		}
	})

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

		// Second call should WAIT and then fetch again
		p2, _, _, _, err := tp.GetPower()
		if err != nil {
			t.Fatal(err)
		}
		elapsed := time.Since(start)

		if base.calls != 2 {
			t.Errorf("expected 2 calls, got %d", base.calls)
		}
		if p2 != 2 {
			t.Errorf("expected power 2, got %f", p2)
		}
		if elapsed < interval {
			t.Errorf("expected call to wait for interval, but took only %v", elapsed)
		}
	})

	t.Run("Throttling enabled - no wait if enough time passed", func(t *testing.T) {
		base := &counterProvider{}
		interval := 50 * time.Millisecond
		tp := NewThrottledProvider(base, interval)

		tp.GetPower()
		time.Sleep(interval + 10*time.Millisecond)

		start := time.Now()
		tp.GetPower()
		elapsed := time.Since(start)

		if base.calls != 2 {
			t.Errorf("expected 2 calls, got %d", base.calls)
		}
		// Should not have waited much
		if elapsed > 20*time.Millisecond {
			t.Errorf("expected second call to be fast, but took %v", elapsed)
		}
	})
}

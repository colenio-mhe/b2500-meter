package provider

import (
	"log/slog"
	"sync"
	"time"
)

// ThrottledProvider is a decorator for a PowerProvider that limits the frequency of data fetches.
// If GetPower is called more often than the throttle interval, it returns the last known value.
type ThrottledProvider struct {
	wrapped          PowerProvider
	throttleInterval time.Duration
	lastUpdateTime   time.Time
	lastA            float64
	lastB            float64
	lastC            float64
	lastTotal        float64
	mu               sync.Mutex
}

// NewThrottledProvider wraps an existing PowerProvider with the specified throttle interval.
func NewThrottledProvider(wrapped PowerProvider, throttleInterval time.Duration) *ThrottledProvider {
	return &ThrottledProvider{
		wrapped:          wrapped,
		throttleInterval: throttleInterval,
	}
}

// GetPower returns the power readings, enforcing a minimum delay between fetches to prevent hammering the source.
func (t *ThrottledProvider) GetPower() (phaseA, phaseB, phaseC, total float64, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	if t.throttleInterval <= 0 {
		a, b, c, totalVal, pErr := t.wrapped.GetPower()
		if pErr != nil {
			if t.lastUpdateTime.IsZero() {
				slog.Error("Throttling: Error getting initial power values", "error", pErr)
				return 0, 0, 0, 0, pErr
			}
			slog.Warn("Throttling: Error getting fresh values, using cache", "error", pErr)
			return t.lastA, t.lastB, t.lastC, t.lastTotal, nil
		}
		t.lastA, t.lastB, t.lastC, t.lastTotal = a, b, c, totalVal
		t.lastUpdateTime = now
		return a, b, c, totalVal, nil
	}

	timeSinceLastUpdate := now.Sub(t.lastUpdateTime)

	if !t.lastUpdateTime.IsZero() && timeSinceLastUpdate < t.throttleInterval {
		waitTime := t.throttleInterval - timeSinceLastUpdate
		slog.Debug("Throttling: Waiting before fetching fresh values", "wait_time", waitTime)
		time.Sleep(waitTime)
		now = time.Now()
		timeSinceLastUpdate = now.Sub(t.lastUpdateTime)
	}

	a, b, c, totalVal, pErr := t.wrapped.GetPower()
	if pErr != nil {
		if t.lastUpdateTime.IsZero() {
			slog.Error("Throttling: Error getting initial power values", "error", pErr)
			return 0, 0, 0, 0, pErr
		}
		slog.Warn("Throttling: Error getting fresh values, using cache", "error", pErr)
		return t.lastA, t.lastB, t.lastC, t.lastTotal, nil
	}

	t.lastA, t.lastB, t.lastC, t.lastTotal = a, b, c, totalVal
	t.lastUpdateTime = now

	slog.Debug("Throttling: Fetched fresh values", "interval", timeSinceLastUpdate, "total", totalVal)

	return a, b, c, totalVal, nil
}

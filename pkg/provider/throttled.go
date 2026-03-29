package provider

import (
	"log/slog"
	"sync"
	"time"
)

// ThrottledProvider is a caching proxy around a PowerProvider that limits the frequency of data fetches.
// If GetPower is called more often than the throttle interval, it returns the last known value immediately.
type ThrottledProvider struct {
	wrapped          PowerProvider
	throttleInterval time.Duration
	lastFetchTime    time.Time
	lastA            float64
	lastB            float64
	lastC            float64
	lastTotal        float64
	mu               sync.RWMutex
}

// NewThrottledProvider wraps an existing PowerProvider with the specified throttle interval.
func NewThrottledProvider(wrapped PowerProvider, throttleInterval time.Duration) *ThrottledProvider {
	return &ThrottledProvider{
		wrapped:          wrapped,
		throttleInterval: throttleInterval,
	}
}

// GetPower returns the power readings. If called within the throttle interval, it returns the cached values.
// Otherwise, it fetches fresh data from the underlying provider.
func (t *ThrottledProvider) GetPower() (phaseA, phaseB, phaseC, total float64, err error) {
	// First, try to read from cache if the interval hasn't passed.
	t.mu.RLock()
	if t.throttleInterval > 0 && !t.lastFetchTime.IsZero() && time.Since(t.lastFetchTime) < t.throttleInterval {
		t.mu.RUnlock()
		return t.lastA, t.lastB, t.lastC, t.lastTotal, nil
	}
	t.mu.RUnlock()

	// If we need to fetch fresh data, we need a write lock.
	t.mu.Lock()
	defer t.mu.Unlock()

	// Re-check after acquiring write lock to avoid multiple simultaneous fetches.
	now := time.Now()
	if t.throttleInterval > 0 && !t.lastFetchTime.IsZero() && now.Sub(t.lastFetchTime) < t.throttleInterval {
		return t.lastA, t.lastB, t.lastC, t.lastTotal, nil
	}

	a, b, c, totalVal, pErr := t.wrapped.GetPower()
	if pErr != nil {
		if t.lastFetchTime.IsZero() {
			slog.Error("Throttling: Error getting initial power values", "error", pErr)
			return 0, 0, 0, 0, pErr
		}
		slog.Warn("Throttling: Error getting fresh values, using cache", "error", pErr, "last_fetch", t.lastFetchTime)
		return t.lastA, t.lastB, t.lastC, t.lastTotal, nil
	}

	t.lastA, t.lastB, t.lastC, t.lastTotal = a, b, c, totalVal
	t.lastFetchTime = now

	slog.Debug("Throttling: Fetched fresh values", "total", totalVal, "elapsed", time.Since(now))

	return a, b, c, totalVal, nil
}

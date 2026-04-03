package provider

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// ThrottledProvider is a caching proxy around a PowerProvider that limits the frequency of data fetches.
// It ensures that fresh data is fetched at most once per throttle interval.
// If multiple requests arrive too frequently, it waits for the remaining time before fetching fresh values.
type ThrottledProvider struct {
	wrapped          PowerProvider
	throttleInterval time.Duration
	staleThreshold   time.Duration
	lastFetchTime    time.Time
	lastA            float64
	lastB            float64
	lastC            float64
	lastTotal        float64
	mu               sync.Mutex
}

// NewThrottledProvider wraps an existing PowerProvider with the specified throttle interval and stale threshold.
// If throttleInterval > 0, it ensures that fresh data is fetched at most once per that interval.
// If staleThreshold > 0, GetPower will return 0 if no fresh data has been successfully fetched within that period.
func NewThrottledProvider(_ context.Context, wrapped PowerProvider, throttleInterval time.Duration, staleThreshold time.Duration) *ThrottledProvider {
	t := &ThrottledProvider{
		wrapped:          wrapped,
		throttleInterval: throttleInterval,
		staleThreshold:   staleThreshold,
	}

	// Initial fetch to populate cache
	if throttleInterval > 0 {
		t.mu.Lock()
		t.fetchLocked(time.Now())
		t.mu.Unlock()
	}

	return t
}

// GetPower returns the power readings. If throttling is enabled, it waits if the throttle interval hasn't passed,
// then fetches fresh data. This ensures the caller always receives data that is as fresh as possible within the throttling constraints.
func (t *ThrottledProvider) GetPower() (phaseA, phaseB, phaseC, total float64, err error) {
	if t.throttleInterval <= 0 {
		return t.wrapped.GetPower()
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if !t.lastFetchTime.IsZero() {
		timeSinceLast := now.Sub(t.lastFetchTime)
		if timeSinceLast < t.throttleInterval {
			wait := t.throttleInterval - timeSinceLast
			slog.Debug("Throttling: Waiting before fetching fresh values...", "wait", wait)
			time.Sleep(wait)
			now = time.Now()
		}
	}

	return t.fetchLocked(now)
}

// fetchLocked fetches fresh values from the wrapped provider and updates the cache.
// It assumes the mutex is already held.
func (t *ThrottledProvider) fetchLocked(startTime time.Time) (float64, float64, float64, float64, error) {
	a, b, c, totalVal, pErr := t.wrapped.GetPower()
	fetchDuration := time.Since(startTime)

	if pErr != nil {
		if t.lastFetchTime.IsZero() {
			slog.Error("Throttling: Error getting initial power values", "error", pErr, "fetch_duration", fetchDuration)
			return 0, 0, 0, 0, pErr
		}

		// Check for staleness even on error
		if t.staleThreshold > 0 && time.Since(t.lastFetchTime) > t.staleThreshold {
			slog.Warn("Throttling: Cache is stale, returning 0", "stale_threshold", t.staleThreshold, "last_fetch", t.lastFetchTime)
			return 0, 0, 0, 0, nil
		}

		slog.Warn("Throttling: Error getting fresh values, using cache", "error", pErr, "last_fetch", t.lastFetchTime, "fetch_duration", fetchDuration)
		return t.lastA, t.lastB, t.lastC, t.lastTotal, nil
	}

	t.lastA, t.lastB, t.lastC, t.lastTotal = a, b, c, totalVal
	t.lastFetchTime = startTime // Set to the start of the fetch (after potential wait)

	slog.Debug("Throttling: Fetched fresh values", "total", totalVal, "fetch_duration", fetchDuration)
	return t.lastA, t.lastB, t.lastC, t.lastTotal, nil
}

package provider

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ThrottledProvider is a caching proxy around a PowerProvider that limits the frequency of data fetches.
// It decouples the data fetch from the GetPower request by updating the cache in a background goroutine.
type ThrottledProvider struct {
	wrapped          PowerProvider
	throttleInterval time.Duration
	staleThreshold   time.Duration
	lastFetchTime    time.Time
	lastA            float64
	lastB            float64
	lastC            float64
	lastTotal        float64
	mu               sync.RWMutex
}

// NewThrottledProvider wraps an existing PowerProvider with the specified throttle interval and stale threshold.
// If throttleInterval > 0, it starts a background goroutine to update the cache.
// If staleThreshold > 0, GetPower will return 0 if the cache hasn't been updated within that period.
func NewThrottledProvider(ctx context.Context, wrapped PowerProvider, throttleInterval time.Duration, staleThreshold time.Duration) *ThrottledProvider {
	t := &ThrottledProvider{
		wrapped:          wrapped,
		throttleInterval: throttleInterval,
		staleThreshold:   staleThreshold,
	}

	// Initial fetch
	if throttleInterval > 0 {
		t.fetch()
		go t.run(ctx)
	}

	return t
}

func (t *ThrottledProvider) run(ctx context.Context) {
	ticker := time.NewTicker(t.throttleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.fetch()
		}
	}
}

func (t *ThrottledProvider) fetch() {
	a, b, c, totalVal, pErr := t.wrapped.GetPower()
	t.mu.Lock()
	defer t.mu.Unlock()

	if pErr != nil {
		if t.lastFetchTime.IsZero() {
			slog.Error("Throttling: Error getting initial power values", "error", pErr)
			return
		}
		slog.Warn("Throttling: Error getting fresh values, using cache", "error", pErr, "last_fetch", t.lastFetchTime)
		return
	}

	t.lastA, t.lastB, t.lastC, t.lastTotal = a, b, c, totalVal
	t.lastFetchTime = time.Now()

	slog.Debug("Throttling: Fetched fresh values", "total", totalVal)
}

// GetPower returns the power readings. If throttling is enabled, it returns the cached values immediately.
// If no values have been fetched yet, it returns an error.
func (t *ThrottledProvider) GetPower() (phaseA, phaseB, phaseC, total float64, err error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.throttleInterval <= 0 {
		return t.wrapped.GetPower()
	}

	if t.lastFetchTime.IsZero() {
		return 0, 0, 0, 0, fmt.Errorf("no power data available from throttled provider yet")
	}

	if t.staleThreshold > 0 && time.Since(t.lastFetchTime) > t.staleThreshold {
		slog.Warn("Throttling: Cache is stale, returning 0", "stale_threshold", t.staleThreshold, "last_fetch", t.lastFetchTime)
		return 0, 0, 0, 0, nil
	}

	return t.lastA, t.lastB, t.lastC, t.lastTotal, nil
}

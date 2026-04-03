package provider

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// ThrottledProvider is a proxy around a PowerProvider that limits the frequency of data fetches.
// It ensures that fresh data is fetched at most once per throttle interval.
// If multiple requests arrive too frequently, it waits for the remaining time before fetching fresh values.
type ThrottledProvider struct {
	wrapped   PowerProvider
	interval  time.Duration
	mu        sync.Mutex
	pA        float64
	pB        float64
	pC        float64
	pTotal    float64
	lastError error
}

// NewThrottledProvider wraps an existing PowerProvider with the specified throttle interval.
// If interval > 0, it starts a background goroutine to fetch data periodically.
func NewThrottledProvider(ctx context.Context, wrapped PowerProvider, interval time.Duration) *ThrottledProvider {
	t := &ThrottledProvider{
		wrapped:  wrapped,
		interval: interval,
	}

	if interval > 0 {
		// Initial fetch to have data ready
		t.fetchAndUpdate()
		go t.run(ctx)
	}

	return t
}

func (t *ThrottledProvider) run(ctx context.Context) {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.fetchAndUpdate()
		}
	}
}

func (t *ThrottledProvider) fetchAndUpdate() {
	a, b, c, total, err := t.wrapped.GetPower()

	t.mu.Lock()
	defer t.mu.Unlock()

	if err != nil {
		slog.Warn("Throttling: Error getting fresh values, zeroing cache", "error", err)
		t.pA, t.pB, t.pC, t.pTotal = 0, 0, 0, 0
		t.lastError = err
	} else {
		t.pA, t.pB, t.pC, t.pTotal = a, b, c, total
		t.lastError = nil
		slog.Debug("Throttling: Fetched fresh values", "total", total)
	}
}

// GetPower returns the cached power readings from RAM and IMMEDIATELY clears them.
// This prevents the integrating Marstek battery from double-counting values.
// If multiple requests arrive before the next background fetch, they receive 0.
// If throttling is disabled (interval <= 0), it fetches fresh data directly.
func (t *ThrottledProvider) GetPower() (phaseA, phaseB, phaseC, total float64, err error) {
	if t.interval <= 0 {
		return t.wrapped.GetPower()
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	a, b, c, tot := t.pA, t.pB, t.pC, t.pTotal
	currentErr := t.lastError

	t.pA, t.pB, t.pC, t.pTotal = 0, 0, 0, 0
	t.lastError = nil

	return a, b, c, tot, currentErr
}

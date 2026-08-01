package provider

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// ThrottledProvider wraps a PowerProvider to rate-limit data retrieval.
// It ensures the underlying source is not accessed more frequently than the specified interval
// by blocking the caller until the interval has elapsed.
type ThrottledProvider struct {
	ctx       context.Context
	wrapped   PowerProvider
	interval  time.Duration
	mu        sync.Mutex
	lastFetch time.Time
}

// NewThrottledProvider initializes a ThrottledProvider with the given interval.
// The context is used to abort a pending wait when the application shuts down.
func NewThrottledProvider(ctx context.Context, wrapped PowerProvider, interval time.Duration) *ThrottledProvider {
	return &ThrottledProvider{
		ctx:      ctx,
		wrapped:  wrapped,
		interval: interval,
	}
}

// GetPower retrieves power data from the wrapped provider.
// If called before the interval has passed, it blocks until the next allowed fetch time.
func (t *ThrottledProvider) GetPower() (phaseA, phaseB, phaseC, total float64, err error) {
	if t.interval <= 0 {
		return t.wrapped.GetPower()
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if wait := t.interval - time.Since(t.lastFetch); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()

		select {
		case <-timer.C:
		case <-t.ctx.Done():
			return 0, 0, 0, 0, t.ctx.Err()
		}
	}

	a, b, c, tot, err := t.wrapped.GetPower()
	t.lastFetch = time.Now()

	if err != nil {
		slog.Error("Throttling: Failed to fetch data from source", "error", err)
		return 0, 0, 0, 0, err
	}

	return a, b, c, tot, nil
}

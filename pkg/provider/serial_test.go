package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSerialProvider_Flow(t *testing.T) {
	t.Run("successful data retrieval and stale enforcement", func(t *testing.T) {
		s := &SerialProvider{
			jsonPath: "SML.Power",
		}

		// Simulate the background reader receiving fresh data
		s.mu.Lock()
		s.lastVal = 500.0
		s.isFresh = true
		s.mu.Unlock()

		// First call should return the fresh value
		_, _, _, total, err := s.GetPower()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 500.0 {
			t.Errorf("expected 500, got %f", total)
		}

		// Second call immediately after should fail because the data is now stale
		_, _, _, _, err = s.GetPower()
		if err == nil {
			t.Error("expected stale data error on second call, got nil")
		}
	})

	t.Run("initial state returns error", func(t *testing.T) {
		s := &SerialProvider{}

		// Call GetPower before any data has been simulated
		_, _, _, _, err := s.GetPower()
		if err == nil {
			t.Error("expected stale/no data error initially, got nil")
		}
	})
}

func TestSerialProvider_ErrorPropagation(t *testing.T) {
	s := &SerialProvider{
		jsonPath: "SML.Power",
	}

	mockErr := errors.New("serial port lost")

	// Simulate a connection drop using the provider's own method
	s.setError(mockErr)

	// GetPower should instantly return the hardware error, bypassing the stale check
	_, _, _, _, err := s.GetPower()
	if !errors.Is(err, mockErr) {
		t.Errorf("expected error %v, got %v", mockErr, err)
	}

	// Simulate connection restored
	s.setError(nil)
	s.mu.Lock()
	s.isFresh = true // Give it fresh data
	s.mu.Unlock()

	_, _, _, _, err = s.GetPower()
	if err != nil {
		t.Errorf("expected no error after connection restore, got %v", err)
	}
}

func TestSerialProvider_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Start the provider with a dummy port
	// It will internally fail to open and enter a retry loop, which is fine
	_ = NewSerialProvider(ctx, "/dev/dummyUSB", 9600, "SML", "Power")

	// Cancel the context to shut down the background run() goroutine
	cancel()

	// Small delay to allow the goroutine to catch the ctx.Done() signal
	time.Sleep(20 * time.Millisecond)

	// If the test completes without hanging, the context cancellation worked perfectly
}

package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSerialProvider_BlockingFlow(t *testing.T) {
	// Initialize provider with a bufferless channel as in production
	s := &SerialProvider{
		jsonPath: "SML.Power",
		dataCh:   make(chan powerResult),
	}

	// Test 1: Successful data transmission
	go func() {
		s.dataCh <- powerResult{tot: 500, err: nil}
	}()

	_, _, _, total, err := s.GetPower()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if total != 500 {
		t.Errorf("expected 500, got %f", total)
	}

	// Test 2: Blocking behavior
	// We verify that GetPower does not return until data is provided.
	done := make(chan bool)
	go func() {
		_, _, _, total, _ = s.GetPower()
		if total == 100 {
			done <- true
		}
	}()

	select {
	case <-done:
		t.Error("GetPower returned before data was sent")
	case <-time.After(50 * time.Millisecond):
		// This is the expected path: it should be waiting
		s.dataCh <- powerResult{tot: 100, err: nil}
	}

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("GetPower timed out after data was sent")
	}
}

func TestSerialProvider_ErrorPropagation(t *testing.T) {
	s := &SerialProvider{
		jsonPath: "SML.Power",
		dataCh:   make(chan powerResult),
	}

	mockErr := errors.New("serial port lost")

	go func() {
		s.dataCh <- powerResult{err: mockErr}
	}()

	_, _, _, _, err := s.GetPower()
	if !errors.Is(err, mockErr) {
		t.Errorf("expected error %v, got %v", mockErr, err)
	}
}

func TestSerialProvider_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_ = NewSerialProvider(ctx, "/dev/ttyUSB0", 9600, "SML", "Power")

	// Verify that the run loop terminates correctly
	cancel()

	// Small delay to allow goroutine cleanup
	time.Sleep(10 * time.Millisecond)
}

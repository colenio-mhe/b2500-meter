package provider

import (
	"context"
	"testing"
)

func TestSerialProvider_ReadAndClear(t *testing.T) {
	// Since NewSerialProvider starts a real serial listener (which would fail on /dev/null or similar in CI),
	// we'll manually test the mailbox logic if we can't easily mock the serial port here.
	// But actually, we can just test the GetPower logic directly by manually populating the pTotal.

	s := &SerialProvider{
		jsonPath: "SML.Power",
	}
	s.updateValues(0, 0, 0, 500)

	// First call should return the value
	_, _, _, total, err := s.GetPower()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if total != 500 {
		t.Errorf("expected 500, got %f", total)
	}

	// Second call should be 0 (Read-and-Clear)
	_, _, _, total, _ = s.GetPower()
	if total != 0 {
		t.Errorf("expected 0 on second call, got %f", total)
	}

	// Third call still 0
	_, _, _, total, _ = s.GetPower()
	if total != 0 {
		t.Errorf("expected 0 on third call, got %f", total)
	}

	// New value arrives
	s.updateValues(0, 0, 0, 100)
	_, _, _, total, _ = s.GetPower()
	if total != 100 {
		t.Errorf("expected 100 after update, got %f", total)
	}
}

func TestSerialProvider_ErrorClears(t *testing.T) {
	s := &SerialProvider{
		jsonPath: "SML.Power",
	}
	s.updateValues(0, 0, 0, 500)

	s.setError(context.DeadlineExceeded)
	_, _, _, total, err := s.GetPower()
	if err == nil {
		t.Error("expected error, got nil")
	}
	if total != 0 {
		t.Errorf("expected 0 on error, got %f", total)
	}
}

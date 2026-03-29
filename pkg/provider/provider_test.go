package provider

import "testing"

func TestMockProvider(t *testing.T) {
	initialPower := -250.0
	p := NewMockProvider(initialPower)

	pA, pB, pC, total, _ := p.GetPower()
	if pA != initialPower {
		t.Errorf("expected phase A %f, got %f", initialPower, pA)
	}
	if pB != 0 {
		t.Errorf("expected phase B 0, got %f", pB)
	}
	if pC != 0 {
		t.Errorf("expected phase C 0, got %f", pC)
	}
	if total != initialPower {
		t.Errorf("expected total %f, got %f", initialPower, total)
	}

	newPower := 100.0
	p.SetPower(newPower)
	pA, _, _, total, _ = p.GetPower()
	if pA != newPower {
		t.Errorf("expected phase A %f, got %f", newPower, pA)
	}
	if total != newPower {
		t.Errorf("expected total %f, got %f", newPower, total)
	}
}

func TestMultiProvider(t *testing.T) {
	p1 := NewMockProvider(100.0)
	p2 := NewMockProvider(50.0)
	multi := NewMultiProvider([]PowerProvider{p1, p2})

	pA, _, _, total, _ := multi.GetPower()
	if pA != 150.0 {
		t.Errorf("expected phase A 150.0, got %f", pA)
	}
	if total != 150.0 {
		t.Errorf("expected total 150.0, got %f", total)
	}
}

package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	yamlContent := `
providers:
  - type: tasmota
    ip: 1.2.3.4
    status: StatusSNS
    payload: SML
    label: Power
  - type: mock
    power: 123.4
`
	tmpfile, err := os.CreateTemp("", "config*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(yamlContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(cfg.Providers))
	}

	p1 := cfg.Providers[0]
	if p1.Type != "tasmota" || p1.IP != "1.2.3.4" || p1.Status != "StatusSNS" {
		t.Errorf("p1 mismatch: %+v", p1)
	}

	p2 := cfg.Providers[1]
	if p2.Type != "mock" || p2.Power != 123.4 {
		t.Errorf("p2 mismatch: %+v", p2)
	}
}

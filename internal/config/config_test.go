package config

import (
	"os"
	"path/filepath"
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
  - type: mqtt
    broker: localhost
    port: 1883
    topic: tele/sensor/SENSOR
    json_path: ENERGY.Power
`
	tmpfile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(tmpfile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmpfile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Providers) != 3 {
		t.Errorf("expected 3 providers, got %d", len(cfg.Providers))
	}

	p1 := cfg.Providers[0]
	if p1.Type != "tasmota" || p1.IP != "1.2.3.4" || p1.Status != "StatusSNS" {
		t.Errorf("p1 mismatch: %+v", p1)
	}

	p2 := cfg.Providers[1]
	if p2.Type != "mock" || p2.Power != 123.4 {
		t.Errorf("p2 mismatch: %+v", p2)
	}

	p3 := cfg.Providers[2]
	if p3.Type != "mqtt" || p3.Broker != "localhost" || p3.Port != 1883 || p3.Topic != "tele/sensor/SENSOR" || p3.JsonPath != "ENERGY.Power" {
		t.Errorf("p3 mismatch: %+v", p3)
	}
}

package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTasmotaProvider_GetPower(t *testing.T) {
	tests := []struct {
		name               string
		jsonPowerCalculate bool
		response           map[string]any
		expectedPower      float64
	}{
		{
			name:               "Single value power",
			jsonPowerCalculate: false,
			response: map[string]any{
				"StatusSNS": map[string]any{
					"ENERGY": map[string]any{
						"Power": 123,
					},
				},
			},
			expectedPower: 123,
		},
		{
			name:               "Calculated power (in - out)",
			jsonPowerCalculate: true,
			response: map[string]any{
				"StatusSNS": map[string]any{
					"ENERGY": map[string]any{
						"PowerIn":  500,
						"PowerOut": 200,
					},
				},
			},
			expectedPower: 300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.RawQuery, "cmnd=status+10") && !strings.Contains(r.URL.RawQuery, "cmnd=status%2010") {
					t.Errorf("Expected status 10 command, got %s", r.URL.RawQuery)
				}
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			// Extract IP and Port from test server
			ipPort := strings.TrimPrefix(server.URL, "http://")

			p := NewTasmotaProvider(
				ipPort,
				"", "", // no auth
				"StatusSNS",
				"ENERGY",
				"Power",
				"PowerIn",
				"PowerOut",
				"", "", "", // no custom paths
				tt.jsonPowerCalculate,
			)

			_, _, _, total, err := p.GetPower()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if total != tt.expectedPower {
				t.Errorf("expected total power %v, got %v", tt.expectedPower, total)
			}
		})
	}
}

func TestTasmotaProvider_Auth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("user") != "admin" || query.Get("password") != "secret" {
			t.Errorf("expected auth params, got %v", r.URL.RawQuery)
		}

		resp := map[string]any{
			"StatusSNS": map[string]any{
				"ENERGY": map[string]any{
					"Power": 456,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ipPort := strings.TrimPrefix(server.URL, "http://")
	p := NewTasmotaProvider(
		ipPort,
		"admin", "secret",
		"StatusSNS",
		"ENERGY",
		"Power",
		"PowerIn",
		"PowerOut",
		"", "", "", // no custom paths
		false,
	)

	_, _, _, total, err := p.GetPower()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 456 {
		t.Errorf("expected total power 456, got %v", total)
	}
}

func TestTasmotaProvider_JsonPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond with a complex/non-standard JSON structure
		resp := `{
			"StatusSNS": {
				"Meter": {
					"Power": 789.5
				},
				"In": "100.2",
				"Out": "50.1"
			}
		}`
		w.Write([]byte(resp))
	}))
	defer server.Close()

	ipPort := strings.TrimPrefix(server.URL, "http://")

	t.Run("Custom JSON path single", func(t *testing.T) {
		p := NewTasmotaProvider(
			ipPort, "", "",
			"", "", "", "", "", // Ignore standard fields
			"StatusSNS.Meter.Power", "", "",
			false,
		)
		_, _, _, total, err := p.GetPower()
		if err != nil {
			t.Fatal(err)
		}
		if total != 789.5 {
			t.Errorf("expected 789.5, got %v", total)
		}
	})

	t.Run("Custom JSON path calculated (with string values)", func(t *testing.T) {
		p := NewTasmotaProvider(
			ipPort, "", "",
			"", "", "", "", "", // Ignore standard fields
			"", "StatusSNS.In", "StatusSNS.Out",
			true,
		)
		_, _, _, total, err := p.GetPower()
		if err != nil {
			t.Fatal(err)
		}
		// 100.2 - 50.1 = 50.1
		if total != 50.1 {
			t.Errorf("expected 50.1, got %v", total)
		}
	})
}

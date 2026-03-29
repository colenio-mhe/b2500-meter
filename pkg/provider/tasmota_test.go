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

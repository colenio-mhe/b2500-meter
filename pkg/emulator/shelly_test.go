package emulator

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"b2500-meter-go/pkg/api"
	"b2500-meter-go/pkg/provider"
)

func TestShellyPro3EMHandler_Rounding(t *testing.T) {
	h := &ShellyPro3EMHandler{DeviceID: "test"}

	tests := []struct {
		name     string
		power    float64
		expected float64
	}{
		{"Zero", 0, 0},
		{"Small Positive", 0.05, 0.1},
		{"Small Negative", -0.05, -0.1},
		{"Negative Zero Rounding", -0.04, 0},
		{"Integer", 200, 200},
		{"Integer Negative", -100, -100},
		{"Decimal", 123.4, 123.4},
		{"Decimal Many", 123.456, 123.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := float64(h.round(tt.power))
			if math.Abs(got-tt.expected) > 0.0001 {
				t.Errorf("round(%v) = %v; want %v", tt.power, got, tt.expected)
			}
			if math.Signbit(got) && got == 0 {
				t.Errorf("round(%v) returned negative zero", tt.power)
			}
		})
	}
}

func TestShellyPro3EMHandler_JsonDecimalPoint(t *testing.T) {
	h := &ShellyPro3EMHandler{DeviceID: "test"}
	p := provider.NewMockProvider(200.0)

	req := api.RpcRequest{ID: 1, Method: "EM.GetStatus"}
	respAny, handled := h.Handle(req, p)
	if !handled {
		t.Fatal("expected handled")
	}

	data, err := json.Marshal(respAny)
	if err != nil {
		t.Fatal(err)
	}

	// Integer-valued powers must still serialize with a decimal point for the battery firmware.
	for _, want := range []string{`"a_act_power":200.0`, `"b_act_power":0.0`, `"total_act_power":200.0`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected JSON to contain %s, got %s", want, data)
		}
	}
}

func TestShellyPro3EMHandler_Handle(t *testing.T) {
	h := &ShellyPro3EMHandler{DeviceID: "test-id"}
	p := provider.NewMockProvider(200.0)

	t.Run("EM.GetStatus", func(t *testing.T) {
		req := api.RpcRequest{ID: 123, Method: "EM.GetStatus"}
		respAny, handled := h.Handle(req, p)
		if !handled {
			t.Fatal("expected handled")
		}
		resp := respAny.(api.RpcResponse)
		if resp.ID != 123 {
			t.Errorf("ID mismatch: %v", resp.ID)
		}
		if resp.Src != "test-id" {
			t.Errorf("Src mismatch: %v", resp.Src)
		}
		res := resp.Result.(api.EmStatusResponse)
		if res.AActPower != 200.0 {
			t.Errorf("AActPower mismatch: %v", res.AActPower)
		}
	})

	t.Run("EM1.GetStatus", func(t *testing.T) {
		req := api.RpcRequest{ID: 456, Method: "EM1.GetStatus"}
		respAny, handled := h.Handle(req, p)
		if !handled {
			t.Fatal("expected handled")
		}
		resp := respAny.(api.RpcResponse)
		res := resp.Result.(api.Em1StatusResponse)
		if res.ActPower != 200.0 {
			t.Errorf("ActPower mismatch: %v", res.ActPower)
		}
	})
}

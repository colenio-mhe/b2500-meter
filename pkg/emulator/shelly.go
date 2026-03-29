package emulator

import (
	"log/slog"
	"math"
	"time"

	"b2500-meter-go/pkg/api"
	"b2500-meter-go/pkg/provider"
)

// ShellyPro3EMHandler implements the logic for a Shelly Pro 3EM device.
type ShellyPro3EMHandler struct {
	DeviceID string
}

// Handle processes incoming RPC requests for the Shelly Pro 3EM device.
// It responds to EM.GetStatus, EM1.GetStatus and Shelly.GetStatus methods with the current power data.
func (h *ShellyPro3EMHandler) Handle(req api.RpcRequest, p provider.PowerProvider) (any, bool) {
	if req.Method == "Shelly.GetConfig" {
		return api.RpcResponse{
			ID:  req.ID,
			Src: h.DeviceID,
			Dst: "unknown",
			Result: map[string]any{
				"ble": map[string]any{"enable": false},
				"cloud": map[string]any{
					"enable":    true,
					"server":    "iot.shelly.cloud:6012/jrpc",
					"connected": true,
				},
				"mqtt": map[string]any{"enable": false},
				"sys": map[string]any{
					"cfg_rev": 10,
					"device": map[string]any{
						"name": "Shelly Pro 3EM",
						"mac":  "1234567890AB",
					},
					"unixtime": time.Now().Unix(),
				},
			},
		}, true
	}

	pA, pB, pC, total, err := p.GetPower()
	if err != nil {
		slog.Error("Failed to get power from provider", "error", err)
		return nil, false
	}

	var result any
	switch req.Method {
	case "EM.GetStatus", "Shelly.GetStatus":
		result = api.EmStatusResponse{
			AActPower:     h.round(pA),
			BActPower:     h.round(pB),
			CActPower:     h.round(pC),
			TotalActPower: h.roundTotal(total),
		}
	case "EM1.GetStatus":
		result = api.Em1StatusResponse{
			ActPower: h.roundTotal(total),
		}
	default:
		return nil, false
	}

	return api.RpcResponse{
		ID:     req.ID,
		Src:    h.DeviceID,
		Dst:    "unknown",
		Result: result,
	}, true
}

// round applies rounding and decimal point enforcement
// This ensures that integer values are represented as floats in the JSON response (e.g., 200.001).
func (h *ShellyPro3EMHandler) round(power float64) float64 {
	const decimalPointEnforcer = 0.001
	if math.Abs(power) < 0.1 {
		return decimalPointEnforcer
	}

	rounded := math.Round(power*10) / 10
	if power == math.Round(power) || power == 0 {
		if power < 0 {
			return rounded - decimalPointEnforcer
		}
		return rounded + decimalPointEnforcer
	}
	return rounded
}

// roundTotal applies the rounding logic to the total power value.
func (h *ShellyPro3EMHandler) roundTotal(total float64) float64 {
	const decimalPointEnforcer = 0.001
	rounded := math.Round(total*1000) / 1000
	if total == math.Round(total) || total == 0 {
		if total < 0 {
			return rounded - decimalPointEnforcer
		}
		return rounded + decimalPointEnforcer
	}
	return rounded
}

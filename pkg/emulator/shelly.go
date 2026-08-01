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
	DeviceID     string
	ZeroFallback bool
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
		slog.Debug("Failed to get power from provider", "error", err)

		if h.ZeroFallback {
			slog.Debug("Zero fallback active: sending 0W response")
			pA, pB, pC, total = 0, 0, 0, 0
		} else {
			return nil, false
		}
	}

	var result any
	switch req.Method {
	case "EM.GetStatus", "Shelly.GetStatus":
		result = api.EmStatusResponse{
			AActPower:     h.round(pA),
			BActPower:     h.round(pB),
			CActPower:     h.round(pC),
			TotalActPower: h.round(total),
		}
	case "EM1.GetStatus":
		result = api.Em1StatusResponse{
			ActPower: h.round(total),
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

// round rounds a power value to one decimal place, matching the precision of a real
// Shelly Pro 3EM. The decimal point in the JSON output is guaranteed by api.JsonFloat,
// so no value distortion is needed.
func (h *ShellyPro3EMHandler) round(power float64) api.JsonFloat {
	rounded := math.Round(power*10) / 10

	// Normalize negative zero so the response never contains "-0.0".
	if rounded == 0 {
		rounded = 0
	}

	return api.JsonFloat(rounded)
}

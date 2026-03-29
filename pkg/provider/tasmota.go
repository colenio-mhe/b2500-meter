package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// TasmotaProvider fetches power consumption data from a Tasmota device over HTTP.
// It can either use a single power label or calculate net power (import - export).
type TasmotaProvider struct {
	client                   *http.Client
	ip                       string
	user                     string
	password                 string
	jsonStatus               string
	jsonPayloadMQTTPrefix    string
	jsonPowerMQTTLabel       string
	jsonPowerInputMQTTLabel  string
	jsonPowerOutputMQTTLabel string
	jsonPowerCalculate       bool
}

// NewTasmotaProvider initializes a TasmotaProvider with the necessary fields for HTTP fetching.
func NewTasmotaProvider(
	ip, user, password, jsonStatus, jsonPayloadMQTTPrefix, jsonPowerMQTTLabel,
	jsonPowerInputMQTTLabel, jsonPowerOutputMQTTLabel string,
	jsonPowerCalculate bool,
) *TasmotaProvider {
	return &TasmotaProvider{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		ip:                       ip,
		user:                     user,
		password:                 password,
		jsonStatus:               jsonStatus,
		jsonPayloadMQTTPrefix:    jsonPayloadMQTTPrefix,
		jsonPowerMQTTLabel:       jsonPowerMQTTLabel,
		jsonPowerInputMQTTLabel:  jsonPowerInputMQTTLabel,
		jsonPowerOutputMQTTLabel: jsonPowerOutputMQTTLabel,
		jsonPowerCalculate:       jsonPowerCalculate,
	}
}

// GetPower returns the power readings by fetching them from the Tasmota device.
func (t *TasmotaProvider) GetPower() (phaseA, phaseB, phaseC, total float64, err error) {
	var power float64

	if t.jsonPowerCalculate {
		in, out, fetchErr := t.getPowerInOut()
		if fetchErr != nil {
			slog.Error("Tasmota fetch error (calculated)", "ip", t.ip, "error", fetchErr)
			return 0, 0, 0, 0, fetchErr
		}
		power = float64(int(in) - int(out))
		slog.Debug("Tasmota values", "in", in, "out", out, "net", power)
	} else {
		p, fetchErr := t.getPowerSingle()
		if fetchErr != nil {
			slog.Error("Tasmota fetch error (single)", "ip", t.ip, "error", fetchErr)
			return 0, 0, 0, 0, fetchErr
		}
		power = float64(int(p))
		slog.Debug("Tasmota value", "power", power)
	}

	return power, 0, 0, power, nil
}

func (t *TasmotaProvider) getJSON(path string) (map[string]any, error) {
	reqURL := fmt.Sprintf("http://%s%s", t.ip, path)
	resp, err := t.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (t *TasmotaProvider) getPowerSingle() (float64, error) {
	resp, err := t.fetchStatus10()
	if err != nil {
		return 0, err
	}

	val, err := t.extractValue(resp, t.jsonPowerMQTTLabel)
	if err != nil {
		return 0, err
	}
	return val, nil
}

func (t *TasmotaProvider) getPowerInOut() (in, out float64, err error) {
	resp, err := t.fetchStatus10()
	if err != nil {
		return 0, 0, err
	}

	in, err = t.extractValue(resp, t.jsonPowerInputMQTTLabel)
	if err != nil {
		return 0, 0, err
	}

	out, err = t.extractValue(resp, t.jsonPowerOutputMQTTLabel)
	if err != nil {
		return 0, 0, err
	}

	return in, out, nil
}

func (t *TasmotaProvider) fetchStatus10() (map[string]any, error) {
	var path string
	if t.user == "" {
		path = "/cm?cmnd=status%2010"
	} else {
		params := url.Values{}
		params.Add("user", t.user)
		params.Add("password", t.password)
		params.Add("cmnd", "status 10")
		path = "/cm?" + params.Encode()
	}
	return t.getJSON(path)
}

func (t *TasmotaProvider) extractValue(resp map[string]any, label string) (float64, error) {
	status, ok := resp[t.jsonStatus].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("could not find status key: %s", t.jsonStatus)
	}

	payload, ok := status[t.jsonPayloadMQTTPrefix].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("could not find payload prefix key: %s", t.jsonPayloadMQTTPrefix)
	}

	val, ok := payload[label]
	if !ok {
		return 0, fmt.Errorf("could not find label: %s", label)
	}

	switch v := val.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("unexpected value type for %s: %T", label, val)
	}
}

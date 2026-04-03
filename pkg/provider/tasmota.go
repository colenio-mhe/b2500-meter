package provider

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/tidwall/gjson"
)

// TasmotaProvider fetches power consumption data from a Tasmota device over HTTP.
// It can either use a single power label or calculate net power (import - export).
type TasmotaProvider struct {
	client               *http.Client
	ip                   string
	user                 string
	password             string
	jsonStatus           string
	jsonPayloadPrefix    string
	jsonPowerLabel       string
	jsonPowerInputLabel  string
	jsonPowerOutputLabel string
	jsonPath             string
	jsonPathIn           string
	jsonPathOut          string
	jsonPowerCalculate   bool
}

// NewTasmotaProvider initializes a TasmotaProvider with the necessary fields for HTTP fetching.
func NewTasmotaProvider(
	ip, user, password, jsonStatus, jsonPayloadPrefix, jsonPowerLabel,
	jsonPowerInputLabel, jsonPowerOutputLabel,
	jsonPath, jsonPathIn, jsonPathOut string,
	jsonPowerCalculate bool,
) *TasmotaProvider {
	return &TasmotaProvider{
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		ip:                   ip,
		user:                 user,
		password:             password,
		jsonStatus:           jsonStatus,
		jsonPayloadPrefix:    jsonPayloadPrefix,
		jsonPowerLabel:       jsonPowerLabel,
		jsonPowerInputLabel:  jsonPowerInputLabel,
		jsonPowerOutputLabel: jsonPowerOutputLabel,
		jsonPath:             jsonPath,
		jsonPathIn:           jsonPathIn,
		jsonPathOut:          jsonPathOut,
		jsonPowerCalculate:   jsonPowerCalculate,
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
		power = in - out
		slog.Debug("Tasmota values", "in", in, "out", out, "net", power)
	} else {
		p, fetchErr := t.getPowerSingle()
		if fetchErr != nil {
			slog.Error("Tasmota fetch error (single)", "ip", t.ip, "error", fetchErr)
			return 0, 0, 0, 0, fetchErr
		}
		power = p
		slog.Debug("Tasmota value", "power", power)
	}

	return power, 0, 0, power, nil
}

func (t *TasmotaProvider) fetchBytes(path string) ([]byte, error) {
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

	return body, nil
}

func (t *TasmotaProvider) getPowerSingle() (float64, error) {
	resp, err := t.fetchStatus10()
	if err != nil {
		return 0, err
	}

	val, err := t.extractValue(resp, t.jsonPath, t.jsonPowerLabel)
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

	in, err = t.extractValue(resp, t.jsonPathIn, t.jsonPowerInputLabel)
	if err != nil {
		return 0, 0, err
	}

	out, err = t.extractValue(resp, t.jsonPathOut, t.jsonPowerOutputLabel)
	if err != nil {
		return 0, 0, err
	}

	return in, out, nil
}

func (t *TasmotaProvider) fetchStatus10() ([]byte, error) {
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
	return t.fetchBytes(path)
}

func (t *TasmotaProvider) extractValue(resp []byte, customPath, label string) (float64, error) {
	var path string
	if customPath != "" {
		path = customPath
	} else {
		path = fmt.Sprintf("%s.%s.%s", t.jsonStatus, t.jsonPayloadPrefix, label)
	}

	res := gjson.GetBytes(resp, path)
	if !res.Exists() {
		return 0, fmt.Errorf("JSON path not found: %s", path)
	}

	if res.Type == gjson.Number {
		return res.Float(), nil
	}

	if res.Type == gjson.String {
		val, err := strconv.ParseFloat(res.String(), 64)
		if err != nil {
			return 0, fmt.Errorf("could not parse %s as float: %w", res.String(), err)
		}
		return val, nil
	}

	return 0, fmt.Errorf("unexpected value type for %s: %s", path, res.Type.String())
}

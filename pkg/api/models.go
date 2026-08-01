// Package api defines the JSON RPC models for communicating with the Marstek battery.
package api

import "strconv"

// JsonFloat is a float64 that always marshals with exactly one decimal place (e.g., "200.0").
// The battery firmware expects a real float literal in the JSON response, while Go's
// encoding/json would marshal integer-valued floats without a decimal point.
type JsonFloat float64

// MarshalJSON renders the value with a fixed single decimal place.
func (f JsonFloat) MarshalJSON() ([]byte, error) {
	return strconv.AppendFloat(nil, float64(f), 'f', 1, 64), nil
}

// RpcRequest represents a request coming from the client (e.g., the battery).
type RpcRequest struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
}

// RpcResponse is the root response structure for JSON-RPC 2.0.
type RpcResponse struct {
	ID     int    `json:"id"`
	Src    string `json:"src"`
	Dst    string `json:"dst"`
	Result any    `json:"result"`
}

// EmStatusResponse contains power data for Shelly Pro 3EM devices.
type EmStatusResponse struct {
	AActPower     JsonFloat `json:"a_act_power"`
	BActPower     JsonFloat `json:"b_act_power"`
	CActPower     JsonFloat `json:"c_act_power"`
	TotalActPower JsonFloat `json:"total_act_power"`
}

// Em1StatusResponse contains total power data for single-phase Shelly devices.
type Em1StatusResponse struct {
	ActPower JsonFloat `json:"act_power"`
}

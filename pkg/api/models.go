// Package api defines the JSON RPC models for communicating with the Marstek battery.
package api

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
	AActPower     float64 `json:"a_act_power"`
	BActPower     float64 `json:"b_act_power"`
	CActPower     float64 `json:"c_act_power"`
	TotalActPower float64 `json:"total_act_power"`
}

// Em1StatusResponse contains total power data for single-phase Shelly devices.
type Em1StatusResponse struct {
	ActPower float64 `json:"act_power"`
}

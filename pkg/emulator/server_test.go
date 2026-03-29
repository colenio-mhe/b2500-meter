package emulator

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"b2500-meter-go/pkg/api"
	"b2500-meter-go/pkg/provider"
)

func TestServer_Response(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	p := provider.NewMockProvider(-123.4)
	srv := &Server{
		Port:    0, // Random port
		Handler: &ShellyPro3EMHandler{DeviceID: "test-device"},
		Power:   p,
	}

	// We need to know which port it bound to.
	// Since srv.Run doesn't easily expose the port if it's 0, let's use a fixed one for simplicity in this test or find a better way.
	// Actually, let's listen manually and pass it to Server or just pick a free port.

	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	boundPort := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close() // Close it so Server can use it (there is a race here, but usually okay for tests)

	srv.Port = boundPort

	go func() {
		if err := srv.Run(ctx); err != nil {
			t.Logf("Server exited: %v", err)
		}
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	clientConn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: boundPort})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	req := api.RpcRequest{
		ID:     1,
		Method: "EM.GetStatus",
	}
	reqData, _ := json.Marshal(req)

	_, err = clientConn.Write(reqData)
	if err != nil {
		t.Fatal(err)
	}

	buffer := make([]byte, 2048)
	_ = clientConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, _, err := clientConn.ReadFromUDP(buffer)
	if err != nil {
		t.Fatal(err)
	}

	var resp api.RpcResponse
	if err := json.Unmarshal(buffer[:n], &resp); err != nil {
		t.Fatal(err)
	}

	if resp.ID != 1 {
		t.Errorf("expected ID 1, got %d", resp.ID)
	}
	if resp.Src != "test-device" {
		t.Errorf("expected Src 'test-device', got %s", resp.Src)
	}

	// Result should be EmStatusResponse
	resultData, _ := json.Marshal(resp.Result)
	var emResp api.EmStatusResponse
	if err := json.Unmarshal(resultData, &emResp); err != nil {
		t.Fatal(err)
	}

	// -123.4 is not an integer, so it should be rounded to -123.4 (1 decimal)
	if emResp.TotalActPower != -123.4 {
		t.Errorf("expected -123.4, got %f", emResp.TotalActPower)
	}
}

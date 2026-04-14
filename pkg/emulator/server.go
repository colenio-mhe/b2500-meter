// Package emulator implements the UDP server that mimics a specific power meter device.
package emulator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"b2500-meter-go/pkg/api"
	"b2500-meter-go/pkg/provider"
)

// DeviceHandler handles different device emulation types.
type DeviceHandler interface {
	Handle(req api.RpcRequest, p provider.PowerProvider) (any, bool)
}

// Server is a UDP listener that responds to status requests using a device handler.
type Server struct {
	Port    int
	Handler DeviceHandler
	Power   provider.PowerProvider

	mu          sync.Mutex
	seenClients map[string]bool
}

// Run starts the UDP server and blocks until the context is canceled.
func (s *Server) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.seenClients == nil {
		s.seenClients = make(map[string]bool)
	}
	s.mu.Unlock()
	addr := net.UDPAddr{
		Port: s.Port,
		IP:   net.ParseIP("0.0.0.0"),
	}

	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		return fmt.Errorf("failed to bind UDP port %d: %w", s.Port, err)
	}
	defer conn.Close()

	slog.Info("Emulator listening", "port", s.Port)

	go func() {
		buffer := make([]byte, 2048)
		for {
			n, clientAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					slog.Error("UDP Read error", "port", s.Port, "error", err)
					continue
				}
			}

			// Capture data for the goroutine
			data := make([]byte, n)
			copy(data, buffer[:n])

			//go s.handleRequest(ctx, conn, data, clientAddr)
			// We try synchron handling for now
			s.handleRequest(ctx, conn, data, clientAddr)
		}
	}()

	<-ctx.Done()
	return nil
}

func (s *Server) handleRequest(ctx context.Context, conn *net.UDPConn, data []byte, clientAddr *net.UDPAddr) {
	var req api.RpcRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}

	if resp, handled := s.Handler.Handle(req, s.Power); handled {
		clientIP := clientAddr.IP.String()
		s.mu.Lock()
		if !s.seenClients[clientIP] {
			s.seenClients[clientIP] = true
			slog.Info("Device connected", "port", s.Port, "client", clientIP)
		}
		s.mu.Unlock()

		respData, err := json.Marshal(resp)
		if err != nil {
			slog.Error("JSON serialization error", "port", s.Port, "error", err)
			return
		}

		_, err = conn.WriteToUDP(respData, clientAddr)
		if err != nil {
			slog.Error("Failed to send reply", "port", s.Port, "client", clientAddr.String(), "error", err)
		} else {
			slog.Debug("Responded to client", "port", s.Port, "client", clientAddr.String(), "method", req.Method)
		}
	}
}

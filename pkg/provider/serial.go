package provider

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/tidwall/gjson"
	"go.bug.st/serial"
)

// SerialProvider reads telemetry from a USB serial port and caches the latest value.
type SerialProvider struct {
	portName string
	baudRate int
	jsonPath string

	mu      sync.Mutex
	lastVal float64
	isFresh bool
	lastErr error // Stores connection errors so the UDP server can fail fast
}

// NewSerialProvider initializes the provider and starts the background reader.
func NewSerialProvider(ctx context.Context, portName string, baudRate int, payload, label string) *SerialProvider {
	path := fmt.Sprintf("%s.%s", payload, label)
	s := &SerialProvider{
		portName: portName,
		baudRate: baudRate,
		jsonPath: path,
	}

	// Start the non-blocking reader loop
	go s.run(ctx)

	return s
}

func (s *SerialProvider) run(ctx context.Context) {
	mode := &serial.Mode{
		BaudRate: s.baudRate,
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			port, err := serial.Open(s.portName, mode)
			if err != nil {
				slog.Error("Serial: Could not open port", "port", s.portName, "error", err)
				s.setError(err)

				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
					continue
				}
			}

			slog.Info("Serial: Listening on USB port", "port", s.portName, "baud", s.baudRate)
			s.setError(nil) // Clear any previous connection errors

			scanner := bufio.NewScanner(port)
			for scanner.Scan() {
				line := scanner.Bytes()

				idx := bytes.IndexByte(line, '{')
				if idx == -1 {
					continue
				}
				jsonBytes := line[idx:]

				res := gjson.GetBytes(jsonBytes, s.jsonPath)
				if res.Exists() {
					var val float64
					if res.Type == gjson.Number {
						val = res.Float()
					} else if res.Type == gjson.String {
						if parsed, err := strconv.ParseFloat(res.String(), 64); err == nil {
							val = parsed
						}
					}

					// Safely update the cache and flag the value as fresh
					s.mu.Lock()
					s.lastVal = val
					s.isFresh = true
					s.lastErr = nil
					s.mu.Unlock()
				}
			}

			if err := scanner.Err(); err != nil {
				slog.Error("Serial: Error reading", "error", err)
				s.setError(err)
			}
			port.Close()

			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
				continue
			}
		}
	}
}

// setError safely updates the connection error state.
func (s *SerialProvider) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastErr = err
}

// GetPower sends the fresh value or returns an error if data is stale/port is down.
func (s *SerialProvider) GetPower() (phaseA, phaseB, phaseC, total float64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastErr != nil {
		return 0, 0, 0, 0, s.lastErr
	}

	if !s.isFresh {
		return 0, 0, 0, 0, errors.New("stale data: wait for new serial update")
	}

	s.isFresh = false
	return 0, 0, 0, s.lastVal, nil
}

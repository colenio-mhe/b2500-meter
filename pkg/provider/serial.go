package provider

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/tidwall/gjson"
	"go.bug.st/serial"
)

// SerialProvider reads continuous JSON lines from a serial port (USB)
// and acts as a mailbox for the Marstek battery.
type SerialProvider struct {
	portName string
	baudRate int
	jsonPath string

	mu        sync.Mutex
	pA        float64
	pB        float64
	pC        float64
	pTotal    float64
	lastError error
}

// NewSerialProvider initializes a SerialProvider and starts the background listener.
func NewSerialProvider(ctx context.Context, portName string, baudRate int, payload, label string) *SerialProvider {
	path := fmt.Sprintf("%s.%s", payload, label)
	s := &SerialProvider{
		portName: portName,
		baudRate: baudRate,
		jsonPath: path,
	}

	// Starts the background listening process
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
				// Wait before retry
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
					continue
				}
			}

			slog.Info("Serial: Listening on USB port", "port", s.portName, "baud", s.baudRate)

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
					if res.Type == gjson.Number {
						s.updateValues(0, 0, 0, res.Float())
					} else if res.Type == gjson.String {
						if val, err := strconv.ParseFloat(res.String(), 64); err == nil {
							s.updateValues(0, 0, 0, val)
						}
					}
				}
			}

			if err := scanner.Err(); err != nil {
				slog.Error("Serial: Error reading", "error", err)
				s.setError(err)
			}
			port.Close()

			// Wait before reconnecting
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
				continue
			}
		}
	}
}

func (s *SerialProvider) updateValues(a, b, c, total float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pA, s.pB, s.pC, s.pTotal = a, b, c, total
	s.lastError = nil
}

func (s *SerialProvider) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pA, s.pB, s.pC, s.pTotal = 0, 0, 0, 0
	s.lastError = err
}

// GetPower returns the most recently received power value and IMMEDIATELY clears it.
// This prevents the provider from returning stale values if the serial stream stops.
func (s *SerialProvider) GetPower() (float64, float64, float64, float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, b, c, tot := s.pA, s.pB, s.pC, s.pTotal
	currentErr := s.lastError

	// Clear the values after reading (Read-and-Clear)
	s.pA, s.pB, s.pC, s.pTotal = 0, 0, 0, 0
	s.lastError = nil

	return a, b, c, tot, currentErr
}

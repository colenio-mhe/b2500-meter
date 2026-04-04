package provider

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/tidwall/gjson"
	"go.bug.st/serial"
)

type powerResult struct {
	a, b, c, tot float64
	err          error
}

// SerialProvider blocks GetPower calls until a new telegram is received via USB.
type SerialProvider struct {
	portName string
	baudRate int
	jsonPath string
	dataCh   chan powerResult
}

// NewSerialProvider initializes the provider and the internal signaling channel.
func NewSerialProvider(ctx context.Context, portName string, baudRate int, payload, label string) *SerialProvider {
	path := fmt.Sprintf("%s.%s", payload, label)
	s := &SerialProvider{
		portName: portName,
		baudRate: baudRate,
		jsonPath: path,
		dataCh:   make(chan powerResult),
	}

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
				s.broadcastError(ctx, err)
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
					var val float64
					if res.Type == gjson.Number {
						val = res.Float()
					} else if res.Type == gjson.String {
						if parsed, err := strconv.ParseFloat(res.String(), 64); err == nil {
							val = parsed
						}
					}

					// Block until a consumer is ready or context is canceled
					select {
					case <-ctx.Done():
						port.Close()
						return
					case s.dataCh <- powerResult{a: 0, b: 0, c: 0, tot: val, err: nil}:
					}
				}
			}

			if err := scanner.Err(); err != nil {
				slog.Error("Serial: Error reading", "error", err)
				s.broadcastError(ctx, err)
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

func (s *SerialProvider) broadcastError(ctx context.Context, err error) {
	select {
	case <-ctx.Done():
	case s.dataCh <- powerResult{err: err}:
	default:
		// Do not block if no one is listening to an error
	}
}

// GetPower blocks until a new JSON object is parsed from the serial stream.
func (s *SerialProvider) GetPower() (float64, float64, float64, float64, error) {
	res := <-s.dataCh
	return res.a, res.b, res.c, res.tot, res.err
}

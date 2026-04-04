package provider

import (
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tidwall/gjson"
)

// MqttProvider fetches power consumption data from an MQTT broker.
// It implements a Read-and-Clear strategy to ensure each update is processed exactly once.
type MqttProvider struct {
	mu        sync.Mutex
	pA        float64
	pB        float64
	pC        float64
	pTotal    float64
	received  bool // Tracking if ANY message has arrived (even 0W)
	lastError error
	client    mqtt.Client
	topic     string
	jsonPath  string
}

// NewMqttProvider initializes the MQTT client and sets up the reconnection handler.
func NewMqttProvider(broker string, port int, topic, user, password, jsonPath string) (*MqttProvider, error) {
	p := &MqttProvider{
		topic:    topic,
		jsonPath: jsonPath,
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
	if user != "" && password != "" {
		opts.SetUsername(user)
		opts.SetPassword(password)
	}
	opts.SetClientID(fmt.Sprintf("b2500-meter-go-%d", time.Now().UnixNano()))
	opts.SetAutoReconnect(true)

	// Resubscribe on every connect to handle broker restarts
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		slog.Info("MQTT connected", "broker", broker, "port", port, "topic", topic)
		if token := c.Subscribe(topic, 0, p.onMessage); token.Wait() && token.Error() != nil {
			slog.Error("MQTT subscribe error", "topic", topic, "error", token.Error())
		}
	})

	p.client = mqtt.NewClient(opts)
	if token := p.client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("MQTT connection error: %w", token.Error())
	}

	return p, nil
}

// onMessage updates the state when a new MQTT message is received.
func (p *MqttProvider) onMessage(_ mqtt.Client, msg mqtt.Message) {
	payload := msg.Payload()
	var val float64
	var err error

	if p.jsonPath != "" {
		res := gjson.GetBytes(payload, p.jsonPath)
		if !res.Exists() {
			slog.Error("MQTT JSON path not found", "path", p.jsonPath)
			return
		}
		val, err = strconv.ParseFloat(res.String(), 64)
		if err != nil {
			slog.Error("MQTT parse error", "path", p.jsonPath, "error", err)
			return
		}
	} else {
		val, err = strconv.ParseFloat(string(payload), 64)
		if err != nil {
			slog.Error("MQTT payload parse error", "error", err)
			return
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.pA, p.pTotal = val, val
	p.pB, p.pC = 0, 0
	p.received = true // Mark as valid data received
	p.lastError = nil
	slog.Debug("MQTT received value", "topic", p.topic, "value", val)
}

// setError resets the power values and stores the current error state.
func (p *MqttProvider) setError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pA, p.pB, p.pC, p.pTotal = 0, 0, 0, 0
	p.lastError = err
}

// GetPower returns the latest values and resets them to 0 (Read-and-Clear).
// This ensures that Marstek sees 0 (no change) if no new MQTT message has arrived.
func (p *MqttProvider) GetPower() (phaseA, phaseB, phaseC, total float64, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	a, b, c, tot := p.pA, p.pB, p.pC, p.pTotal
	currentErr := p.lastError

	// Reset for next poll (Read-and-Clear to 0)
	p.pA, p.pB, p.pC, p.pTotal = 0, 0, 0, 0
	p.lastError = nil

	return a, b, c, tot, currentErr
}

// WaitForMessage blocks until the first valid message or error is received.
func (p *MqttProvider) WaitForMessage(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		if p.received || p.lastError != nil {
			p.mu.Unlock()
			return nil
		}
		p.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for MQTT message on topic %s", p.topic)
}

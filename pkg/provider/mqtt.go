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
// It subscribes to a topic and updates its internal value whenever a new message is received.
type MqttProvider struct {
	mu       sync.RWMutex
	value    *float64
	client   mqtt.Client
	topic    string
	jsonPath string
}

// NewMqttProvider initializes an MqttProvider and starts the MQTT connection.
// It subscribes to the specified topic and uses the optional jsonPath to extract the power value.
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

	// Set a handler that will be called when the client connects.
	// This ensures that we re-subscribe upon reconnection.
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

// onMessage is the callback function that handles incoming MQTT messages.
func (p *MqttProvider) onMessage(_ mqtt.Client, msg mqtt.Message) {
	payload := msg.Payload()
	var val float64
	var err error

	if p.jsonPath != "" {
		res := gjson.GetBytes(payload, p.jsonPath)
		if !res.Exists() {
			slog.Error("MQTT JSON path not found", "path", p.jsonPath, "payload", string(payload))
			return
		}
		if res.Type == gjson.JSON {
			slog.Error("MQTT JSON path points to an object or array", "path", p.jsonPath)
			return
		}
		// res.Float() will try to parse strings as well, but if it's not a number it returns 0.
		// We want to be sure it's at least a number or a string that looks like a number.
		if res.Type != gjson.Number && res.Type != gjson.String {
			slog.Error("MQTT JSON path value is not a number or string", "path", p.jsonPath, "type", res.Type.String())
			return
		}

		val, err = strconv.ParseFloat(res.String(), 64)
		if err != nil {
			slog.Error("MQTT JSON path value is not a valid float", "path", p.jsonPath, "value", res.String(), "error", err)
			return
		}
	} else {
		val, err = strconv.ParseFloat(string(payload), 64)
		if err != nil {
			slog.Error("MQTT payload is not a float", "payload", string(payload), "error", err)
			return
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.value = &val
	slog.Debug("MQTT received value", "topic", p.topic, "value", val)
}

// GetPower returns the most recently received power value and IMMEDIATELY clears it.
// This prevents the provider from returning stale values if the MQTT stream stops.
// It satisfies the PowerProvider interface.
func (p *MqttProvider) GetPower() (phaseA, phaseB, phaseC, total float64, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.value == nil {
		return 0, 0, 0, 0, fmt.Errorf("no value received from MQTT topic %s yet", p.topic)
	}

	val := *p.value
	// Clear the value after reading (Read-and-Clear)
	p.value = nil

	return val, 0, 0, val, nil
}

// WaitForMessage blocks until the first message is received or the timeout is reached.
func (p *MqttProvider) WaitForMessage(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		p.mu.RLock()
		if p.value != nil {
			p.mu.RUnlock()
			return nil
		}
		p.mu.RUnlock()
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for MQTT message on topic %s", p.topic)
}

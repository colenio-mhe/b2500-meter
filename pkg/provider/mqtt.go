package provider

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tidwall/gjson"
)

// MqttProvider blocks GetPower calls until a new message is received from the broker.
type MqttProvider struct {
	client   mqtt.Client
	topic    string
	jsonPath string

	mu      sync.Mutex
	lastVal float64
	isFresh bool
}

// NewMqttProvider initializes the MQTT client and the signaling channel.
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

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		slog.Info("MQTT connected", "broker", broker, "topic", topic)
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
	} else {
		val, err = strconv.ParseFloat(string(payload), 64)
	}

	if err != nil {
		slog.Error("MQTT parse error", "error", err)
		return
	}

	// Refresh Value
	p.mu.Lock()
	p.lastVal = val
	p.isFresh = true
	p.mu.Unlock()
}

// GetPower sends fresh value or returns stale data
func (p *MqttProvider) GetPower() (phaseA, phaseB, phaseC, total float64, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isFresh {
		return 0, 0, 0, 0, errors.New("stale data: wait for new MQTT update")
	}

	p.isFresh = false
	return p.lastVal, 0, 0, p.lastVal, nil
}

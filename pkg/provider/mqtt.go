package provider

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tidwall/gjson"
)

type mqttResult struct {
	val float64
	err error
}

// MqttProvider blocks GetPower calls until a new message is received from the broker.
type MqttProvider struct {
	client   mqtt.Client
	topic    string
	jsonPath string
	dataCh   chan mqttResult
}

// NewMqttProvider initializes the MQTT client and the signaling channel.
func NewMqttProvider(broker string, port int, topic, user, password, jsonPath string) (*MqttProvider, error) {
	p := &MqttProvider{
		topic:    topic,
		jsonPath: jsonPath,
		dataCh:   make(chan mqttResult),
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

	// Non-blocking send: if no one is calling GetPower, the message is dropped
	// to ensure the next caller always gets the freshest possible data.
	select {
	case p.dataCh <- mqttResult{val: val, err: nil}:
		slog.Debug("MQTT message forwarded to consumer", "value", val)
	default:
		// Buffer is full or no listener, drop old value
	}
}

// GetPower blocks until the next MQTT message arrives on the subscribed topic.
func (p *MqttProvider) GetPower() (phaseA, phaseB, phaseC, total float64, err error) {
	res := <-p.dataCh
	if res.err != nil {
		return 0, 0, 0, 0, res.err
	}
	return res.val, 0, 0, res.val, nil
}

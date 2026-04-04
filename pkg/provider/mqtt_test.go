package provider

import (
	"testing"
	"time"
)

type mockMessage struct {
	payload []byte
}

func (m *mockMessage) Duplicate() bool   { return false }
func (m *mockMessage) Qos() byte         { return 0 }
func (m *mockMessage) Retained() bool    { return false }
func (m *mockMessage) Topic() string     { return "test" }
func (m *mockMessage) MessageID() uint16 { return 0 }
func (m *mockMessage) Payload() []byte   { return m.payload }
func (m *mockMessage) Ack()              {}

func TestMqttProvider_Flow(t *testing.T) {
	t.Run("successful data parsing and blocking", func(t *testing.T) {
		p := &MqttProvider{
			jsonPath: "ENERGY.Power",
			dataCh:   make(chan mqttResult),
		}

		payload := `{"ENERGY": {"Power": 123.45}}`
		msg := &mockMessage{payload: []byte(payload)}

		// Start GetPower in a goroutine because it blocks
		resultChan := make(chan float64)
		go func() {
			val, _, _, _, _ := p.GetPower()
			resultChan <- val
		}()

		// Small sleep to ensure GetPower is waiting on the channel
		time.Sleep(10 * time.Millisecond)
		p.onMessage(nil, msg)

		select {
		case val := <-resultChan:
			if val != 123.45 {
				t.Errorf("expected 123.45, got %v", val)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout: GetPower did not receive the message")
		}
	})

	t.Run("drop message when no listener is active", func(t *testing.T) {
		p := &MqttProvider{
			dataCh: make(chan mqttResult),
		}

		// Send message while NO GetPower is waiting
		msg := &mockMessage{payload: []byte(`100.0`)}
		p.onMessage(nil, msg)

		// Now start a listener
		resultChan := make(chan float64)
		go func() {
			val, _, _, _, _ := p.GetPower()
			resultChan <- val
		}()

		// This should block because the 100.0 was dropped
		select {
		case <-resultChan:
			t.Error("expected to block, but received a dropped value")
		case <-time.After(50 * time.Millisecond):
			// Success: No data received yet
		}

		// Send a new fresh value
		p.onMessage(nil, &mockMessage{payload: []byte(`200.0`)})
		val := <-resultChan
		if val != 200.0 {
			t.Errorf("expected 200.0, got %v", val)
		}
	})

	t.Run("raw payload without json path", func(t *testing.T) {
		p := &MqttProvider{
			dataCh: make(chan mqttResult),
		}

		go func() {
			time.Sleep(10 * time.Millisecond)
			p.onMessage(nil, &mockMessage{payload: []byte(`456.78`)})
		}()

		val, _, _, _, _ := p.GetPower()
		if val != 456.78 {
			t.Errorf("expected 456.78, got %v", val)
		}
	})

	t.Run("invalid payload is ignored", func(t *testing.T) {
		p := &MqttProvider{
			dataCh: make(chan mqttResult),
		}

		// Start listener
		resultChan := make(chan bool)
		go func() {
			p.GetPower()
			resultChan <- true
		}()

		// Send invalid data - onMessage should log error and NOT send to channel
		p.onMessage(nil, &mockMessage{payload: []byte(`invalid`)})

		select {
		case <-resultChan:
			t.Error("GetPower should have remained blocked after invalid payload")
		case <-time.After(50 * time.Millisecond):
			// Success
		}
	})
}

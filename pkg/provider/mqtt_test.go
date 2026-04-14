package provider

import (
	"testing"
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
	t.Run("successful data parsing and stale flag enforcement", func(t *testing.T) {
		p := &MqttProvider{
			jsonPath: "ENERGY.Power",
		}

		payload := `{"ENERGY": {"Power": 123.45}}`
		msg := &mockMessage{payload: []byte(payload)}

		// 1. Simulate incoming MQTT message
		p.onMessage(nil, msg)

		// 2. First call should succeed and return the fresh value
		val, _, _, _, err := p.GetPower()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if val != 123.45 {
			t.Errorf("expected 123.45, got %v", val)
		}

		// 3. Second call immediately after should fail because the value is now stale
		_, _, _, _, err = p.GetPower()
		if err == nil {
			t.Error("expected stale data error on second call, but got nil")
		}
	})

	t.Run("raw payload without json path", func(t *testing.T) {
		p := &MqttProvider{}

		// Simulate incoming raw MQTT message
		p.onMessage(nil, &mockMessage{payload: []byte(`456.78`)})

		val, _, _, _, err := p.GetPower()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if val != 456.78 {
			t.Errorf("expected 456.78, got %v", val)
		}
	})

	t.Run("invalid payload is safely ignored", func(t *testing.T) {
		p := &MqttProvider{}

		// Send invalid data - onMessage should log an error and abort
		p.onMessage(nil, &mockMessage{payload: []byte(`invalid_number`)})

		// Since the payload was invalid, the provider should not flag any fresh data
		_, _, _, _, err := p.GetPower()
		if err == nil {
			t.Error("expected error because no valid fresh data was parsed, but got nil")
		}
	})

	t.Run("initial state returns error without crashing", func(t *testing.T) {
		p := &MqttProvider{}

		// Call GetPower before ANY message has been received
		_, _, _, _, err := p.GetPower()
		if err == nil {
			t.Error("expected stale/no data error, but got nil")
		}
	})
}

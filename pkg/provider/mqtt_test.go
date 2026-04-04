package provider

import (
	"fmt"
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

func TestMqttProvider_onMessage(t *testing.T) {
	t.Run("with json path", func(t *testing.T) {
		p := &MqttProvider{
			jsonPath: "ENERGY.Power",
		}

		payload := `{"ENERGY": {"Power": 123.45}}`
		msg := &mockMessage{payload: []byte(payload)}
		p.onMessage(nil, msg)

		a, _, _, total, err := p.GetPower()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if a != 123.45 || total != 123.45 {
			t.Errorf("expected 123.45, got %v", a)
		}
	})

	t.Run("without json path", func(t *testing.T) {
		p := &MqttProvider{}

		payload := `456.78`
		msg := &mockMessage{payload: []byte(payload)}
		p.onMessage(nil, msg)

		a, _, _, total, err := p.GetPower()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if a != 456.78 || total != 456.78 {
			t.Errorf("expected 456.78, got %v", a)
		}
	})

	t.Run("invalid payload with json path", func(t *testing.T) {
		p := &MqttProvider{
			jsonPath: "ENERGY.Power",
		}

		payload := `{"ENERGY": {"Power": "invalid"}}`
		msg := &mockMessage{payload: []byte(payload)}
		p.onMessage(nil, msg)

		// It should not update the value if it's invalid
		pA, _, _, _, _ := p.GetPower()
		if pA != 0 {
			t.Errorf("expected 0 because no valid value should have been received, got %f", pA)
		}
	})

	t.Run("read and clear behavior", func(t *testing.T) {
		p := &MqttProvider{}
		payload := `100.0`
		msg := &mockMessage{payload: []byte(payload)}
		p.onMessage(nil, msg)

		// First call should return the value
		val, _, _, _, err := p.GetPower()
		if err != nil {
			t.Fatalf("first call: expected no error, got %v", err)
		}
		if val != 100.0 {
			t.Errorf("first call: expected 100.0, got %v", val)
		}

		// Second call should return 0 (Read-and-Clear)
		val2, _, _, _, err := p.GetPower()
		if err != nil {
			t.Fatalf("second call: expected no error, got %v", err)
		}
		if val2 != 0 {
			t.Errorf("second call: expected 0.0, got %v", val2)
		}
	})

	t.Run("error clears value", func(t *testing.T) {
		p := &MqttProvider{}
		payload := `100.0`
		msg := &mockMessage{payload: []byte(payload)}
		p.onMessage(nil, msg)

		p.setError(fmt.Errorf("mqtt error"))

		_, _, _, total, err := p.GetPower()
		if err == nil {
			t.Error("expected error, got nil")
		}
		if total != 0 {
			t.Errorf("expected 0 on error, got %f", total)
		}

		// Second call should have no error and 0
		_, _, _, total, err = p.GetPower()
		if err != nil {
			t.Errorf("second call: expected no error, got %v", err)
		}
		if total != 0 {
			t.Errorf("second call: expected 0, got %f", total)
		}
	})
}

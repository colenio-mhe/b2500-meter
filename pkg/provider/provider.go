// Package provider defines the logic for retrieving power data from various sources.
package provider

import "sync"

// PowerProvider is the interface that all power data sources must implement.
// It returns the current power consumption (in Watts) for three phases and the total sum.
type PowerProvider interface {
	GetPower() (phaseA, phaseB, phaseC, total float64, err error)
}

// MockProvider is a simple provider that returns a fixed power value for phase A and total.
// This is useful for testing or providing a static reading.
type MockProvider struct {
	mu     sync.RWMutex
	phaseA float64
}

// NewMockProvider creates a MockProvider with a starting power value.
func NewMockProvider(phaseA float64) *MockProvider {
	return &MockProvider{phaseA: phaseA}
}

// GetPower returns the mocked power values.
func (m *MockProvider) GetPower() (float64, float64, float64, float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.phaseA, 0, 0, m.phaseA, nil
}

// SetPower updates the mocked phase A power value.
func (m *MockProvider) SetPower(phaseA float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.phaseA = phaseA
}

// MultiProvider aggregates multiple PowerProviders by summing their readings.
// This allows combining data from several meters into a single output.
type MultiProvider struct {
	providers []PowerProvider
}

// NewMultiProvider initializes a MultiProvider with the given slice of providers.
func NewMultiProvider(providers []PowerProvider) *MultiProvider {
	return &MultiProvider{providers: providers}
}

// GetPower iterates through all sub-providers and returns the total sum for each phase.
func (m *MultiProvider) GetPower() (phaseA, phaseB, phaseC, total float64, err error) {
	for _, p := range m.providers {
		a, b, c, t, pErr := p.GetPower()
		if pErr != nil {
			err = pErr
			continue
		}
		phaseA += a
		phaseB += b
		phaseC += c
		total += t
	}
	return
}

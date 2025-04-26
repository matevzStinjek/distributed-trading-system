package aggregator

import (
	"sync"
	"time"
)

// MockTicker is a mock implementation of time.Ticker
type MockTicker struct {
	C      chan time.Time
	mutex  sync.Mutex
	ticked bool
}

// NewMockTicker creates a new MockTicker
func NewMockTicker() *MockTicker {
	return &MockTicker{
		C:      make(chan time.Time, 1),
		ticked: false,
	}
}

// Tick triggers a tick event on the ticker
func (m *MockTicker) Tick() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Only send a tick if the channel is empty to avoid blocking
	select {
	case m.C <- time.Now():
		m.ticked = true
	default:
		// Channel is full, do nothing
	}
}

// Stop implements the Stop method of time.Ticker
func (m *MockTicker) Stop() {
	// Closing the channel would be unsafe as it could be read from after closing
	// Instead, we'll rely on the context cancellation in real usage
}

// HasTicked returns whether the ticker has ticked
func (m *MockTicker) HasTicked() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.ticked
}

// MockTickerFactory is a factory for creating new mock tickers
type MockTickerFactory struct {
	mutex   sync.Mutex
	tickers []*MockTicker
}

// NewMockTickerFactory creates a new MockTickerFactory
func NewMockTickerFactory() *MockTickerFactory {
	return &MockTickerFactory{
		tickers: make([]*MockTicker, 0),
	}
}

// NewTicker creates a new mock ticker and registers it with the factory
func (f *MockTickerFactory) NewTicker(d time.Duration) *time.Ticker {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	mockTicker := NewMockTicker()
	f.tickers = append(f.tickers, mockTicker)

	// Create a time.Ticker struct with the mock ticker's channel
	return &time.Ticker{
		C: mockTicker.C,
	}
}

// Tick triggers a tick event on all registered tickers
func (f *MockTickerFactory) TickAll() {
	f.mutex.Lock()
	tickers := make([]*MockTicker, len(f.tickers)) // Copy to avoid holding lock during Tick()
	copy(tickers, f.tickers)
	f.mutex.Unlock()

	for _, ticker := range tickers {
		ticker.Tick()
	}
}

// GetTicker returns the most recently created ticker
func (f *MockTickerFactory) GetLastTicker() *MockTicker {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.tickers) == 0 {
		return nil
	}
	return f.tickers[len(f.tickers)-1]
}

package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

// MockMarketDataClient implements the MarketDataClient interface for testing
type MockMarketDataClient struct {
	mu               sync.Mutex
	subscribed       bool
	symbols          []string
	tradeChan        chan<- marketdata.Trade
	closed           bool
	tradesToGenerate []marketdata.Trade
	err              error
}

func NewMockMarketDataClient() *MockMarketDataClient {
	return &MockMarketDataClient{}
}

func (m *MockMarketDataClient) SetError(err error) *MockMarketDataClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
	return m
}

func (m *MockMarketDataClient) SetTrades(trades []marketdata.Trade) *MockMarketDataClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tradesToGenerate = trades
	return m
}

func (m *MockMarketDataClient) SubscribeToSymbols(ctx context.Context, tradeChan chan<- marketdata.Trade, symbols []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	m.subscribed = true
	m.symbols = symbols
	m.tradeChan = tradeChan

	// Generate trades in background if any were set
	if len(m.tradesToGenerate) > 0 {
		go func() {
			for _, trade := range m.tradesToGenerate {
				select {
				case <-ctx.Done():
					return
				case tradeChan <- trade:
					// Trade sent successfully
				}
			}
		}()
	}

	return nil
}

func (m *MockMarketDataClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *MockMarketDataClient) IsSubscribed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.subscribed
}

func (m *MockMarketDataClient) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func (m *MockMarketDataClient) GetSymbols() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.symbols
}

// MockCacheClient implements the CacheClient interface for testing
type MockCacheClient struct {
	mu      sync.Mutex
	storage map[string]interface{}
	closed  bool
	err     error
}

func NewMockCacheClient() *MockCacheClient {
	return &MockCacheClient{
		storage: make(map[string]interface{}),
	}
}

func (m *MockCacheClient) SetError(err error) *MockCacheClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
	return m
}

func (m *MockCacheClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	m.storage[key] = value
	return nil
}

func (m *MockCacheClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *MockCacheClient) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func (m *MockCacheClient) GetValue(key string) (interface{}, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val, exists := m.storage[key]
	return val, exists
}

// MockPubsubClient implements the PubsubClient interface for testing
type MockPubsubClient struct {
	mu              sync.Mutex
	publishedTopics map[string][]interface{}
	closed          bool
	err             error
}

func NewMockPubsubClient() *MockPubsubClient {
	return &MockPubsubClient{
		publishedTopics: make(map[string][]interface{}),
	}
}

func (m *MockPubsubClient) SetError(err error) *MockPubsubClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
	return m
}

func (m *MockPubsubClient) Publish(ctx context.Context, topic string, message interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	if _, exists := m.publishedTopics[topic]; !exists {
		m.publishedTopics[topic] = make([]interface{}, 0)
	}

	m.publishedTopics[topic] = append(m.publishedTopics[topic], message)
	return nil
}

func (m *MockPubsubClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *MockPubsubClient) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func (m *MockPubsubClient) GetPublished(topic string) []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.publishedTopics[topic]
}

// MockKafkaProducer implements the KafkaProducer interface for testing
type MockKafkaProducer struct {
	mu       sync.Mutex
	messages []marketdata.Trade
	closed   bool
	err      error
}

func NewMockKafkaProducer() *MockKafkaProducer {
	return &MockKafkaProducer{
		messages: make([]marketdata.Trade, 0),
	}
}

func (m *MockKafkaProducer) SetError(err error) *MockKafkaProducer {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
	return m
}

func (m *MockKafkaProducer) Produce(ctx context.Context, trade marketdata.Trade) (partition int32, offset int64, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return 0, 0, m.err
	}

	m.messages = append(m.messages, trade)
	return 0, int64(len(m.messages) - 1), nil
}

func (m *MockKafkaProducer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *MockKafkaProducer) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func (m *MockKafkaProducer) GetProducedMessages() []marketdata.Trade {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.messages
}

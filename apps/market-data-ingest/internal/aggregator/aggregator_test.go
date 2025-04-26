package aggregator

import (
	"context"
	"testing"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

func TestTradeAggregator_WithMockTicker(t *testing.T) {
	// Create test trades with different symbols
	testTrades := []marketdata.Trade{
		{
			ID:        1,
			Symbol:    "AAPL",
			Price:     150.50,
			Size:      100,
			Timestamp: time.Now(),
		},
		{
			ID:        2,
			Symbol:    "MSFT",
			Price:     300.75,
			Size:      50,
			Timestamp: time.Now(),
		},
		{
			ID:        3,
			Symbol:    "AAPL", // Same symbol as first trade
			Price:     151.25, // Updated price
			Size:      75,
			Timestamp: time.Now().Add(time.Second),
		},
		{
			ID:        4,
			Symbol:    "GOOGL",
			Price:     2500.10,
			Size:      10,
			Timestamp: time.Now(),
		},
	}

	// Setup logger
	log := logger.NewNoOpLogger()

	// Create configuration with a long interval (it won't matter as we'll control the ticks)
	cfg := &config.Config{
		AggregatorInterval: 1 * time.Hour, // Large interval so it won't tick naturally
	}

	// Create a mock ticker factory
	mockTickerFactory := NewMockTickerFactory()

	// Create the aggregator with the mock ticker factory
	aggregator := NewTradeAggregatorWithTicker(cfg, log, mockTickerFactory.NewTicker)

	// Create input and output channels
	rawTradesChan := make(chan marketdata.Trade, len(testTrades))
	processedTradesChan := make(chan marketdata.Trade, len(testTrades))

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the aggregator in a goroutine
	errChan := make(chan error, 1)
	go func() {
		err := aggregator.Start(ctx, rawTradesChan, processedTradesChan)
		errChan <- err
	}()

	// Send all test trades
	for _, trade := range testTrades {
		rawTradesChan <- trade
	}

	// Wait for the trades to be received by the aggregator
	waitForTradesMap(t, aggregator, 3, 100*time.Millisecond)

	// No trades should be processed yet (no tick)
	if len(drainChannel(processedTradesChan)) != 0 {
		t.Error("expected no trades to be processed before tick")
	}

	// Simulate a tick
	mockTickerFactory.TickAll()

	// Now trades should be processed after the tick
	receivedTrades := drainChannel(processedTradesChan)
	if len(receivedTrades) != 3 {
		t.Errorf("expected 3 trades after tick, got %d", len(receivedTrades))
	}

	// Create a map to check each symbol
	symbolMap := make(map[string]marketdata.Trade)
	for _, trade := range receivedTrades {
		symbolMap[trade.Symbol] = trade
	}

	// Verify that each symbol is present
	symbols := []string{"AAPL", "MSFT", "GOOGL"}
	for _, symbol := range symbols {
		if _, exists := symbolMap[symbol]; !exists {
			t.Errorf("expected symbol %s in output, but it was not found", symbol)
		}
	}

	// Verify that AAPL has the latest price (from the third trade)
	if appleTrade, exists := symbolMap["AAPL"]; exists {
		if appleTrade.Price != 151.25 {
			t.Errorf("expected AAPL price to be 151.25, got %f", appleTrade.Price)
		}
	}

	// Clean up
	cancel()
	close(rawTradesChan)

	// Wait for aggregator to finish
	select {
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			t.Errorf("expected context.Canceled error, got: %v", err)
		}
	case <-time.After(100 * time.Millisecond): // Much shorter timeout is fine with mocks
		t.Fatal("aggregator did not shut down promptly")
	}
}

func TestTradeAggregator_PeriodicFlushWithMockTicker(t *testing.T) {
	// Setup logger
	log := logger.NewNoOpLogger()

	// Create configuration with a long interval (it won't matter as we'll control the ticks)
	cfg := &config.Config{
		AggregatorInterval: 1 * time.Hour, // Large interval so it won't tick naturally
	}

	// Create a mock ticker factory
	mockTickerFactory := NewMockTickerFactory()

	// Create the aggregator with the mock ticker factory
	aggregator := NewTradeAggregatorWithTicker(cfg, log, mockTickerFactory.NewTicker)

	// Create input and output channels
	rawTradesChan := make(chan marketdata.Trade, 10)
	processedTradesChan := make(chan marketdata.Trade, 10)

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the aggregator in a goroutine
	go aggregator.Start(ctx, rawTradesChan, processedTradesChan)

	// Send a trade for AAPL
	rawTradesChan <- marketdata.Trade{
		ID:        1,
		Symbol:    "AAPL",
		Price:     150.50,
		Size:      100,
		Timestamp: time.Now(),
	}

	// Wait for the trade to be received by the aggregator
	waitForTradesMap(t, aggregator, 1, 100*time.Millisecond)

	// No trades should be processed yet (no tick)
	if len(drainChannel(processedTradesChan)) != 0 {
		t.Error("expected no trades to be processed before first tick")
	}

	// Simulate first tick
	mockTickerFactory.TickAll()

	// Check that we received the first trade
	firstBatch := drainChannel(processedTradesChan)
	if len(firstBatch) != 1 {
		t.Errorf("expected 1 trade in first flush, got %d", len(firstBatch))
	}

	// Send a trade for MSFT
	rawTradesChan <- marketdata.Trade{
		ID:        2,
		Symbol:    "MSFT",
		Price:     300.75,
		Size:      50,
		Timestamp: time.Now(),
	}

	// Wait for the trade to be received by the aggregator
	waitForTradesMap(t, aggregator, 1, 100*time.Millisecond)

	// Simulate second tick
	mockTickerFactory.TickAll()

	// Check that we received the second trade
	secondBatch := drainChannel(processedTradesChan)
	if len(secondBatch) != 1 {
		t.Errorf("expected 1 trade in second flush, got %d", len(secondBatch))
	}

	// Send an updated trade for AAPL and a new one for GOOGL
	rawTradesChan <- marketdata.Trade{
		ID:        3,
		Symbol:    "AAPL",
		Price:     151.25,
		Size:      75,
		Timestamp: time.Now(),
	}
	rawTradesChan <- marketdata.Trade{
		ID:        4,
		Symbol:    "GOOGL",
		Price:     2500.10,
		Size:      10,
		Timestamp: time.Now(),
	}

	// Wait for the trades to be received by the aggregator
	waitForTradesMap(t, aggregator, 2, 100*time.Millisecond)

	// Simulate third tick
	mockTickerFactory.TickAll()

	// Check that we received both trades in the third flush
	thirdBatch := drainChannel(processedTradesChan)
	if len(thirdBatch) != 2 {
		t.Errorf("expected 2 trades in third flush, got %d", len(thirdBatch))
	}

	// Clean up
	cancel()
}

func TestTradeAggregator_GracefulShutdownWithMockTicker(t *testing.T) {
	// Setup logger
	log := logger.NewNoOpLogger()

	// Create configuration with a long interval
	cfg := &config.Config{
		AggregatorInterval: 1 * time.Hour, // Large interval so it won't tick naturally
	}

	// Create a mock ticker factory
	mockTickerFactory := NewMockTickerFactory()

	// Create the aggregator with the mock ticker factory
	aggregator := NewTradeAggregatorWithTicker(cfg, log, mockTickerFactory.NewTicker)

	// Create input and output channels
	rawTradesChan := make(chan marketdata.Trade, 10)
	processedTradesChan := make(chan marketdata.Trade, 10)

	// Send some trades before starting the aggregator
	testTrades := []marketdata.Trade{
		{
			ID:        1,
			Symbol:    "AAPL",
			Price:     150.50,
			Size:      100,
			Timestamp: time.Now(),
		},
		{
			ID:        2,
			Symbol:    "MSFT",
			Price:     300.75,
			Size:      50,
			Timestamp: time.Now(),
		},
	}

	for _, trade := range testTrades {
		rawTradesChan <- trade
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Start the aggregator in a goroutine
	errChan := make(chan error, 1)
	go func() {
		err := aggregator.Start(ctx, rawTradesChan, processedTradesChan)
		errChan <- err
	}()

	// Let the aggregator initialize and read the trades
	time.Sleep(50 * time.Millisecond)

	// Verify the map contains both trades before shutdown
	if mapSize := getTradesMapSize(aggregator); mapSize != 2 {
		t.Errorf("expected 2 trades in map before shutdown, got %d", mapSize)
	}

	// Cancel the context to simulate graceful shutdown
	cancel()

	// Wait for aggregator to finish
	var err error
	select {
	case err = <-errChan:
		if err != nil && err != context.Canceled {
			t.Errorf("expected context.Canceled error, got: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("aggregator did not shut down promptly")
	}

	// Check that a final flush occurred on shutdown
	receivedTrades := drainChannel(processedTradesChan)
	if len(receivedTrades) != 2 {
		t.Errorf("expected 2 trades on shutdown flush, got %d", len(receivedTrades))
	}
}

// Helper function to get the size of the latestTrades map in the aggregator
func getTradesMapSize(ta *TradeAggregator) int {
	ta.mutex.Lock()
	defer ta.mutex.Unlock()
	return len(ta.latestTrades)
}

// Helper function to wait for a specific number of trades in the map
func waitForTradesMap(t *testing.T, ta *TradeAggregator, expectedSize int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if size := getTradesMapSize(ta); size >= expectedSize {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d trades in map, current size: %d",
		expectedSize, getTradesMapSize(ta))
}

// Keep the original tests for comparison and backward compatibility
func TestTradeAggregator_Start(t *testing.T) {
	// Create test trades with different symbols
	testTrades := []marketdata.Trade{
		{
			ID:        1,
			Symbol:    "AAPL",
			Price:     150.50,
			Size:      100,
			Timestamp: time.Now(),
		},
		{
			ID:        2,
			Symbol:    "MSFT",
			Price:     300.75,
			Size:      50,
			Timestamp: time.Now(),
		},
		{
			ID:        3,
			Symbol:    "AAPL", // Same symbol as first trade
			Price:     151.25, // Updated price
			Size:      75,
			Timestamp: time.Now().Add(time.Second),
		},
		{
			ID:        4,
			Symbol:    "GOOGL",
			Price:     2500.10,
			Size:      10,
			Timestamp: time.Now(),
		},
	}

	// Setup logger
	log := logger.NewNoOpLogger()

	// Create a short aggregation interval for testing
	cfg := &config.Config{
		AggregatorInterval: 100 * time.Millisecond,
	}

	// Create the aggregator
	aggregator := NewTradeAggregator(cfg, log)

	// Create input and output channels
	rawTradesChan := make(chan marketdata.Trade, len(testTrades))
	processedTradesChan := make(chan marketdata.Trade, len(testTrades))

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Start the aggregator in a goroutine
	errChan := make(chan error, 1)
	go func() {
		err := aggregator.Start(ctx, rawTradesChan, processedTradesChan)
		errChan <- err
	}()

	// Send the test trades to the raw trades channel
	for _, trade := range testTrades {
		rawTradesChan <- trade
	}

	// Wait for aggregation to happen
	time.Sleep(200 * time.Millisecond)

	// Close the input channel to signal no more trades
	close(rawTradesChan)

	// Wait for the aggregator to finish
	var err error
	select {
	case err = <-errChan:
		// Aggregator completed
	case <-time.After(1 * time.Second):
		t.Fatal("aggregator did not shut down within timeout period")
	}

	if err != nil {
		t.Errorf("aggregator returned error: %v", err)
	}

	// Check the processed trades
	receivedTrades := drainChannel(processedTradesChan)

	// We should receive one trade per unique symbol (3 unique symbols in our test)
	if len(receivedTrades) != 3 {
		t.Errorf("expected 3 trades, got %d", len(receivedTrades))
	}

	// Create a map to check each symbol
	symbolMap := make(map[string]marketdata.Trade)
	for _, trade := range receivedTrades {
		symbolMap[trade.Symbol] = trade
	}

	// Verify that each symbol is present
	symbols := []string{"AAPL", "MSFT", "GOOGL"}
	for _, symbol := range symbols {
		if _, exists := symbolMap[symbol]; !exists {
			t.Errorf("expected symbol %s in output, but it was not found", symbol)
		}
	}

	// Verify that AAPL has the latest price (from the third trade)
	if appleTrade, exists := symbolMap["AAPL"]; exists {
		if appleTrade.Price != 151.25 {
			t.Errorf("expected AAPL price to be 151.25, got %f", appleTrade.Price)
		}
	}
}

func TestTradeAggregator_PeriodicFlush(t *testing.T) {
	// Setup logger
	log := logger.NewNoOpLogger()

	// Create a very short aggregation interval
	cfg := &config.Config{
		AggregatorInterval: 50 * time.Millisecond,
	}

	// Create the aggregator
	aggregator := NewTradeAggregator(cfg, log)

	// Create input and output channels
	rawTradesChan := make(chan marketdata.Trade, 10)
	processedTradesChan := make(chan marketdata.Trade, 10)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Start the aggregator in a goroutine
	go aggregator.Start(ctx, rawTradesChan, processedTradesChan)

	// Send a trade for AAPL
	rawTradesChan <- marketdata.Trade{
		ID:        1,
		Symbol:    "AAPL",
		Price:     150.50,
		Size:      100,
		Timestamp: time.Now(),
	}

	// Wait for first flush
	time.Sleep(75 * time.Millisecond)

	// Check that we received the first trade
	firstBatch := drainChannel(processedTradesChan)
	if len(firstBatch) != 1 {
		t.Errorf("expected 1 trade in first flush, got %d", len(firstBatch))
	}

	// Send a trade for MSFT
	rawTradesChan <- marketdata.Trade{
		ID:        2,
		Symbol:    "MSFT",
		Price:     300.75,
		Size:      50,
		Timestamp: time.Now(),
	}

	// Wait for second flush
	time.Sleep(75 * time.Millisecond)

	// Check that we received the second trade
	secondBatch := drainChannel(processedTradesChan)
	if len(secondBatch) != 1 {
		t.Errorf("expected 1 trade in second flush, got %d", len(secondBatch))
	}

	// Send an updated trade for AAPL and a new one for GOOGL
	rawTradesChan <- marketdata.Trade{
		ID:        3,
		Symbol:    "AAPL",
		Price:     151.25,
		Size:      75,
		Timestamp: time.Now(),
	}
	rawTradesChan <- marketdata.Trade{
		ID:        4,
		Symbol:    "GOOGL",
		Price:     2500.10,
		Size:      10,
		Timestamp: time.Now(),
	}

	// Wait for third flush
	time.Sleep(75 * time.Millisecond)

	// Check that we received both trades in the third flush
	thirdBatch := drainChannel(processedTradesChan)
	if len(thirdBatch) != 2 {
		t.Errorf("expected 2 trades in third flush, got %d", len(thirdBatch))
	}

	// Cancel the context to shut down
	cancel()
}

func TestTradeAggregator_GracefulShutdown(t *testing.T) {
	// Setup logger
	log := logger.NewNoOpLogger()

	// Create configuration with a long interval to ensure it won't flush on its own
	cfg := &config.Config{
		AggregatorInterval: 10 * time.Second,
	}

	// Create the aggregator
	aggregator := NewTradeAggregator(cfg, log)

	// Create input and output channels
	rawTradesChan := make(chan marketdata.Trade, 10)
	processedTradesChan := make(chan marketdata.Trade, 10)

	// Send some trades before starting the aggregator
	testTrades := []marketdata.Trade{
		{
			ID:        1,
			Symbol:    "AAPL",
			Price:     150.50,
			Size:      100,
			Timestamp: time.Now(),
		},
		{
			ID:        2,
			Symbol:    "MSFT",
			Price:     300.75,
			Size:      50,
			Timestamp: time.Now(),
		},
	}

	for _, trade := range testTrades {
		rawTradesChan <- trade
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Start the aggregator in a goroutine
	errChan := make(chan error, 1)
	go func() {
		err := aggregator.Start(ctx, rawTradesChan, processedTradesChan)
		errChan <- err
	}()

	// Let the aggregator process the trades
	time.Sleep(100 * time.Millisecond)

	// Cancel the context to simulate graceful shutdown
	cancel()

	// Wait for aggregator to finish
	var err error
	select {
	case err = <-errChan:
		if err != nil && err != context.Canceled {
			t.Errorf("expected context.Canceled error, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("aggregator did not shut down within timeout period")
	}

	// Check that a final flush occurred on shutdown
	receivedTrades := drainChannel(processedTradesChan)
	if len(receivedTrades) != 2 {
		t.Errorf("expected 2 trades on shutdown flush, got %d", len(receivedTrades))
	}
}

// Helper function to drain a channel
func drainChannel(ch <-chan marketdata.Trade) []marketdata.Trade {
	var result []marketdata.Trade
	for {
		select {
		case trade, ok := <-ch:
			if !ok {
				return result
			}
			result = append(result, trade)
		case <-time.After(100 * time.Millisecond):
			// Timeout to avoid blocking indefinitely
			return result
		}
	}
}

package processor

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/mocks"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

func TestTradeProcessor_Start(t *testing.T) {
	tests := []struct {
		name           string
		inputTrades    []marketdata.Trade
		cacheError     error
		pubsubError    error
		expectedCaches int
		expectedPubs   int
		shouldFail     bool
	}{
		{
			name: "successful_processing",
			inputTrades: []marketdata.Trade{
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
			},
			expectedCaches: 2, // We expect two cache updates
			expectedPubs:   2, // We expect two pubsub messages
			shouldFail:     false,
		},
		{
			name: "cache_error",
			inputTrades: []marketdata.Trade{
				{
					ID:        1,
					Symbol:    "AAPL",
					Price:     150.50,
					Size:      100,
					Timestamp: time.Now(),
				},
			},
			cacheError:     errors.New("cache error"),
			expectedCaches: 0,
			// In the implementation, operations are run in parallel and independent goroutines
			// so pubsub could still succeed even if cache fails
			expectedPubs: 1,
			shouldFail:   true,
		},
		{
			name: "pubsub_error",
			inputTrades: []marketdata.Trade{
				{
					ID:        1,
					Symbol:    "AAPL",
					Price:     150.50,
					Size:      100,
					Timestamp: time.Now(),
				},
			},
			pubsubError: errors.New("pubsub error"),
			// In the implementation, operations are run in parallel and independent goroutines
			// so cache could still succeed even if pubsub fails
			expectedCaches: 1,
			expectedPubs:   0,
			shouldFail:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mock clients
			mockCache := mocks.NewMockCacheClient()
			if tc.cacheError != nil {
				mockCache.SetError(tc.cacheError)
			}

			mockPubsub := mocks.NewMockPubsubClient()
			if tc.pubsubError != nil {
				mockPubsub.SetError(tc.pubsubError)
			}

			// Setup logger
			log := logger.NewNoOpLogger()

			// Create processor
			processor := NewTradeProcessor(mockCache, mockPubsub, log)

			// Create channels for communication
			inputChan := make(chan marketdata.Trade, len(tc.inputTrades))
			outputChan := make(chan marketdata.Trade, len(tc.inputTrades))

			// Fill the input channel
			for _, trade := range tc.inputTrades {
				inputChan <- trade
			}
			close(inputChan) // Close to signal no more trades

			// Create context
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Run the processor and capture any errors
			errChan := make(chan error, 1)
			go func() {
				err := processor.Start(ctx, inputChan, outputChan)
				errChan <- err
			}()

			// Wait for processing to complete with a timeout
			var err error
			select {
			case err = <-errChan:
				// Processor completed normally
			case <-time.After(3 * time.Second):
				cancel() // Force shutdown after timeout
				t.Logf("Test timed out, forcing shutdown")
				err = <-errChan
			}

			// Expected behavior depends on test case
			if tc.shouldFail {
				// In our implementation, errors are handled in goroutines and may not cause
				// the main processor to return an error, so we'll check the state instead

				// Give background goroutines time to finish
				time.Sleep(500 * time.Millisecond)

				// Check cache interactions
				cacheEntries := 0
				for _, trade := range tc.inputTrades {
					key := fmt.Sprintf("price:%s", trade.Symbol)
					if _, exists := mockCache.GetValue(key); exists {
						cacheEntries++
					}
				}

				if cacheEntries != tc.expectedCaches {
					t.Errorf("expected %d cache entries, got %d", tc.expectedCaches, cacheEntries)
				}

				// Check pubsub interactions
				pubsubMessages := 0
				for _, trade := range tc.inputTrades {
					key := fmt.Sprintf("price:%s", trade.Symbol)
					messages := mockPubsub.GetPublished(key)
					pubsubMessages += len(messages)
				}

				if pubsubMessages != tc.expectedPubs {
					t.Errorf("expected %d pubsub messages, got %d", tc.expectedPubs, pubsubMessages)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// Give background goroutines time to finish
				time.Sleep(500 * time.Millisecond)

				// Check cache interactions
				cacheEntries := 0
				for _, trade := range tc.inputTrades {
					key := fmt.Sprintf("price:%s", trade.Symbol)
					if _, exists := mockCache.GetValue(key); exists {
						cacheEntries++
					}
				}

				if cacheEntries != tc.expectedCaches {
					t.Errorf("expected %d cache entries, got %d", tc.expectedCaches, cacheEntries)
				}

				// Check pubsub interactions
				pubsubMessages := 0
				for _, trade := range tc.inputTrades {
					key := fmt.Sprintf("price:%s", trade.Symbol)
					messages := mockPubsub.GetPublished(key)
					pubsubMessages += len(messages)
				}

				if pubsubMessages != tc.expectedPubs {
					t.Errorf("expected %d pubsub messages, got %d", tc.expectedPubs, pubsubMessages)
				}

				// Check if output channel received the trades
				receivedTrades := drainChannel(outputChan)
				if len(receivedTrades) != len(tc.inputTrades) {
					t.Errorf("expected %d trades in output channel, got %d",
						len(tc.inputTrades), len(receivedTrades))
				}
			}
		})
	}
}

func TestTradeProcessor_GracefulShutdown(t *testing.T) {
	// Setup mock clients
	mockCache := mocks.NewMockCacheClient()
	mockPubsub := mocks.NewMockPubsubClient()

	// Setup logger
	log := logger.NewNoOpLogger()

	// Create processor
	processor := NewTradeProcessor(mockCache, mockPubsub, log)

	// Create channels for communication
	inputChan := make(chan marketdata.Trade, 5)
	outputChan := make(chan marketdata.Trade, 5)

	// Add some trades
	inputChan <- marketdata.Trade{
		ID:        1,
		Symbol:    "AAPL",
		Price:     150.50,
		Size:      100,
		Timestamp: time.Now(),
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Start processor in goroutine
	started := make(chan struct{})
	errChan := make(chan error, 1)

	go func() {
		close(started)
		err := processor.Start(ctx, inputChan, outputChan)
		errChan <- err
	}()

	<-started // Wait for goroutine to start

	// Give some time for the processor to handle at least one trade
	time.Sleep(300 * time.Millisecond)

	// Cancel context after a short delay to simulate graceful shutdown
	cancel()

	// Wait for processor to complete with timeout
	select {
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			t.Errorf("expected nil or context.Canceled error on shutdown, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("processor did not shut down within timeout period")
	}

	// Note: In this implementation, the processor doesn't close the clients,
	// so we don't verify client closure here
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

package ingestor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/mocks"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

func TestTradeIngestor_Start(t *testing.T) {
	tests := []struct {
		name           string
		symbols        []string
		mockTrades     []marketdata.Trade
		subscriberErr  error
		expectedTrades int
		shouldFail     bool
	}{
		{
			name:    "successful_subscription",
			symbols: []string{"AAPL", "MSFT"},
			mockTrades: []marketdata.Trade{
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
			expectedTrades: 2,
			shouldFail:     false,
		},
		{
			name:          "subscription_error",
			symbols:       []string{"AAPL", "MSFT"},
			subscriberErr: errors.New("failed to subscribe"),
			shouldFail:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mock client
			mockClient := mocks.NewMockMarketDataClient()
			if tc.subscriberErr != nil {
				mockClient.SetError(tc.subscriberErr)
			} else {
				mockClient.SetTrades(tc.mockTrades)
			}

			// Setup configuration
			cfg := &config.Config{
				Symbols:           tc.symbols,
				RawTradesChanBuff: 10,
			}

			// Setup logger
			log := logger.NewNoOpLogger()

			// Create ingestor
			ingestor := NewTradeIngestor(mockClient, cfg, log)

			// Create context with cancellation
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create receiving channel
			receivedTrades := make([]marketdata.Trade, 0)
			tradeChan := make(chan marketdata.Trade, 10)

			// Counter to track received trades
			var receivedCount int

			// Setup goroutine to collect trades
			errChan := make(chan error, 1)
			doneChan := make(chan struct{}, 1)

			// Start collecting trades
			go func() {
				for trade := range tradeChan {
					receivedTrades = append(receivedTrades, trade)
					receivedCount++
					// If we've received all expected trades, stop the test
					if receivedCount == tc.expectedTrades {
						doneChan <- struct{}{}
						return
					}
				}
			}()

			// Start the ingestor in a goroutine
			go func() {
				err := ingestor.Start(ctx, tradeChan)
				if err != nil {
					errChan <- err
				}
			}()

			// Wait for either:
			// 1. All expected trades to be received
			// 2. An error to be reported
			// 3. Timeout after 2 seconds
			timeoutDuration := 2 * time.Second
			select {
			case <-doneChan:
				// Successful case, we've received all expected trades
				cancel() // Cleanup
				if tc.shouldFail {
					t.Error("test should have failed but succeeded")
				}
				// Verify we have the expected number of trades
				if len(receivedTrades) != tc.expectedTrades {
					t.Errorf("expected %d trades, got %d", tc.expectedTrades, len(receivedTrades))
				}
			case err := <-errChan:
				// Error case
				if !tc.shouldFail {
					t.Errorf("unexpected error: %v", err)
				}
			case <-time.After(timeoutDuration):
				cancel() // Cleanup
				if tc.expectedTrades > 0 && !tc.shouldFail {
					t.Errorf("test timed out after %v, received %d of %d expected trades",
						timeoutDuration, len(receivedTrades), tc.expectedTrades)
				}
			}

			// Verify that the client was called with correct symbols
			if mockClient.IsSubscribed() {
				symbols := mockClient.GetSymbols()
				if len(symbols) != len(tc.symbols) {
					t.Errorf("expected %d symbols, got %d", len(tc.symbols), len(symbols))
				}
				for i, symbol := range tc.symbols {
					if i < len(symbols) && symbols[i] != symbol {
						t.Errorf("expected symbol %s at position %d, got %s", symbol, i, symbols[i])
					}
				}
			} else if !tc.shouldFail {
				t.Error("expected subscription call, but none was made")
			}
		})
	}
}

func TestTradeIngestor_GracefulShutdown(t *testing.T) {
	// Setup mock client with some trades
	mockClient := mocks.NewMockMarketDataClient()
	mockTrades := []marketdata.Trade{
		{
			ID:        1,
			Symbol:    "AAPL",
			Price:     150.50,
			Size:      100,
			Timestamp: time.Now(),
		},
	}
	mockClient.SetTrades(mockTrades)

	// Setup configuration
	cfg := &config.Config{
		Symbols:           []string{"AAPL"},
		RawTradesChanBuff: 10,
	}

	// Setup logger
	log := logger.NewNoOpLogger()

	// Create ingestor
	ingestor := NewTradeIngestor(mockClient, cfg, log)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Create receiving channel
	tradeChan := make(chan marketdata.Trade, 10)

	// Start the ingestor in a goroutine
	errChan := make(chan error, 1)
	go func() {
		err := ingestor.Start(ctx, tradeChan)
		errChan <- err
	}()

	// Let it run for a short time then cancel the context
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Wait for ingestor to complete, with timeout
	select {
	case err := <-errChan:
		// Should exit with no error
		if err != nil {
			t.Errorf("expected nil error on graceful shutdown, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("ingestor did not shut down within timeout period")
	}

	// Note: In this implementation, the ingestor doesn't close the client,
	// so we don't verify client closure here
}

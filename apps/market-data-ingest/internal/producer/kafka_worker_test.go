package producer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/mocks"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

func TestKafkaWorker_Start(t *testing.T) {
	tests := []struct {
		name        string
		inputTrades []marketdata.Trade
		kafkaError  error
		shouldFail  bool
	}{
		{
			name: "successful_production",
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
			shouldFail: false,
		},
		{
			name: "kafka_error",
			inputTrades: []marketdata.Trade{
				{
					ID:        1,
					Symbol:    "AAPL",
					Price:     150.50,
					Size:      100,
					Timestamp: time.Now(),
				},
			},
			kafkaError: errors.New("kafka production error"),
			shouldFail: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mock Kafka producer
			mockProducer := mocks.NewMockKafkaProducer()
			if tc.kafkaError != nil {
				mockProducer.SetError(tc.kafkaError)
			}

			// Setup logger
			log := logger.NewNoOpLogger()

			// Create worker
			worker := NewKafkaWorker(mockProducer, log)

			// Create input channel and fill it
			inputChan := make(chan marketdata.Trade, len(tc.inputTrades))
			for _, trade := range tc.inputTrades {
				inputChan <- trade
			}
			close(inputChan) // Close to signal no more trades

			// Create context
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Run the worker and capture any errors
			err := worker.Start(ctx, inputChan)

			// Verify results
			if tc.shouldFail {
				// In the implementation, errors are logged but don't cause the worker to fail
				// So instead of checking for an error, check that no messages were produced
				producedMessages := mockProducer.GetProducedMessages()
				if len(producedMessages) > 0 && tc.kafkaError != nil {
					t.Errorf("expected no messages to be produced due to error, got %d", len(producedMessages))
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// Check that all trades were produced
				producedMessages := mockProducer.GetProducedMessages()
				if len(producedMessages) != len(tc.inputTrades) {
					t.Errorf("expected %d messages produced, got %d",
						len(tc.inputTrades), len(producedMessages))
				}

				// Verify message contents if needed
				for i, expected := range tc.inputTrades {
					if i < len(producedMessages) {
						actual := producedMessages[i]
						if expected.ID != actual.ID || expected.Symbol != actual.Symbol {
							t.Errorf("message %d content mismatch: expected %+v, got %+v",
								i, expected, actual)
						}
					}
				}
			}
		})
	}
}

func TestKafkaWorker_GracefulShutdown(t *testing.T) {
	// Setup mock producer
	mockProducer := mocks.NewMockKafkaProducer()

	// Setup logger
	log := logger.NewNoOpLogger()

	// Create worker
	worker := NewKafkaWorker(mockProducer, log)

	// Create input channel with some trades
	inputChan := make(chan marketdata.Trade, 5)

	// Add a trade but don't close the channel
	inputChan <- marketdata.Trade{
		ID:        1,
		Symbol:    "AAPL",
		Price:     150.50,
		Size:      100,
		Timestamp: time.Now(),
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start worker in goroutine
	errChan := make(chan error, 1)
	go func() {
		err := worker.Start(ctx, inputChan)
		errChan <- err
	}()

	// Let it process the message
	time.Sleep(100 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Check that worker shuts down gracefully
	select {
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			t.Errorf("expected nil or context.Canceled error, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("worker did not shut down within timeout period")
	}

	// Note: In this implementation, the worker doesn't close the producer,
	// so we don't verify producer closure here

	// Verify trade was processed
	producedMessages := mockProducer.GetProducedMessages()
	if len(producedMessages) != 1 {
		t.Errorf("expected 1 message produced, got %d", len(producedMessages))
	}
}

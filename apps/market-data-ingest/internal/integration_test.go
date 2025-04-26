package internal

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/ingestor"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/mocks"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/processor"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/producer"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
	"golang.org/x/sync/errgroup"
)

func TestIntegrationPipeline(t *testing.T) {
	// Setup test configuration
	cfg := &config.Config{
		Symbols:            []string{"AAPL", "MSFT"},
		RawTradesChanBuff:  10,
		ProcTradesChanBuff: 10,
		KafkaChanBuff:      10,
	}

	// Setup mock clients
	mockMarketData := mocks.NewMockMarketDataClient()
	mockCache := mocks.NewMockCacheClient()
	mockPubsub := mocks.NewMockPubsubClient()
	mockKafka := mocks.NewMockKafkaProducer()

	// Setup logger
	log := logger.NewNoOpLogger()

	// Setup test trades
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
			Symbol:    "AAPL",
			Price:     151.00,
			Size:      75,
			Timestamp: time.Now().Add(time.Second),
		},
	}
	mockMarketData.SetTrades(testTrades)

	// Setup channels
	rawTradesChan := make(chan marketdata.Trade, cfg.RawTradesChanBuff)
	processedTradesChan := make(chan marketdata.Trade, cfg.ProcTradesChanBuff)
	kafkaTradesChan := make(chan marketdata.Trade, cfg.KafkaChanBuff)

	// Initialize components
	ingestorSvc := ingestor.NewTradeIngestor(mockMarketData, cfg, log)
	processorSvc := processor.NewTradeProcessor(mockCache, mockPubsub, log)
	kafkaWorkerSvc := producer.NewKafkaWorker(mockKafka, log)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start all components in parallel
	g, ctx := errgroup.WithContext(ctx)

	// Start Kafka worker
	g.Go(func() error {
		return kafkaWorkerSvc.Start(ctx, kafkaTradesChan)
	})

	// Start processor
	g.Go(func() error {
		return processorSvc.Start(ctx, processedTradesChan, kafkaTradesChan)
	})

	// Start ingestor
	g.Go(func() error {
		return ingestorSvc.Start(ctx, rawTradesChan)
	})

	// Create additional goroutine to simulate aggregator (which we're not testing here)
	// by forwarding trades from raw to processed
	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case trade, ok := <-rawTradesChan:
				if !ok {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case processedTradesChan <- trade:
					// Successfully forwarded
				}
			}
		}
	})

	// Wait a bit for processing, then cancel to shut down gracefully
	time.Sleep(1 * time.Second)
	cancel()

	// Wait for all components to stop
	if err := g.Wait(); err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify that all trades made it through the pipeline
	producedMessages := mockKafka.GetProducedMessages()
	if len(producedMessages) != len(testTrades) {
		t.Errorf("expected %d trades in Kafka, got %d", len(testTrades), len(producedMessages))
	}

	// Verify that Redis cache was updated
	cacheEntries := 0
	for _, symbol := range []string{"AAPL", "MSFT"} {
		key := fmt.Sprintf("price:%s", symbol)
		if _, exists := mockCache.GetValue(key); exists {
			cacheEntries++
		}
	}
	if cacheEntries != 2 {
		t.Errorf("expected 2 cache entries (one per symbol), got %d", cacheEntries)
	}

	// Verify that Redis pubsub was updated
	pubsubMessages := 0
	for _, symbol := range []string{"AAPL", "MSFT"} {
		topic := fmt.Sprintf("price:%s", symbol)
		messages := mockPubsub.GetPublished(topic)
		pubsubMessages += len(messages)
	}
	if pubsubMessages != len(testTrades) {
		t.Errorf("expected %d pubsub messages, got %d", len(testTrades), pubsubMessages)
	}
}

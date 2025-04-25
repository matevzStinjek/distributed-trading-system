package processor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/metrics"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/utils"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/interfaces"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
	"golang.org/x/sync/errgroup"
)

type TradeProcessor struct {
	cacheClient  interfaces.CacheClient
	pubsubClient interfaces.PubsubClient
	logger       *logger.Logger
	wg           *sync.WaitGroup
}

func NewTradeProcessor(
	cacheClient interfaces.CacheClient,
	pubsubClient interfaces.PubsubClient,
	log *logger.Logger,
) *TradeProcessor {
	return &TradeProcessor{
		cacheClient:  cacheClient,
		pubsubClient: pubsubClient,
		logger:       log,
		wg:           &sync.WaitGroup{},
	}
}

func (tp *TradeProcessor) Start(
	ctx context.Context,
	tradeChannel <-chan marketdata.Trade,
	bgTradesChan chan<- marketdata.Trade,
) error {
	tp.logger.Info("trade processor starting",
		logger.Int("kafka_channel_buffer", cap(bgTradesChan)),
		logger.Int("input_channel_buffer", cap(tradeChannel)))

	// Stats tracking
	var (
		processedCount  int
		successCount    int
		errorCount      int
		lastReportTime  = time.Now()
		reportingPeriod = 30 * time.Second
	)

	defer func() {
		tp.logger.Info("closing background trades channel")
		close(bgTradesChan)

		tp.logger.Info("processor final statistics",
			logger.Int("trades_processed", processedCount),
			logger.Int("successful", successCount),
			logger.Int("errors", errorCount),
			logger.Int("success_rate_pct", calculateSuccessRate(successCount, errorCount)))
	}()

	// Function to periodically log statistics
	reportStats := func() {
		if time.Since(lastReportTime) >= reportingPeriod {
			tp.logger.Info("processor statistics",
				logger.Int("trades_processed", processedCount),
				logger.Int("success_rate_pct", calculateSuccessRate(successCount, errorCount)),
				logger.Int("errors", errorCount),
				logger.Duration("period", reportingPeriod))
			lastReportTime = time.Now()
		}
	}

	for {
		select {
		case trade, ok := <-tradeChannel:
			if !ok {
				tp.logger.Info("trade channel closed, waiting for pending operations",
					logger.Int("pending_operations", tp.getPendingOperations()))

				tp.wg.Wait()
				tp.logger.Info("all trade processing completed")
				return nil
			}

			processedCount++
			tp.wg.Add(1)
			go func(trade marketdata.Trade) {
				defer tp.wg.Done()

				// Start timing the processing operation
				start := time.Now()
				timer := metrics.NewTimer(metrics.RedisOperationDuration)

				err := tp.processTrade(ctx, trade, bgTradesChan)

				timer.ObserveDuration()
				duration := time.Since(start)

				if err != nil {
					errorCount++
					tp.logger.Error("trade processing failed",
						logger.Error(err),
						logger.String("symbol", trade.Symbol),
						logger.Float64("price", trade.Price),
						logger.Duration("duration_ms", duration))
				} else {
					successCount++
					// Increment processed trades counter on success
					metrics.TradesProcessedTotal.Inc()

					tp.logger.Debug("trade processed successfully",
						logger.String("symbol", trade.Symbol),
						logger.Float64("price", trade.Price),
						logger.Duration("duration_ms", duration))
				}

				// Report stats periodically
				reportStats()
			}(trade)

		case <-ctx.Done():
			tp.logger.Info("processor received shutdown signal",
				logger.Int("pending_operations", tp.getPendingOperations()))

			tp.logger.Info("waiting for in-flight operations to complete")
			tp.wg.Wait()
			tp.logger.Info("processor shutdown complete")
			return nil
		}
	}
}

func (tp *TradeProcessor) processTrade(ctx context.Context, trade marketdata.Trade, bgTradesChan chan<- marketdata.Trade) error {
	// Time-bound the processing of each trade
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	key := fmt.Sprintf("price:%s", trade.Symbol)

	g, ctx := errgroup.WithContext(ctx)

	// First async operation: update cache with latest price
	g.Go(func() error {
		op := func(ctx context.Context) error {
			// Increment the Redis SET attempt counter
			metrics.RedisSetTotal.Inc()

			// Measure the Redis SET operation duration
			start := time.Now()
			setTimer := metrics.NewTimer(metrics.RedisOperationDuration)
			err := tp.cacheClient.Set(ctx, key, trade.Price, 0)
			setTimer.ObserveDuration()
			duration := time.Since(start)

			if err != nil {
				// Increment error counter on failure
				metrics.RedisSetErrorsTotal.Inc()
				tp.logger.Warn("redis SET operation failed",
					logger.Error(err),
					logger.String("key", key),
					logger.Float64("value", trade.Price),
					logger.Duration("duration_ms", duration))
			} else {
				tp.logger.Debug("redis SET successful",
					logger.String("key", key),
					logger.Float64("value", trade.Price),
					logger.Duration("duration_ms", duration))
			}

			return err
		}
		return utils.RetryWithDefault(ctx, op, tp.logger)
	})

	// Second async operation: publish price update to subscribers
	g.Go(func() error {
		op := func(ctx context.Context) error {
			// Increment the Redis PUBLISH attempt counter
			metrics.RedisPublishTotal.Inc()

			// Measure the Redis PUBLISH operation duration
			start := time.Now()
			pubTimer := metrics.NewTimer(metrics.RedisOperationDuration)
			err := tp.pubsubClient.Publish(ctx, key, trade.Price)
			pubTimer.ObserveDuration()
			duration := time.Since(start)

			if err != nil {
				// Increment error counter on failure
				metrics.RedisPublishErrorsTotal.Inc()
				tp.logger.Warn("redis PUBLISH operation failed",
					logger.Error(err),
					logger.String("channel", key),
					logger.Float64("value", trade.Price),
					logger.Duration("duration_ms", duration))
			} else {
				tp.logger.Debug("redis PUBLISH successful",
					logger.String("channel", key),
					logger.Float64("value", trade.Price),
					logger.Duration("duration_ms", duration))
			}

			return err
		}
		return utils.RetryWithDefault(ctx, op, tp.logger)
	})

	// Third action: send trade to Kafka channel (non-blocking)
	select {
	case bgTradesChan <- trade:
		tp.logger.Debug("trade forwarded to kafka channel",
			logger.String("symbol", trade.Symbol))
	default:
		tp.logger.Warn("kafka channel full, dropping trade",
			logger.String("symbol", trade.Symbol),
			logger.Float64("price", trade.Price))
	}

	// Wait for both Redis operations to complete
	if err := g.Wait(); err != nil {
		return fmt.Errorf("trade processing encountered an error: %w", err)
	}

	return nil
}

// Helper method to get the number of pending operations
func (tp *TradeProcessor) getPendingOperations() int {
	// This is an approximate way to get pending operations
	// It's not perfect because wg doesn't expose its counter
	// We could modify the struct to track this internally if needed
	wg := &sync.WaitGroup{}
	wg.Add(1)

	var pendingCount int
	go func() {
		pendingCount = 1 // Minimum is 1 (this goroutine)
		tp.wg.Wait()
		wg.Done()
	}()

	// Wait a tiny bit to see if the waitgroup resolves immediately
	timer := time.NewTimer(10 * time.Millisecond)
	select {
	case <-timer.C:
		// WaitGroup didn't resolve, meaning there are pending operations
		pendingCount = 2 // We don't know exact count, but > 0
	default:
		// The done channel was closed, meaning no pending ops
		pendingCount = 0
	}

	// Clean up
	if !timer.Stop() {
		<-timer.C
	}

	return pendingCount
}

// calculateSuccessRate returns the success rate as percentage
func calculateSuccessRate(success, failure int) int {
	total := success + failure
	if total == 0 {
		return 100 // No operations, consider 100% success
	}
	return (success * 100) / total
}

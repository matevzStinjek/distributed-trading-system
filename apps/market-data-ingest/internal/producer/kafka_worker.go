package producer

import (
	"context"
	"errors"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/metrics"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/interfaces"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type KafkaWorker struct {
	producer interfaces.KafkaProducer
	logger   *logger.Logger
}

func NewKafkaWorker(producer interfaces.KafkaProducer, log *logger.Logger) *KafkaWorker {
	return &KafkaWorker{
		producer: producer,
		logger:   log,
	}
}

func (kw *KafkaWorker) Start(ctx context.Context, tradeChan <-chan marketdata.Trade) error {
	kw.logger.Info("kafka worker starting")

	var (
		successCount    int
		errorCount      int
		lastReportTime  = time.Now()
		reportingPeriod = 30 * time.Second
	)

	// Periodically log statistics
	reportStats := func() {
		if time.Since(lastReportTime) >= reportingPeriod {
			kw.logger.Info("kafka worker statistics",
				logger.Int("messages_sent", successCount),
				logger.Int("messages_failed", errorCount),
				logger.Int("success_rate_pct", calculateSuccessRate(successCount, errorCount)),
				logger.Duration("period", reportingPeriod))
			lastReportTime = time.Now()
		}
	}

	for {
		select {
		case trade, ok := <-tradeChan:
			if !ok {
				kw.logger.Info("trade channel closed, stopping kafka worker",
					logger.Int("total_success", successCount),
					logger.Int("total_errors", errorCount),
					logger.Int("success_rate_pct", calculateSuccessRate(successCount, errorCount)))

				// Close producer gracefully
				if err := kw.producer.Close(); err != nil {
					kw.logger.Error("error closing kafka producer", logger.Error(err))
				}
				return nil
			}

			// Track Kafka operation attempt
			metrics.KafkaPublishTotal.Inc()

			// Time the Kafka produce operation
			start := time.Now()
			timer := metrics.NewTimer(metrics.KafkaOperationDuration)
			partition, offset, err := kw.producer.Produce(ctx, trade)
			timer.ObserveDuration()
			duration := time.Since(start)

			if err != nil {
				// Increment error counter on failure
				metrics.KafkaPublishErrorsTotal.Inc()
				errorCount++

				kw.logger.Warn("failed to produce message to kafka",
					logger.Error(err),
					logger.String("symbol", trade.Symbol),
					logger.Float64("price", trade.Price),
					logger.Duration("duration_ms", duration))

				// Report stats periodically, even if some operations fail
				reportStats()
				continue
			}

			// Message sent successfully
			successCount++
			kw.logger.Debug("message sent to kafka",
				logger.String("symbol", trade.Symbol),
				logger.Float64("price", trade.Price),
				logger.Int("partition", int(partition)),
				logger.Int64("offset", offset),
				logger.Duration("duration_ms", duration))

			// Report stats periodically
			reportStats()

		case <-ctx.Done():
			kw.logger.Info("kafka worker shutting down",
				logger.Int("total_messages", successCount+errorCount),
				logger.Int("success_rate_pct", calculateSuccessRate(successCount, errorCount)))

			// Close producer gracefully
			if err := kw.producer.Close(); err != nil {
				kw.logger.Error("error closing kafka producer on shutdown", logger.Error(err))
			}

			// Return nil for normal context cancellation
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil
			}
			return ctx.Err()
		}
	}
}

// calculateSuccessRate returns the success rate as percentage
func calculateSuccessRate(success, failure int) int {
	total := success + failure
	if total == 0 {
		return 100 // No operations, consider 100% success
	}
	return (success * 100) / total
}

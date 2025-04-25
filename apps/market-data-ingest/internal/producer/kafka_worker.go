package producer

import (
	"context"
	"errors"

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
	kw.logger.Info("Starting synchronous Kafka worker (using producer's internal retries)")

	for {
		select {
		case trade, ok := <-tradeChan:
			if !ok {
				kw.logger.Info("Trade channel closed, stopping Kafka worker")
				if err := kw.producer.Close(); err != nil {
					kw.logger.Error("Error closing Kafka producer", logger.Error(err))
				}
				kw.logger.Info("Kafka worker finished gracefully")
				return nil
			}

			// Track Kafka operation attempt
			metrics.KafkaPublishTotal.Inc()

			// Time the Kafka produce operation
			timer := metrics.NewTimer(metrics.KafkaOperationDuration)
			partition, offset, err := kw.producer.Produce(ctx, trade)
			timer.ObserveDuration()

			if err != nil {
				// Increment error counter on failure
				metrics.KafkaPublishErrorsTotal.Inc()

				kw.logger.Error("Failed to produce message after producer's internal retries",
					logger.Error(err),
					logger.String("symbol", trade.Symbol),
					logger.Any("trade", trade),
				)
				continue
			}

			kw.logger.Debug("produced message to kafka",
				logger.String("symbol", trade.Symbol),
				logger.Int("partition", int(partition)),
				logger.Int64("offset", offset),
			)

		case <-ctx.Done():
			kw.logger.Info("context cancelled, stopping Kafka worker")
			if err := kw.producer.Close(); err != nil {
				kw.logger.Error("Error closing Kafka producer on context cancellation", logger.Error(err))
			}
			kw.logger.Info("kafka worker finished due to context cancellation")
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil
			}
			return ctx.Err()
		}
	}
}

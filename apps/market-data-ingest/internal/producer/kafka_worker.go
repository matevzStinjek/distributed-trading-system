package producer

import (
	"context"
	"errors"
	"log/slog"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/interfaces"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type KafkaWorker struct {
	producer interfaces.KafkaProducer
	logger   *slog.Logger
}

func NewKafkaWorker(producer interfaces.KafkaProducer, logger *slog.Logger) *KafkaWorker {
	return &KafkaWorker{
		producer: producer,
		logger:   logger,
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
					kw.logger.Error("Error closing Kafka producer", slog.Any("error", err))
				}
				kw.logger.Info("Kafka worker finished gracefully")
				return nil
			}

			partition, offset, err := kw.producer.Produce(ctx, trade)

			if err != nil {
				kw.logger.Error("Failed to produce message after producer's internal retries",
					slog.Any("error", err),
					slog.String("symbol", trade.Symbol),
					slog.Any("trade", trade),
				)
				continue
			}

			kw.logger.Debug("produced message to kafka",
				slog.String("symbol", trade.Symbol),
				slog.Int("partition", int(partition)),
				slog.Int64("offset", offset),
			)

		case <-ctx.Done():
			kw.logger.Info("context cancelled, stopping Kafka worker")
			if err := kw.producer.Close(); err != nil {
				kw.logger.Error("Error closing Kafka producer on context cancellation", slog.Any("error", err))
			}
			kw.logger.Info("kafka worker finished due to context cancellation")
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil
			}
			return ctx.Err()
		}
	}
}

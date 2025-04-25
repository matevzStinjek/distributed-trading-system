package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/interfaces"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type SaramaSyncProducer struct {
	producer sarama.SyncProducer
	topic    string
	logger   *logger.Logger
}

func NewKafkaAsyncProducer(cfg *config.Config, log *logger.Logger) (*SaramaSyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Retry.Backoff = 100 * time.Millisecond
	config.Producer.Return.Successes = true
	config.Producer.Partitioner = sarama.NewHashPartitioner

	producer, err := sarama.NewSyncProducer(cfg.KafkaBrokers, config)
	if err != nil {
		return nil, err
	}
	return &SaramaSyncProducer{
		producer: producer,
		topic:    cfg.KafkaTopicMarketData,
		logger:   log,
	}, nil
}

func (p *SaramaSyncProducer) Produce(ctx context.Context, t marketdata.Trade) (int32, int64, error) {
	bytes, err := json.Marshal(t)
	if err != nil {
		p.logger.Error("could not marshal trade object for Kafka",
			logger.Error(err),
			logger.Any("trade", t))
		return -1, -1, fmt.Errorf("failed to marshal trade: %w", err)
	}

	message := &sarama.ProducerMessage{
		Topic:     p.topic,
		Key:       sarama.StringEncoder(t.Symbol),
		Value:     sarama.ByteEncoder(bytes),
		Timestamp: t.Timestamp,
	}

	partition, offset, err := p.producer.SendMessage(message)
	if err != nil {
		p.logger.Error("Failed to send message to Kafka",
			logger.Error(err),
			logger.String("symbol", t.Symbol))
		return -1, -1, err
	}

	p.logger.Debug("produced message to Kafka",
		logger.String("symbol", t.Symbol),
		logger.Int("partition", int(partition)),
		logger.Int64("offset", offset),
	)
	return partition, offset, nil
}

func (p *SaramaSyncProducer) Close() error {
	p.logger.Info("Closing Kafka sync producer")
	return p.producer.Close()
}

// interface check
var _ interfaces.KafkaProducer = (*SaramaSyncProducer)(nil)

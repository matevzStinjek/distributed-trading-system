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
	if len(cfg.KafkaBrokers) == 0 {
		return nil, fmt.Errorf("no Kafka brokers configured")
	}

	log.Info("configuring kafka producer",
		logger.Any("brokers", cfg.KafkaBrokers),
		logger.String("topic", cfg.KafkaTopicMarketData))

	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Retry.Backoff = 100 * time.Millisecond
	config.Producer.Return.Successes = true
	config.Producer.Partitioner = sarama.NewHashPartitioner

	log.Debug("connecting to kafka brokers")
	producer, err := sarama.NewSyncProducer(cfg.KafkaBrokers, config)
	if err != nil {
		log.Error("failed to create kafka producer",
			logger.Error(err),
			logger.Any("brokers", cfg.KafkaBrokers))
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	log.Info("kafka producer successfully connected")
	return &SaramaSyncProducer{
		producer: producer,
		topic:    cfg.KafkaTopicMarketData,
		logger:   log,
	}, nil
}

func (p *SaramaSyncProducer) Produce(ctx context.Context, t marketdata.Trade) (int32, int64, error) {
	// Serialize trade to JSON
	bytes, err := json.Marshal(t)
	if err != nil {
		p.logger.Error("failed to serialize trade",
			logger.Error(err),
			logger.String("symbol", t.Symbol),
			logger.Float64("price", t.Price))
		return -1, -1, fmt.Errorf("failed to marshal trade: %w", err)
	}

	// Create producer message
	message := &sarama.ProducerMessage{
		Topic:     p.topic,
		Key:       sarama.StringEncoder(t.Symbol),
		Value:     sarama.ByteEncoder(bytes),
		Timestamp: t.Timestamp,
	}

	// Send message to Kafka
	start := time.Now()
	partition, offset, err := p.producer.SendMessage(message)
	latency := time.Since(start)

	if err != nil {
		p.logger.Error("failed to produce message to kafka",
			logger.Error(err),
			logger.String("symbol", t.Symbol),
			logger.String("topic", p.topic),
			logger.Duration("attempt_duration", latency))
		return -1, -1, err
	}

	p.logger.Debug("message produced to kafka",
		logger.String("symbol", t.Symbol),
		logger.Int("partition", int(partition)),
		logger.Int64("offset", offset),
		logger.Duration("latency_ms", latency),
		logger.Int("msg_size_bytes", len(bytes)),
	)
	return partition, offset, nil
}

func (p *SaramaSyncProducer) Close() error {
	p.logger.Info("closing kafka producer")
	err := p.producer.Close()
	if err != nil {
		p.logger.Error("error closing kafka producer", logger.Error(err))
	} else {
		p.logger.Info("kafka producer closed successfully")
	}
	return err
}

// Verify interface
var _ interfaces.KafkaProducer = (*SaramaSyncProducer)(nil)

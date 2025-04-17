package kafka

import (
	"github.com/IBM/sarama"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
)

type SaramaAsyncProducer struct {
	Producer sarama.AsyncProducer
}

func NewKafkaAsyncProducer(cfg *config.Config) (*SaramaAsyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForLocal
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.Return.Errors = true

	producer, err := sarama.NewAsyncProducer(cfg.KafkaBrokers, config)
	if err != nil {
		return nil, err
	}
	return &SaramaAsyncProducer{
		Producer: producer,
	}, nil
}

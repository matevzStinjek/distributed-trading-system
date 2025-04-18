package kafka

import (
	"encoding/json"
	"log"

	"github.com/IBM/sarama"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type SaramaAsyncProducer struct {
	Producer sarama.AsyncProducer
	Topic    string
}

func (p *SaramaAsyncProducer) Produce(t marketdata.Trade) error {
	bytes, err := json.Marshal(t)
	if err != nil {
		log.Printf("could not marshall trade object: %v", err)
		return err
	}

	message := &sarama.ProducerMessage{
		Topic:     p.Topic,
		Key:       sarama.StringEncoder(t.Symbol),
		Value:     sarama.ByteEncoder(bytes),
		Timestamp: t.Timestamp,
	}
	p.Producer.Input() <- message
	return nil
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
		Topic:    cfg.KafkaTopicMarketData,
	}, nil
}

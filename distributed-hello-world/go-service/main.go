package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/IBM/sarama"
	"github.com/gorilla/mux"
)

const (
	TOPIC = "my-topic-1"
)

type Message struct {
	UserID  string `json:"user_id"`
	Content string `json:"content"`
}

type KafkaProducer struct {
	producer sarama.AsyncProducer
}

type ProducerService struct {
	producer *KafkaProducer
}

func (s *ProducerService) MessageHandler(w http.ResponseWriter, r *http.Request) {
	// validate input
	var msg Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// produce
	bytes, err := json.Marshal(msg)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	kafkaMsg := &sarama.ProducerMessage{
		Topic: TOPIC,
		Key:   sarama.StringEncoder(msg.UserID),
		Value: sarama.ByteEncoder(bytes),
	}

	s.producer.producer.Input() <- kafkaMsg

	// respond with ok
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func NewProducerService(p *KafkaProducer) ProducerService {
	return ProducerService{
		producer: p,
	}
}

func NewKafkaProducer(brokers []string) (*KafkaProducer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForLocal
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true

	producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	go func() {
		for success := range producer.Successes() {
			log.Printf("Message sent successfully: topic=%s partition=%d offset=%d", success.Topic, success.Partition, success.Offset)
		}
	}()

	go func() {
		for err := range producer.Errors() {
			log.Printf("Failed to send message: %v", err)
			// alert etc.
		}
	}()

	return &KafkaProducer{
		producer: producer,
	}, nil
}

type KafkaConsumer struct {
	group   sarama.ConsumerGroup
	topics  []string
	handler MessageHandler
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func (c *KafkaConsumer) Start() error {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			// in inf loop for rebalancing
			if err := c.group.Consume(c.ctx, c.topics, &c.handler); err != nil {
				if err == sarama.ErrClosedConsumerGroup {
					return
				}
				log.Printf("Error from consumer: %v", err)
			}
			if c.ctx.Err() != nil {
				return
			}
		}
	}()
	return nil
}

func (c *KafkaConsumer) Stop() error {
	c.cancel()
	c.wg.Wait()
	return c.group.Close()
}

type MessageHandler struct {
}

func (h *MessageHandler) Setup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *MessageHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *MessageHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		var msg Message
		if err := json.Unmarshal(message.Value, &msg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		log.Printf("Message received: user=%s content=%s", msg.UserID, msg.Content)

		session.MarkMessage(message, "")
	}
	return nil
}

func NewKafkaConsumer(brokers []string, groupId string, topics []string) (*KafkaConsumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupId, config)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &KafkaConsumer{
		group:   group,
		topics:  topics,
		handler: MessageHandler{},
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

func main() {
	KAFKA_BROKERS := os.Getenv("KAFKA_BROKERS")
	KAFKA_CONSUMER_GROUP := os.Getenv("KAFKA_CONSUMER_GROUP")

	// producer
	kafkaProducer, err := NewKafkaProducer([]string{KAFKA_BROKERS})
	if err != nil {
		log.Fatalf("Failed to create a producer: %s", err)
		return
	}
	producerService := NewProducerService(kafkaProducer)

	// consumer
	kafkaConsumer, err := NewKafkaConsumer([]string{KAFKA_BROKERS}, KAFKA_CONSUMER_GROUP, []string{TOPIC})
	if err != nil {
		log.Fatalf("Failed to create a producer: %s", err)
		return
	}
	if err := kafkaConsumer.Start(); err != nil {
		log.Fatalf("Failed to start consumer: %s", err)
		return
	}
	defer kafkaConsumer.Stop()

	// api
	r := mux.NewRouter()
	r.HandleFunc("/health", HealthHandler)
	r.HandleFunc("/message", producerService.MessageHandler)

	srv := http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	log.Printf("Server starting on :8080")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

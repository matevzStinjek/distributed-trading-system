package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/IBM/sarama"
	"github.com/gorilla/mux"
)

const (
	TOPIC = "my-topic-1"
)

type KafkaProducer struct {
	producer sarama.AsyncProducer
}

type Message struct {
	UserID  string `json:"user_id"`
	Content string `json:"content"`
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

func main() {
	KAFKA_BROKERS := os.Getenv("KAFKA_BROKERS")

	// producer
	kafkaProducer, err := NewKafkaProducer([]string{KAFKA_BROKERS})
	if err != nil {
		log.Fatalf("Failed to create a producer: %s", err)
	}
	producerService := NewProducerService(kafkaProducer)

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

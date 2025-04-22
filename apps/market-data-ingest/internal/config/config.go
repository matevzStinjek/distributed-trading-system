package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	AGG_INTERVAL_MS_DEFAULT = 50
	AGG_INTERVAL_MS_MIN     = 10
)

type Config struct {
	AggregatorInterval time.Duration

	KafkaBrokers         []string
	KafkaTopicMarketData string

	RedisCacheAddr string
	RedisCacheUser string
	RedisCachePw   string
	RedisCacheDB   int

	RedisPubsubAddr string
	RedisPubsubUser string
	RedisPubsubPw   string

	Symbols []string

	RawTradesChanBuff  int // 1k-5k
	ProcTradesChanBuff int // 500-1k
	KafkaChanBuff      int // 1k-10k
}

func LoadConfig() (*Config, error) {
	aggregatorIntervalMs, err := strconv.Atoi(os.Getenv("AGGREGATOR_INTERVAL_MS"))
	if err != nil || aggregatorIntervalMs < AGG_INTERVAL_MS_MIN {
		log.Printf("AGGREGATOR_INTERVAL_MS couldn't be parsed to a number or is less than %d, defaulting to %d", AGG_INTERVAL_MS_MIN, AGG_INTERVAL_MS_DEFAULT)
		aggregatorIntervalMs = AGG_INTERVAL_MS_DEFAULT
	}

	redisCacheDB, err := strconv.Atoi(os.Getenv("REDIS_CACHE_DB"))
	if err != nil {
		log.Fatalf("REDIS_CACHE_DB can't be parsed to a number: %v", err)
	}

	cfg := &Config{
		AggregatorInterval: time.Duration(aggregatorIntervalMs) * time.Millisecond,

		KafkaBrokers:         strings.Split(os.Getenv("KAFKA_BROKERS"), ","),
		KafkaTopicMarketData: os.Getenv("KAFKA_TOPIC_MARKET_DATA"),

		RedisCacheAddr: os.Getenv("REDIS_CACHE_ADDR"),
		RedisCacheUser: os.Getenv("REDIS_CACHE_UN"),
		RedisCachePw:   os.Getenv("REDIS_CACHE_PW"),
		RedisCacheDB:   redisCacheDB,

		RedisPubsubAddr: os.Getenv("REDIS_PUBSUB_ADDR"),
		RedisPubsubUser: os.Getenv("REDIS_PUBSUB_UN"),
		RedisPubsubPw:   os.Getenv("REDIS_PUBSUB_PW"),

		Symbols: []string{"AAPL", "MSFT", "GOOG", "AMZN", "TSLA"},

		RawTradesChanBuff:  100,
		ProcTradesChanBuff: 50,
		KafkaChanBuff:      500,
	}
	return cfg, nil
}

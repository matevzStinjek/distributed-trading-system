package config

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const (
	AGG_INTERVAL_MS_DEFAULT = 100
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

func LoadConfig(getenv func(string) string, logger *slog.Logger) (*Config, error) {
	aggregatorIntervalMsStr := getenv("AGGREGATOR_INTERVAL_MS")
	aggregatorIntervalMs, err := strconv.Atoi(aggregatorIntervalMsStr)
	if err != nil || aggregatorIntervalMs < AGG_INTERVAL_MS_MIN {
		logger.Warn("AGGREGATOR_INTERVAL_MS invalid, defaulting",
			slog.String("value", aggregatorIntervalMsStr),
			slog.Int("min_ms", AGG_INTERVAL_MS_MIN),
			slog.Int("default_ms", AGG_INTERVAL_MS_DEFAULT))
		aggregatorIntervalMs = AGG_INTERVAL_MS_DEFAULT
	}

	redisCacheDB, err := strconv.Atoi(getenv("REDIS_CACHE_DB"))
	if err != nil {
		return nil, fmt.Errorf("REDIS_CACHE_DB can't be parsed to a number: %v", err)
	}

	cfg := &Config{
		AggregatorInterval: time.Duration(aggregatorIntervalMs) * time.Millisecond,

		KafkaBrokers:         strings.Split(getenv("KAFKA_BROKERS"), ","),
		KafkaTopicMarketData: getenv("KAFKA_TOPIC_MARKET_DATA"),

		RedisCacheAddr: getenv("REDIS_CACHE_ADDR"),
		RedisCacheUser: getenv("REDIS_CACHE_UN"),
		RedisCachePw:   getenv("REDIS_CACHE_PW"),
		RedisCacheDB:   redisCacheDB,

		RedisPubsubAddr: getenv("REDIS_PUBSUB_ADDR"),
		RedisPubsubUser: getenv("REDIS_PUBSUB_UN"),
		RedisPubsubPw:   getenv("REDIS_PUBSUB_PW"),

		Symbols: []string{"AAPL", "MSFT", "GOOG", "AMZN", "TSLA"},

		RawTradesChanBuff:  100,
		ProcTradesChanBuff: 50,
		KafkaChanBuff:      500,
	}
	return cfg, nil
}

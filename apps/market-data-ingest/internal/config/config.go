package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
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

func LoadConfig(getenv func(string) string, log *logger.Logger) (*Config, error) {
	log.Info("loading configuration from environment")

	// Aggregator interval handling
	aggregatorIntervalMsStr := getenv("AGGREGATOR_INTERVAL_MS")
	aggregatorIntervalMs, err := strconv.Atoi(aggregatorIntervalMsStr)
	if err != nil {
		log.Warn("invalid aggregator interval format, using default",
			logger.String("value", aggregatorIntervalMsStr),
			logger.Int("default_ms", AGG_INTERVAL_MS_DEFAULT),
			logger.Error(err))
		aggregatorIntervalMs = AGG_INTERVAL_MS_DEFAULT
	} else if aggregatorIntervalMs < AGG_INTERVAL_MS_MIN {
		log.Warn("aggregator interval too low, using minimum value",
			logger.Int("provided_ms", aggregatorIntervalMs),
			logger.Int("min_ms", AGG_INTERVAL_MS_MIN))
		aggregatorIntervalMs = AGG_INTERVAL_MS_MIN
	}

	// Redis cache DB handling
	redisCacheDBStr := getenv("REDIS_CACHE_DB")
	redisCacheDB, err := strconv.Atoi(redisCacheDBStr)
	if err != nil {
		log.Error("invalid Redis cache DB value, must be a number",
			logger.String("value", redisCacheDBStr),
			logger.Error(err))
		return nil, fmt.Errorf("REDIS_CACHE_DB can't be parsed to a number: %w", err)
	}

	// Kafka broker configuration
	kafkaBrokersStr := getenv("KAFKA_BROKERS")
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")
	if len(kafkaBrokers) == 1 && kafkaBrokers[0] == "" {
		log.Warn("no Kafka brokers specified, messages will not be published")
		kafkaBrokers = []string{}
	}

	// Kafka topic validation
	kafkaTopic := getenv("KAFKA_TOPIC_MARKET_DATA")
	if kafkaTopic == "" {
		log.Warn("Kafka topic not specified, defaulting to 'market-data'")
		kafkaTopic = "market-data"
	}

	// Configure channel buffer sizes
	rawTradesChanBuff := 100 // Default for raw trades buffer
	procTradesChanBuff := 50 // Default for processed trades buffer
	kafkaChanBuff := 500     // Default for Kafka buffer

	// Create the config object
	cfg := &Config{
		AggregatorInterval: time.Duration(aggregatorIntervalMs) * time.Millisecond,

		KafkaBrokers:         kafkaBrokers,
		KafkaTopicMarketData: kafkaTopic,

		RedisCacheAddr: getenv("REDIS_CACHE_ADDR"),
		RedisCacheUser: getenv("REDIS_CACHE_UN"),
		RedisCachePw:   getenv("REDIS_CACHE_PW"),
		RedisCacheDB:   redisCacheDB,

		RedisPubsubAddr: getenv("REDIS_PUBSUB_ADDR"),
		RedisPubsubUser: getenv("REDIS_PUBSUB_UN"),
		RedisPubsubPw:   getenv("REDIS_PUBSUB_PW"),

		Symbols: parseSymbols(getenv("SYMBOLS"), log),

		RawTradesChanBuff:  rawTradesChanBuff,
		ProcTradesChanBuff: procTradesChanBuff,
		KafkaChanBuff:      kafkaChanBuff,
	}

	// Log the configuration (hiding sensitive values)
	log.Info("configuration loaded successfully",
		logger.Duration("aggregator_interval", cfg.AggregatorInterval),
		logger.Any("kafka_brokers", cfg.KafkaBrokers),
		logger.String("kafka_topic", cfg.KafkaTopicMarketData),
		logger.String("redis_cache_addr", cfg.RedisCacheAddr),
		logger.Int("redis_cache_db", cfg.RedisCacheDB),
		logger.String("redis_pubsub_addr", cfg.RedisPubsubAddr),
		logger.Any("symbols", cfg.Symbols),
		logger.Int("symbols_count", len(cfg.Symbols)),
		logger.Int("raw_buff_size", cfg.RawTradesChanBuff),
		logger.Int("proc_buff_size", cfg.ProcTradesChanBuff),
		logger.Int("kafka_buff_size", cfg.KafkaChanBuff))

	return cfg, nil
}

// parseSymbols parses the comma-separated list of symbols from the environment
func parseSymbols(symbolsStr string, log *logger.Logger) []string {
	// Default symbols in case none are provided
	defaultSymbols := []string{"AAPL", "MSFT", "GOOG", "AMZN", "TSLA"}

	if symbolsStr == "" {
		log.Info("no symbols provided, using default symbols",
			logger.Any("symbols", defaultSymbols))
		return defaultSymbols
	}

	symbols := strings.Split(symbolsStr, ",")
	// Trim whitespace from each symbol
	for i := range symbols {
		symbols[i] = strings.TrimSpace(symbols[i])
	}

	// Filter out empty symbols
	var filteredSymbols []string
	for _, s := range symbols {
		if s != "" {
			filteredSymbols = append(filteredSymbols, s)
		}
	}

	if len(filteredSymbols) == 0 {
		log.Warn("all provided symbols were empty, using default symbols",
			logger.Any("defaults", defaultSymbols))
		return defaultSymbols
	}

	return filteredSymbols
}

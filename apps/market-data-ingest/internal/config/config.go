package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	Symbols []string

	RedisCacheAddr string
	RedisCacheUser string
	RedisCachePw   string
	RedisCacheDB   int

	RedisPubsubAddr string
	RedisPubsubUser string
	RedisPubsubPw   string

	TradeChannelBuff int
}

func LoadConfig() (*Config, error) {
	redisCacheDB, err := strconv.Atoi(os.Getenv("REDIS_CACHE_DB"))
	if err != nil {
		log.Fatalf("REDIS_CACHE_DB can't be parsed to a number: %v", err)
	}

	cfg := &Config{
		Symbols: []string{"AAPL", "MSFT", "GOOG", "AMZN", "TSLA"},

		RedisCacheAddr: os.Getenv("REDIS_CACHE_ADDR"),
		RedisCacheUser: os.Getenv("REDIS_CACHE_UN"),
		RedisCachePw:   os.Getenv("REDIS_CACHE_PW"),
		RedisCacheDB:   redisCacheDB,

		RedisPubsubAddr: os.Getenv("REDIS_PUBSUB_ADDR"),
		RedisPubsubUser: os.Getenv("REDIS_PUBSUB_UN"),
		RedisPubsubPw:   os.Getenv("REDIS_PUBSUB_PW"),

		TradeChannelBuff: 100,
	}
	return cfg, nil
}

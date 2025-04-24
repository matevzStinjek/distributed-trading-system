package redis

import (
	"context"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/redis/go-redis/v9"
)

type RedisCacheClient struct {
	Client *redis.Client
}

func NewRedisCacheClient(ctx context.Context, cfg *config.Config) (*RedisCacheClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisCacheAddr,
		Username: cfg.RedisCacheUser,
		Password: cfg.RedisCachePw,
		DB:       cfg.RedisCacheDB,
	})

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisCacheClient{
		Client: client,
	}, nil
}

package redis

import (
	"context"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/redis/go-redis/v9"
)

type RedisPubsubClient struct {
	Client *redis.Client
}

func NewRedisPubsubClient(ctx context.Context, cfg *config.Config) (*RedisPubsubClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisPubsubAddr,
		Username: cfg.RedisPubsubUser,
		Password: cfg.RedisPubsubPw,
	})

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisPubsubClient{
		Client: client,
	}, nil
}

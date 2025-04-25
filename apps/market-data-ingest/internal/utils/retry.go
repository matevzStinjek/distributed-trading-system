package utils

import (
	"context"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
)

type RetryConfig struct {
	InitialInterval     time.Duration
	MaxInterval         time.Duration
	Multiplier          float64
	MaxElapsedTime      time.Duration
	RandomizationFactor float64
}

func DefaultTradeRetryConfig() RetryConfig {
	return RetryConfig{
		InitialInterval:     5 * time.Millisecond,
		MaxInterval:         25 * time.Millisecond,
		Multiplier:          2.0,
		MaxElapsedTime:      75 * time.Millisecond,
		RandomizationFactor: 0.3,
	}
}

type RetryableOperation func(ctx context.Context) error

func RetryWithConfig(ctx context.Context, op RetryableOperation, cfg RetryConfig, log *logger.Logger) error {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = cfg.InitialInterval
	b.MaxInterval = cfg.MaxInterval
	b.Multiplier = cfg.Multiplier
	b.MaxElapsedTime = cfg.MaxElapsedTime
	b.RandomizationFactor = cfg.RandomizationFactor

	b.Reset()

	return backoff.RetryNotify(
		func() error {
			return op(ctx)
		},
		b,
		func(err error, d time.Duration) {
			log.Warn("retry operation failed",
				logger.Error(err),
				logger.Duration("backoff_duration", d))
		},
	)
}

func RetryWithDefault(ctx context.Context, op RetryableOperation, log *logger.Logger) error {
	return RetryWithConfig(ctx, op, DefaultTradeRetryConfig(), log)
}

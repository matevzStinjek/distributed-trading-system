package utils

import (
	"context"
	"log/slog"
	"time"

	"github.com/cenkalti/backoff"
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

func RetryWithConfig(ctx context.Context, op RetryableOperation, cfg RetryConfig, logger *slog.Logger) error {
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
			logger.Warn("notify", slog.Any("error", err), slog.Duration("d", d))
		},
	)
}

func RetryWithDefault(ctx context.Context, op RetryableOperation, logger *slog.Logger) error {
	return RetryWithConfig(ctx, op, DefaultTradeRetryConfig(), logger)
}

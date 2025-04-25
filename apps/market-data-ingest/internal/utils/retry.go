package utils

import (
	"context"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
)

// RetryConfig defines the configuration for the retry mechanism
type RetryConfig struct {
	InitialInterval     time.Duration
	MaxInterval         time.Duration
	Multiplier          float64
	MaxElapsedTime      time.Duration
	RandomizationFactor float64
}

// DefaultTradeRetryConfig returns a default retry configuration for trade operations
func DefaultTradeRetryConfig() RetryConfig {
	return RetryConfig{
		InitialInterval:     5 * time.Millisecond,
		MaxInterval:         25 * time.Millisecond,
		Multiplier:          2.0,
		MaxElapsedTime:      75 * time.Millisecond,
		RandomizationFactor: 0.3,
	}
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func(ctx context.Context) error

// RetryWithConfig retries the given operation with the specified configuration
func RetryWithConfig(ctx context.Context, op RetryableOperation, cfg RetryConfig, log *logger.Logger) error {
	log.Debug("configuring retry mechanism",
		logger.Duration("initial_interval", cfg.InitialInterval),
		logger.Duration("max_interval", cfg.MaxInterval),
		logger.Duration("max_elapsed_time", cfg.MaxElapsedTime),
		logger.Float64("multiplier", cfg.Multiplier))

	b := backoff.NewExponentialBackOff()
	b.InitialInterval = cfg.InitialInterval
	b.MaxInterval = cfg.MaxInterval
	b.Multiplier = cfg.Multiplier
	b.MaxElapsedTime = cfg.MaxElapsedTime
	b.RandomizationFactor = cfg.RandomizationFactor

	b.Reset()

	var attempt int
	start := time.Now()

	err := backoff.RetryNotify(
		func() error {
			attempt++

			// Check context before attempting operation
			if ctx.Err() != nil {
				log.Debug("retry abandoned due to context cancellation",
					logger.Error(ctx.Err()),
					logger.Int("attempts", attempt))
				return backoff.Permanent(ctx.Err())
			}

			// Execute the operation
			opStart := time.Now()
			err := op(ctx)
			opDuration := time.Since(opStart)

			if err != nil {
				log.Debug("operation failed, will retry",
					logger.Error(err),
					logger.Int("attempt", attempt),
					logger.Duration("op_duration", opDuration))
				return err
			}

			log.Debug("operation succeeded",
				logger.Int("attempt", attempt),
				logger.Duration("op_duration", opDuration))
			return nil
		},
		b,
		func(err error, d time.Duration) {
			log.Warn("retrying operation after failure",
				logger.Error(err),
				logger.Int("attempt", attempt),
				logger.Duration("backoff_duration", d),
				logger.Duration("elapsed", time.Since(start)))
		},
	)

	totalDuration := time.Since(start)
	if err != nil {
		log.Error("all retry attempts failed",
			logger.Error(err),
			logger.Int("attempts", attempt),
			logger.Duration("total_duration", totalDuration))
	} else if attempt > 1 {
		log.Info("operation succeeded after retries",
			logger.Int("attempts", attempt),
			logger.Duration("total_duration", totalDuration))
	}

	return err
}

// RetryWithDefault retries the operation with default configuration
func RetryWithDefault(ctx context.Context, op RetryableOperation, log *logger.Logger) error {
	return RetryWithConfig(ctx, op, DefaultTradeRetryConfig(), log)
}

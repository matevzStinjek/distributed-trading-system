package processor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/metrics"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/utils"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/interfaces"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
	"golang.org/x/sync/errgroup"
)

type TradeProcessor struct {
	cacheClient  interfaces.CacheClient
	pubsubClient interfaces.PubsubClient
	logger       *slog.Logger
	wg           *sync.WaitGroup
}

func NewTradeProcessor(
	cacheClient interfaces.CacheClient,
	pubsubClient interfaces.PubsubClient,
	logger *slog.Logger,
) *TradeProcessor {
	return &TradeProcessor{
		cacheClient:  cacheClient,
		pubsubClient: pubsubClient,
		logger:       logger,
		wg:           &sync.WaitGroup{},
	}
}

func (tp *TradeProcessor) Start(
	ctx context.Context,
	tradeChannel <-chan marketdata.Trade,
	bgTradesChan chan<- marketdata.Trade,
) error {
	defer func() {
		close(bgTradesChan)
		tp.logger.Info("bgTradesChan closed")
	}()

	for {
		select {
		case trade, ok := <-tradeChannel:
			if !ok {
				tp.logger.Info("trade channel closed, processor stopping...")
				tp.wg.Wait()
				tp.logger.Info("all pending trade processing finished")
				return nil
			}

			tp.wg.Add(1)
			go func(trade marketdata.Trade) {
				defer tp.wg.Done()
				// Start timing the processing operation
				timer := metrics.NewTimer(metrics.RedisOperationDuration)
				defer timer.ObserveDuration()

				err := tp.processTrade(ctx, trade, bgTradesChan)
				if err != nil {
					tp.logger.Error("error processing trade", slog.Any("error", err))
				} else {
					// Increment processed trades counter on success
					metrics.TradesProcessedTotal.Inc()
				}
			}(trade)

		case <-ctx.Done():
			tp.logger.Info("Processor context cancelled, stopping...")
			tp.wg.Wait()
			tp.logger.Info("All pending trade processing finished due to context cancellation.")
			return nil
		}
	}
}

func (tp *TradeProcessor) processTrade(ctx context.Context, trade marketdata.Trade, bgTradesChan chan<- marketdata.Trade) error {
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	key := fmt.Sprintf("price:%s", trade.Symbol)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		op := func(ctx context.Context) error {
			// Increment the Redis SET attempt counter
			metrics.RedisSetTotal.Inc()

			// Measure the Redis SET operation duration
			setTimer := metrics.NewTimer(metrics.RedisOperationDuration)
			err := tp.cacheClient.Set(ctx, key, trade.Price, 0)
			setTimer.ObserveDuration()

			if err != nil {
				// Increment error counter on failure
				metrics.RedisSetErrorsTotal.Inc()
			}
			return err
		}
		return utils.RetryWithDefault(ctx, op, tp.logger)
	})

	g.Go(func() error {
		op := func(ctx context.Context) error {
			// Increment the Redis PUBLISH attempt counter
			metrics.RedisPublishTotal.Inc()

			// Measure the Redis PUBLISH operation duration
			pubTimer := metrics.NewTimer(metrics.RedisOperationDuration)
			err := tp.pubsubClient.Publish(ctx, key, trade.Price)
			pubTimer.ObserveDuration()

			if err != nil {
				// Increment error counter on failure
				metrics.RedisPublishErrorsTotal.Inc()
			}
			return err
		}
		return utils.RetryWithDefault(ctx, op, tp.logger)
	})

	select {
	case bgTradesChan <- trade:
		// Successfully sent to Kafka channel
	default:
		tp.logger.Warn("bgTradesChan full, dropping trade", slog.Any("trade", trade))
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("trade processing encountered an error: %w", err)
	}
	tp.logger.Debug("Published price updates to redis",
		slog.String("Symbol", trade.Symbol),
		slog.Int("Price", int(trade.Price)))

	return nil
}

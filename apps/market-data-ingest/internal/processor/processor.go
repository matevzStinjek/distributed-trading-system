package processor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

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
				err := tp.processTrade(ctx, trade, bgTradesChan)
				if err != nil {
					tp.logger.Error("error processing trade", slog.Any("error", err))
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
			return tp.cacheClient.Set(ctx, key, trade.Price, 0)
		}
		return utils.RetryWithDefault(ctx, op, tp.logger)
	})

	g.Go(func() error {
		op := func(ctx context.Context) error {
			return tp.pubsubClient.Publish(ctx, key, trade.Price)
		}
		return utils.RetryWithDefault(ctx, op, tp.logger)
	})

	select {
	case bgTradesChan <- trade:
	default:
		tp.logger.Warn("bgTradesChan full, dropping trade", slog.Any("trade", trade))
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("trade processing encountered an error: %w", err)
	}

	return nil
}

package aggregator

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type TradeAggregator struct {
	mutex        sync.Mutex
	latestTrades map[string]marketdata.Trade
	cfg          *config.Config
	logger       *slog.Logger
}

func NewTradeAggregator(cfg *config.Config, logger *slog.Logger) *TradeAggregator {
	return &TradeAggregator{
		latestTrades: make(map[string]marketdata.Trade),
		cfg:          cfg,
		logger:       logger,
	}
}

func (ta *TradeAggregator) Start(
	ctx context.Context,
	rawTradesChan <-chan marketdata.Trade,
	processedTradesChan chan<- marketdata.Trade,
) error {
	defer func() {
		close(processedTradesChan)
		ta.logger.Info("processedTradesChan closed")
	}()

	ticker := time.NewTicker(ta.cfg.AggregatorInterval)
	defer ticker.Stop()

	for {
		select {
		case trade, ok := <-rawTradesChan:
			if !ok {
				ta.logger.Info("rawTradesChan has been closed")
				ta.flushPrices(processedTradesChan)
				return nil
			}
			// update trade
			ta.mutex.Lock()
			ta.latestTrades[trade.Symbol] = trade
			ta.mutex.Unlock()
			ta.logger.Debug("price updated", slog.String("Symbol", trade.Symbol))
		case <-ticker.C:
			ta.flushPrices(processedTradesChan)
		case <-ctx.Done():
			ta.flushPrices(processedTradesChan)
			return ctx.Err()
		}
	}
}

func (ta *TradeAggregator) flushPrices(processedTradesChan chan<- marketdata.Trade) {
	if len(ta.latestTrades) == 0 {
		return
	}

	ta.mutex.Lock()
	tradesToSend := make([]marketdata.Trade, 0, len(ta.latestTrades))
	for _, trade := range ta.latestTrades {
		tradesToSend = append(tradesToSend, trade)
	}
	ta.latestTrades = make(map[string]marketdata.Trade)
	ta.mutex.Unlock()

	ta.logger.Debug("flushing trades", slog.Any("trades", tradesToSend))
	for _, trade := range tradesToSend {
		select {
		case processedTradesChan <- trade:
		default:
			ta.logger.Warn("processedTradesChan full, dropping trade", slog.Any("trade", trade))
		}
	}
}

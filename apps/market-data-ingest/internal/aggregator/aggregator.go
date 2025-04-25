package aggregator

import (
	"context"
	"sync"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/metrics"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type TradeAggregator struct {
	mutex        sync.Mutex
	latestTrades map[string]marketdata.Trade
	cfg          *config.Config
	logger       *logger.Logger
}

func NewTradeAggregator(cfg *config.Config, log *logger.Logger) *TradeAggregator {
	return &TradeAggregator{
		latestTrades: make(map[string]marketdata.Trade),
		cfg:          cfg,
		logger:       log,
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

	// Start a separate goroutine to update the map size metric
	go func() {
		mapSizeTicker := time.NewTicker(5 * time.Second)
		defer mapSizeTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-mapSizeTicker.C:
				ta.mutex.Lock()
				metrics.AggregatorMapSize.Set(float64(len(ta.latestTrades)))
				ta.mutex.Unlock()
			}
		}
	}()

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
			ta.logger.Debug("price updated", logger.String("Symbol", trade.Symbol))
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

	// Use a timer to measure flush duration
	timer := metrics.NewTimer(metrics.AggregationDuration)
	defer timer.ObserveDuration()

	ta.mutex.Lock()
	tradesToSend := make([]marketdata.Trade, 0, len(ta.latestTrades))
	for _, trade := range ta.latestTrades {
		tradesToSend = append(tradesToSend, trade)
	}
	mapSize := len(ta.latestTrades)
	ta.latestTrades = make(map[string]marketdata.Trade)
	ta.mutex.Unlock()

	// Set the aggregator map size metric immediately after clearing
	metrics.AggregatorMapSize.Set(0)

	ta.logger.Debug("flushing trades", logger.Any("trades", tradesToSend))
	for _, trade := range tradesToSend {
		select {
		case processedTradesChan <- trade:
			// Increment the aggregated trades counter
			metrics.TradesAggregatedTotal.Inc()
		default:
			ta.logger.Warn("processedTradesChan full, dropping trade", logger.Any("trade", trade))
		}
	}

	// Log the number of trades aggregated
	ta.logger.Debug("trades aggregated", logger.Int("count", mapSize))
}

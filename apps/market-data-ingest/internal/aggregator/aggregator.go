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

type TickerFactory func(d time.Duration) *time.Ticker

func DefaultTickerFactory(d time.Duration) *time.Ticker {
	return time.NewTicker(d)
}

type TradeAggregator struct {
	mutex        sync.Mutex
	latestTrades map[string]marketdata.Trade
	cfg          *config.Config
	logger       *logger.Logger
	newTicker    TickerFactory
}

func NewTradeAggregator(cfg *config.Config, log *logger.Logger) *TradeAggregator {
	return NewTradeAggregatorWithTicker(cfg, log, DefaultTickerFactory)
}

func NewTradeAggregatorWithTicker(cfg *config.Config, log *logger.Logger, tickerFactory TickerFactory) *TradeAggregator {
	return &TradeAggregator{
		latestTrades: make(map[string]marketdata.Trade),
		cfg:          cfg,
		logger:       log,
		newTicker:    tickerFactory,
	}
}

func (ta *TradeAggregator) Start(
	ctx context.Context,
	rawTradesChan <-chan marketdata.Trade,
	processedTradesChan chan<- marketdata.Trade,
) error {
	interval := ta.cfg.AggregatorInterval
	ta.logger.Info("trade aggregator starting",
		logger.Duration("flush_interval", interval),
		logger.Int("output_channel_buffer", cap(processedTradesChan)))

	defer func() {
		ta.logger.Info("closing processed trades channel")
		close(processedTradesChan)
	}()

	ticker := ta.newTicker(interval)
	defer ticker.Stop()

	ta.logger.Debug("initializing aggregator map metric collector")
	// Start a separate goroutine to update the map size metric
	go func() {
		mapSizeTicker := time.NewTicker(5 * time.Second)
		defer mapSizeTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				ta.logger.Debug("stopping map size metric collector")
				return
			case <-mapSizeTicker.C:
				ta.mutex.Lock()
				size := len(ta.latestTrades)
				ta.mutex.Unlock()

				metrics.AggregatorMapSize.Set(float64(size))
				ta.logger.Debug("updated aggregator map size metric", logger.Int("size", size))
			}
		}
	}()

	ta.logger.Info("aggregator main loop started", logger.Duration("flush_interval", interval))
	var tradesReceived int

	for {
		select {
		case trade, ok := <-rawTradesChan:
			if !ok {
				ta.logger.Info("raw trades channel closed, performing final flush",
					logger.Int("total_received", tradesReceived))
				ta.flushPrices(processedTradesChan)
				return nil
			}
			// Update trade in aggregation map
			ta.mutex.Lock()
			ta.latestTrades[trade.Symbol] = trade
			mapSize := len(ta.latestTrades)
			ta.mutex.Unlock()

			tradesReceived++

			// Log progress periodically without excessive logging
			if tradesReceived%50 == 0 {
				ta.logger.Info("aggregation progress",
					logger.Int("trades_received", tradesReceived),
					logger.Int("symbols_in_map", mapSize))
			} else {
				ta.logger.Debug("trade received for aggregation",
					logger.String("symbol", trade.Symbol),
					logger.Float64("price", trade.Price))
			}

		case <-ticker.C:
			ta.logger.Debug("aggregation interval elapsed, flushing prices")
			ta.flushPrices(processedTradesChan)

		case <-ctx.Done():
			ta.logger.Info("received shutdown signal, performing final flush")
			ta.flushPrices(processedTradesChan)
			return ctx.Err()
		}
	}
}

func (ta *TradeAggregator) flushPrices(processedTradesChan chan<- marketdata.Trade) {
	ta.mutex.Lock()
	mapSize := len(ta.latestTrades)
	ta.mutex.Unlock()

	if mapSize == 0 {
		ta.logger.Debug("no trades to flush")
		return
	}

	// Use a timer to measure flush duration
	start := time.Now()
	timer := metrics.NewTimer(metrics.AggregationDuration)

	// Create a snapshot of the trades to process
	ta.mutex.Lock()
	tradesToSend := make([]marketdata.Trade, 0, mapSize)
	for _, trade := range ta.latestTrades {
		tradesToSend = append(tradesToSend, trade)
	}
	// Clear the map after taking the snapshot
	ta.latestTrades = make(map[string]marketdata.Trade)
	ta.mutex.Unlock()

	// Set the aggregator map size metric to zero after clearing
	metrics.AggregatorMapSize.Set(0)

	// Track how many trades were sent vs dropped
	var sentCount, droppedCount int

	// Forward trades to the processed channel
	for _, trade := range tradesToSend {
		select {
		case processedTradesChan <- trade:
			// Increment the aggregated trades counter
			metrics.TradesAggregatedTotal.Inc()
			sentCount++
		default:
			ta.logger.Warn("processed trades channel full, dropping trade",
				logger.String("symbol", trade.Symbol),
				logger.Float64("price", trade.Price))
			droppedCount++
		}
	}

	// Complete timing of flush operation
	timer.ObserveDuration()
	duration := time.Since(start)

	// Log the results
	if sentCount > 0 {
		ta.logger.Debug("trades flushed to processing",
			logger.Int("trades_sent", sentCount),
			logger.Int("trades_dropped", droppedCount),
			logger.Int("symbols_count", mapSize),
			logger.Duration("flush_duration", duration))
	} else {
		ta.logger.Debug("no trades sent during flush",
			logger.Int("attempted", mapSize),
			logger.Int("dropped", droppedCount))
	}
}

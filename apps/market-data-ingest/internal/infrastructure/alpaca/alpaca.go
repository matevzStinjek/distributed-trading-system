package alpaca

import (
	"context"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/interfaces"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type AlpacaClient struct {
	client  *stream.StocksClient
	symbols []string
	logger  *logger.Logger
	stats   *statsTracker
}

// statsTracker keeps track of trade statistics
type statsTracker struct {
	tradesReceived int
	tradesSent     int
	tradesDropped  int
	lastLogTime    time.Time
}

func NewAlpacaClient(ctx context.Context, cfg *config.Config, log *logger.Logger) (*AlpacaClient, error) {
	log.Info("initializing Alpaca market data client",
		logger.Any("symbols", cfg.Symbols))

	client := stream.NewStocksClient("iex")
	log.Debug("connecting to Alpaca streaming API")

	// Connect to the Alpaca stream
	if err := client.Connect(ctx); err != nil {
		log.Error("failed to connect to Alpaca API", logger.Error(err))
		return nil, err
	}

	log.Info("successfully connected to Alpaca streaming API")
	return &AlpacaClient{
		client:  client,
		symbols: cfg.Symbols,
		logger:  log,
		stats: &statsTracker{
			lastLogTime: time.Now(),
		},
	}, nil
}

func (mc *AlpacaClient) SubscribeToSymbols(ctx context.Context, tradeChan chan<- marketdata.Trade, symbols []string) error {
	if len(symbols) == 0 {
		mc.logger.Warn("empty symbol list provided, using default symbols")
		symbols = mc.symbols
	}

	mc.logger.Info("subscribing to market data streams",
		logger.Any("symbols", symbols),
		logger.Int("symbol_count", len(symbols)))

	// Set up a goroutine to log statistics periodically
	go mc.logStatsPeriodically(ctx)

	// Subscribe to trade updates
	return mc.client.SubscribeToTrades(func(t stream.Trade) {
		// Convert from Alpaca trade to internal trade representation
		trade := marketdata.Trade{
			ID:        t.ID,
			Symbol:    t.Symbol,
			Price:     t.Price,
			Size:      t.Size,
			Timestamp: t.Timestamp,
		}

		// Track statistics
		mc.stats.tradesReceived++

		// Send trade to the channel if possible
		select {
		case tradeChan <- trade:
			mc.stats.tradesSent++
			mc.logger.Debug("trade received from Alpaca",
				logger.String("symbol", trade.Symbol),
				logger.Float64("price", trade.Price),
				logger.Int("size", int(trade.Size)))
		case <-ctx.Done():
			mc.logger.Info("trade processing canceled by context")
			return
		default:
			mc.stats.tradesDropped++
			mc.logger.Warn("buffer full, dropping trade",
				logger.String("symbol", trade.Symbol),
				logger.Float64("price", trade.Price))
		}

		// Log milestones for high volume
		if mc.stats.tradesReceived%50 == 0 {
			mc.logStats("trade milestone reached")
		}
	}, symbols...)
}

func (mc *AlpacaClient) Close() error {
	symbols := mc.symbols
	mc.logger.Info("unsubscribing from market data streams",
		logger.Any("symbols", symbols),
		logger.Int("symbol_count", len(symbols)))

	err := mc.client.UnsubscribeFromTrades(symbols...)
	if err != nil {
		mc.logger.Error("error unsubscribing from trades", logger.Error(err))
	} else {
		mc.logger.Info("successfully unsubscribed from all symbols")
	}

	// Log final statistics
	mc.logStats("final market data statistics")

	return err
}

// logStatsPeriodically logs statistics at regular intervals
func (mc *AlpacaClient) logStatsPeriodically(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.logStats("periodic market data statistics")
		case <-ctx.Done():
			return
		}
	}
}

// logStats logs the current statistics with the given event name
func (mc *AlpacaClient) logStats(event string) {
	now := time.Now()
	elapsed := now.Sub(mc.stats.lastLogTime)

	if elapsed.Seconds() > 0 && mc.stats.tradesReceived > 0 {
		tradesPerSecond := float64(mc.stats.tradesReceived) / elapsed.Seconds()
		dropRate := 0.0
		if mc.stats.tradesReceived > 0 {
			dropRate = float64(mc.stats.tradesDropped) / float64(mc.stats.tradesReceived) * 100
		}

		mc.logger.Info(event,
			logger.Int("trades_received", mc.stats.tradesReceived),
			logger.Int("trades_sent", mc.stats.tradesSent),
			logger.Int("trades_dropped", mc.stats.tradesDropped),
			logger.Float64("trades_per_second", tradesPerSecond),
			logger.Float64("drop_rate_pct", dropRate),
			logger.Duration("period", elapsed))

		// Reset stats for next period
		mc.stats.lastLogTime = now
	}
}

// Verify interface
var _ interfaces.MarketDataClient = (*AlpacaClient)(nil)

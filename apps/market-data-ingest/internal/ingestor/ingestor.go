package ingestor

import (
	"context"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/metrics"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/interfaces"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type TradeIngestor struct {
	client interfaces.MarketDataClient
	cfg    *config.Config
	logger *logger.Logger
}

func NewTradeIngestor(
	client interfaces.MarketDataClient,
	cfg *config.Config,
	logger *logger.Logger,
) *TradeIngestor {
	return &TradeIngestor{
		client: client,
		cfg:    cfg,
		logger: logger,
	}
}

// Start begins the ingestor operation, receiving trades from the market data client
// and sending them to the rawTradesChan.
func (ti *TradeIngestor) Start(
	ctx context.Context,
	rawTradesChan chan<- marketdata.Trade,
) error {
	ti.logger.Info("trade ingestor starting",
		logger.Any("symbols", ti.cfg.Symbols),
		logger.Int("channel_buffer", ti.cfg.RawTradesChanBuff))

	defer func() {
		ti.logger.Info("closing raw trades channel")
		close(rawTradesChan)
	}()

	// Create a proxy channel to count incoming trades
	proxyChan := make(chan marketdata.Trade, ti.cfg.RawTradesChanBuff)
	ti.logger.Debug("created proxy channel for trade metrics")

	// Start a goroutine to forward trades and increment metrics
	go func() {
		ti.logger.Debug("starting trade forwarding goroutine")
		receivedCount := 0

		for {
			select {
			case <-ctx.Done():
				ti.logger.Debug("trade forwarding stopped", logger.Int("total_received", receivedCount))
				return
			case trade, ok := <-proxyChan:
				if !ok {
					ti.logger.Debug("proxy channel closed", logger.Int("total_received", receivedCount))
					return
				}
				// Increment the trades received counter
				metrics.TradesReceivedTotal.Inc()
				receivedCount++

				// Only log every 1000 trades to avoid excessive logging
				if receivedCount%1000 == 0 {
					ti.logger.Info("trade processing milestone",
						logger.Int("trades_received", receivedCount),
						logger.String("symbol", trade.Symbol))
				} else {
					ti.logger.Debug("trade received",
						logger.String("symbol", trade.Symbol),
						logger.Float64("price", trade.Price),
						logger.Int("count", receivedCount))
				}

				// Forward the trade
				rawTradesChan <- trade
			}
		}
	}()

	ti.logger.Info("subscribing to market data", logger.Any("symbols", ti.cfg.Symbols))
	err := ti.client.SubscribeToSymbols(ctx, proxyChan, ti.cfg.Symbols)
	if err != nil {
		ti.logger.Error("failed to subscribe to market data",
			logger.Error(err),
			logger.Any("symbols", ti.cfg.Symbols))
		return err
	}
	ti.logger.Info("successfully subscribed to market data")

	<-ctx.Done()
	ti.logger.Info("ingestor received shutdown signal")

	return nil
}

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
	defer func() {
		close(rawTradesChan)
		ti.logger.Info("rawTradesChan closed")
	}()

	// Create a proxy channel to count incoming trades
	proxyChan := make(chan marketdata.Trade, ti.cfg.RawTradesChanBuff)

	// Start a goroutine to forward trades and increment metrics
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case trade, ok := <-proxyChan:
				if !ok {
					return
				}
				// Increment the trades received counter
				metrics.TradesReceivedTotal.Inc()
				// Forward the trade
				rawTradesChan <- trade
			}
		}
	}()

	err := ti.client.SubscribeToSymbols(ctx, proxyChan, ti.cfg.Symbols)
	if err != nil {
		return err
	}

	<-ctx.Done()
	ti.logger.Info("Ingestor context cancelled, stopping...")

	return nil
}

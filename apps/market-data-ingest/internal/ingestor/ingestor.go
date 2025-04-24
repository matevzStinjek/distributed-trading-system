package ingestor

import (
	"context"
	"log/slog"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/interfaces"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type TradeIngestor struct {
	client interfaces.MarketDataClient
	cfg    *config.Config
	logger *slog.Logger
}

func NewTradeIngestor(
	client interfaces.MarketDataClient,
	cfg *config.Config,
	logger *slog.Logger,
) *TradeIngestor {
	return &TradeIngestor{
		client: client,
		cfg:    cfg,
		logger: logger,
	}
}

func (ti *TradeIngestor) Start(
	ctx context.Context,
	rawTradesChan chan<- marketdata.Trade,
) error {
	defer func() {
		close(rawTradesChan)
		ti.logger.Info("rawTradesChan closed")
	}()

	err := ti.client.SubscribeToSymbols(ctx, rawTradesChan, ti.cfg.Symbols)
	if err != nil {
		return err
	}

	<-ctx.Done()
	ti.logger.Info("Ingestor context cancelled, stopping...")

	return nil
}

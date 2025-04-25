package alpaca

import (
	"context"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type AlpacaClient struct {
	client  *stream.StocksClient
	symbols []string
	logger  *logger.Logger
}

func NewAlpacaClient(ctx context.Context, cfg *config.Config, log *logger.Logger) (*AlpacaClient, error) {
	client := stream.NewStocksClient("iex")
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}

	return &AlpacaClient{
		client:  client,
		symbols: cfg.Symbols,
		logger:  log,
	}, nil
}

func (mc *AlpacaClient) SubscribeToSymbols(ctx context.Context, tradeChan chan<- marketdata.Trade, symbols []string) error {
	return mc.client.SubscribeToTrades(func(t stream.Trade) {
		trade := marketdata.Trade{
			ID:        t.ID,
			Symbol:    t.Symbol,
			Price:     t.Price,
			Size:      t.Size,
			Timestamp: t.Timestamp,
		}
		select {
		case tradeChan <- trade:
		case <-ctx.Done():
			return
		default:
			mc.logger.Warn("tradeChan full, dropping trade", logger.Any("trade", trade))
		}
	}, symbols...)
}

func (mc *AlpacaClient) Close() error {
	return mc.client.UnsubscribeFromTrades(mc.symbols...)
}

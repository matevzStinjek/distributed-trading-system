package provider

import (
	"context"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type AlpacaClient struct {
	client  *stream.StocksClient
	symbols []string
}

func (mc *AlpacaClient) SubscribeToTrades(handler func(t marketdata.Trade)) error {
	return mc.client.SubscribeToTrades(func(t stream.Trade) {
		handler(marketdata.Trade{
			ID:        t.ID,
			Symbol:    t.Symbol,
			Price:     t.Price,
			Size:      t.Size,
			Timestamp: t.Timestamp,
		})
	}, mc.symbols...)
}

func (mc *AlpacaClient) UnsubscribeFromTrades() error {
	return mc.client.UnsubscribeFromTrades(mc.symbols...)
}

func NewAlpacaClient(ctx context.Context, cfg *config.Config) (*AlpacaClient, error) {
	client := stream.NewStocksClient("iex")
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}

	return &AlpacaClient{
		client:  client,
		symbols: cfg.Symbols,
	}, nil
}

package provider

import (
	"context"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type AlpacaClient struct {
	client *stream.StocksClient
}

func (mc *AlpacaClient) SubscribeToTrades(handler func(t marketdata.Trade), symbols []string) {
	mc.client.SubscribeToTrades(func(t stream.Trade) {
		handler(marketdata.Trade{
			ID:        t.ID,
			Symbol:    t.Symbol,
			Price:     t.Price,
			Size:      t.Size,
			Timestamp: t.Timestamp,
		})
	}, symbols...)
}

func NewAlpacaClient(ctx context.Context) (*AlpacaClient, error) {
	client := stream.NewStocksClient("iex")
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}

	return &AlpacaClient{
		client: client,
	}, nil
}

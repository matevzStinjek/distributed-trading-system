package provider

import (
	"context"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
)

type MarketClient struct {
	client *stream.StocksClient
}

func (mc *MarketClient) SubscribeToTrades(handler func(t stream.Trade), symbols []string) {
	mc.client.SubscribeToTrades(handler, symbols...)
}

func NewMarketClient(ctx context.Context) (*MarketClient, error) {
	client := stream.NewStocksClient("iex")
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}

	return &MarketClient{
		client: client,
	}, nil
}

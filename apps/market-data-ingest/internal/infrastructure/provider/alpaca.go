package provider

import (
	"context"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
)

type AlpacaClient struct {
	client *stream.StocksClient
}

func (mc *AlpacaClient) SubscribeToTrades(handler func(t stream.Trade), symbols []string) {
	mc.client.SubscribeToTrades(handler, symbols...)
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

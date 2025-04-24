package interfaces

import (
	"context"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type CacheClient interface {
	Set(context.Context, string, any, time.Duration) error
	Close() error
}

type PubsubClient interface {
	Publish(context.Context, string, any) error
	Close() error
}

type MarketDataClient interface {
	SubscribeToSymbols(context.Context, chan<- marketdata.Trade, []string) error
	Close() error
}

type KafkaProducer interface {
	Produce(ctx context.Context, trade marketdata.Trade) (partition int32, offset int64, err error)
	Close() error
}

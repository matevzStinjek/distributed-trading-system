package processor

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/infrastructure/kafka"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/infrastructure/redis"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type TradeProcessor struct {
	cacheClient    *redis.RedisClient
	pubsubClient   *redis.RedisClient
	producerClient *kafka.SaramaAsyncProducer
}

func (tp *TradeProcessor) processTrade(t marketdata.Trade) error {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// publish to cache
	key := fmt.Sprintf("price:%s", t.Symbol)
	err := tp.cacheClient.Client.Set(ctx, key, t.Price, 0).Err()
	if err != nil {
		log.Printf("cache update failed: %v", err)
	}

	// publish to pubsub
	err = tp.pubsubClient.Client.Publish(ctx, key, t.Price).Err()
	if err != nil {
		log.Printf("price publish failed: %v", err)
	}

	// publish to kafka
	err = tp.producerClient.Produce(t)
	if err != nil {
		log.Printf("produce failed: %v", err)
	}

	log.Printf("%v", t)
	return nil
}

func (tp *TradeProcessor) Start(ctx context.Context, tradeChannel <-chan marketdata.Trade) {
	for {
		select {
		case trade, ok := <-tradeChannel:
			log.Printf("OK: %t", ok)
			if !ok {
			}
			tp.processTrade(trade)
		case <-ctx.Done():
			return
		}
	}
}

func NewTradeProcessor(
	cacheClient *redis.RedisClient,
	pubsubClient *redis.RedisClient,
	producerClient *kafka.SaramaAsyncProducer,
) *TradeProcessor {
	return &TradeProcessor{
		cacheClient,
		pubsubClient,
		producerClient,
	}
}

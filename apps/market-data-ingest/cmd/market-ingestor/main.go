package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/infrastructure/kafka"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/infrastructure/redis"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type TradeProcessor struct {
	cfg            *config.Config
	cacheClient    *redis.RedisClient
	pubsubClient   *redis.RedisClient
	saramaProducer *kafka.SaramaAsyncProducer
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
	bytes, err := json.Marshal(t)
	if err != nil {
		log.Printf("could not marshall trade object: %v", err)
	} else {
		message := &sarama.ProducerMessage{
			Topic:     tp.cfg.KafkaTopicMarketData,
			Key:       sarama.StringEncoder(t.Symbol),
			Value:     sarama.ByteEncoder(bytes),
			Timestamp: t.Timestamp,
		}

		tp.saramaProducer.Producer.Input() <- message
	}

	log.Printf("%v", t)
	return nil
}

func (tp *TradeProcessor) ConsumeTrades(ctx context.Context, tradeChannel <-chan marketdata.Trade) {
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

func main() {
	go func() {
		http.ListenAndServe(":6060", nil)
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	client := stream.NewStocksClient("iex")

	err = client.Connect(ctx)
	if err != nil {
		log.Fatalf("error connecting to stocks client: %v", err)
	}

	cacheClient, err := redis.NewRedisCacheClient(ctx, cfg)
	if err != nil {
		log.Fatalf("error pinging redis cache instance: %v", err)
	}

	pubsubClient, err := redis.NewRedisPubsubClient(ctx, cfg)
	if err != nil {
		log.Fatalf("error pinging redis pubsub instance: %v", err)
	}

	saramaProducer, err := kafka.NewKafkaAsyncProducer(cfg)
	if err != nil {
		log.Fatalf("error creating sarama async producer: %v", err)
	}

	processor := TradeProcessor{
		cfg:            cfg,
		cacheClient:    cacheClient,
		pubsubClient:   pubsubClient,
		saramaProducer: saramaProducer,
	}

	tradeChannel := make(chan marketdata.Trade, cfg.TradeChannelBuff)

	var consumeWg sync.WaitGroup
	consumeWg.Add(1)
	go func() {
		defer consumeWg.Done()
		processor.ConsumeTrades(ctx, tradeChannel)
	}()

	client.SubscribeToTrades(func(t stream.Trade) {
		tradeChannel <- marketdata.Trade{
			ID:        t.ID,
			Symbol:    t.Symbol,
			Price:     t.Price,
			Size:      t.Size,
			Timestamp: t.Timestamp,
		}
	})

	tradeChannel <- marketdata.Trade{
		ID:        1,
		Symbol:    "MSFT",
		Price:     1,
		Size:      1,
		Timestamp: time.Now(),
	}
	tradeChannel <- marketdata.Trade{
		ID:        2,
		Symbol:    "MSFT",
		Price:     1,
		Size:      1,
		Timestamp: time.Now(),
	}

	<-ctx.Done()
	consumeWg.Done()
}

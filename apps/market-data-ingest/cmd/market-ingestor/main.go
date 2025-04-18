package main

import (
	"context"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/infrastructure/kafka"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/infrastructure/provider"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/infrastructure/redis"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/processor"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

func main() {
	go func() {
		http.ListenAndServe(":6060", nil)
	}()

	// setup context and config
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	// setup clients
	cacheClient, err := redis.NewRedisCacheClient(ctx, cfg)
	if err != nil {
		log.Fatalf("error pinging redis cache instance: %v", err)
	}
	defer cacheClient.Client.Close()

	pubsubClient, err := redis.NewRedisPubsubClient(ctx, cfg)
	if err != nil {
		log.Fatalf("error pinging redis pubsub instance: %v", err)
	}
	defer pubsubClient.Client.Close()

	saramaProducer, err := kafka.NewKafkaAsyncProducer(cfg)
	if err != nil {
		log.Fatalf("error creating sarama async producer: %v", err)
	}
	defer saramaProducer.Producer.Close()

	// setup tradeProcessor, channel, and start consuming channel
	tradeProcessor := processor.NewTradeProcessor(cacheClient, pubsubClient, saramaProducer)

	tradeChannel := make(chan marketdata.Trade, cfg.TradeChannelBuff)

	var consumeWg sync.WaitGroup
	consumeWg.Add(1)
	go func() {
		defer consumeWg.Done()
		tradeProcessor.Start(ctx, tradeChannel)
	}()

	// setup stocks client
	alpaca, err := provider.NewAlpacaClient(ctx, cfg)
	if err != nil {
		log.Fatalf("error connecting to stocks client: %v", err)
	}

	alpaca.SubscribeToTrades(func(t marketdata.Trade) {
		tradeChannel <- t
	})

	// mock trades
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
	if err = alpaca.UnsubscribeFromTrades(); err != nil {
		log.Printf("failed to unsubscribe from alpaca trades: %v", err)
	}

	close(tradeChannel)
	consumeWg.Done()
}

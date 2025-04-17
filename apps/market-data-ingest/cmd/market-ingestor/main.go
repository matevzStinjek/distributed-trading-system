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
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
)

func processTrade(t marketdata.Trade) error {
	// publish to cache
	// publish to pubsub
	// publish to kafka
	log.Printf("%v", t)
	return nil
}

func ConsumeTrades(ctx context.Context, tradeChannel <-chan marketdata.Trade) {
	for {
		select {
		case trade, ok := <-tradeChannel:
			log.Printf("OK: %t", ok)
			if !ok {
			}
			processTrade(trade)
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

	tradeChannel := make(chan marketdata.Trade, cfg.TradeChannelBuff)

	var consumeWg sync.WaitGroup
	consumeWg.Add(1)
	go func() {
		defer consumeWg.Done()
		ConsumeTrades(ctx, tradeChannel)
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
		Symbol:    "AAPL",
		Price:     1,
		Size:      1,
		Timestamp: time.Now(),
	}
	tradeChannel <- marketdata.Trade{
		ID:        2,
		Symbol:    "AAPL",
		Price:     1,
		Size:      1,
		Timestamp: time.Now(),
	}

	<-ctx.Done()
	consumeWg.Done()
}

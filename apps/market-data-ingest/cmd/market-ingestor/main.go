package main

import (
	"context"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
)

// =============
// Configuration
// =============

type Config struct {
	symbols []string
}

func loadConfig() (*Config, error) {
	cfg := &Config{
		symbols: []string{"AAPL", "MSFT", "GOOG", "AMZN", "TSLA"},
	}
	return cfg, nil
}

type TradeProcessor struct {
	tradesChannel chan marketdata.Trade
}

func (tp *TradeProcessor) recordTrade(t stream.Trade) {
	tp.tradesChannel <- marketdata.Trade{
		ID:        t.ID,
		Symbol:    t.Symbol,
		Price:     t.Price,
		Size:      t.Size,
		Timestamp: t.Timestamp,
	}
}

func (tp *TradeProcessor) processTrade(t marketdata.Trade) {
	log.Printf("%s $%f (chan size: %d)", t.Symbol, t.Price, len(tp.tradesChannel))

}

func (tp *TradeProcessor) Start(ctx context.Context, tradeChan <-chan marketdata.Trade) {
	for {
		select {
		case trade := <-tp.tradesChannel:
			tp.processTrade(trade)
		case <-ctx.Done():
			return
		}
	}
}

func NewTradeProcessor() *TradeProcessor {
	tradesChannel := make(chan marketdata.Trade)
	return &TradeProcessor{
		tradesChannel,
	}
}

type MarketDataClient struct {
	client *stream.StocksClient
}

func (mc *MarketDataClient) Connect(ctx context.Context) error {
	return mc.client.Connect(ctx)
}

func (mc *MarketDataClient) SubscribeToTrades(handler func(stream.Trade), symbols []string) error {
	return mc.client.SubscribeToTrades(handler, symbols...)
}

func (mc *MarketDataClient) UnsubscribeFromTrades(symbols []string) error {
	return mc.client.UnsubscribeFromTrades(symbols...)
}

func NewMarketDataClient() *MarketDataClient {
	return &MarketDataClient{
		client: stream.NewStocksClient("iex"),
	}
}

func main() {
	// --- Load config ---
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalln(err)
	}

	// --- Start pprof server ---
	pprof := &http.Server{Addr: ":6060"}
	go func() {
		log.Println("Starting pprof server on :6060")
		if err := pprof.ListenAndServe(); err != nil {
			log.Printf("pprof server err: %v", err)
		}
	}()

	// --- Setup context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := NewMarketDataClient()

	if err := client.Connect(ctx); err != nil {
		log.Fatalln(err)
	}

	tp := NewTradeProcessor()

	if err := client.SubscribeToTrades(tp.recordTrade, cfg.symbols); err != nil {
		log.Fatalln(err)
	}

	log.Println("Waiting for messages or interrupt signal...")

	<-ctx.Done()

	log.Println("SIGINT received, closing connections and shutting down")
	if err := client.UnsubscribeFromTrades(cfg.symbols); err != nil {
		log.Fatalln(err)
	}
	log.Println("Unsubscrubed, closing")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := pprof.Shutdown(shutdownCtx); err != nil {
		log.Printf("pprof shutdown error: %v", err)
	}
}

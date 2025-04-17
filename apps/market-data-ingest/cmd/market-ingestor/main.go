package main

import (
	"context"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
)

func main() {
	// Start pprof server
	go func() {
		log.Println("Starting pprof server on :6060")
		log.Println(http.ListenAndServe(":6060", nil))
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	symbols := []string{"AAPL", "MSFT", "GOOG", "AMZN", "TSLA"}

	client := stream.NewStocksClient("iex")

	if err := client.Connect(ctx); err != nil {
		log.Fatalln(err)
	}

	if err := client.SubscribeToTrades(func(t stream.Trade) {
		log.Println(t.Symbol, t.Price, t.Exchange)
	}, symbols...); err != nil {
		log.Fatalln(err)
	}

	log.Println("Waiting for messages or interrupt signal...")

	<-ctx.Done()

	log.Println("SIGTERM received, closing connections and shutting down")
	if err := client.UnsubscribeFromTrades(symbols...); err != nil {
		log.Fatalln(err)
	}
	log.Println("Unsubscrubed, closing")
}

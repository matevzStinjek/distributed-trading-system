package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/aggregator"
	appConfig "github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	alpacaInfra "github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/infrastructure/alpaca"
	kafkaInfra "github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/infrastructure/kafka"
	redisInfra "github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/infrastructure/redis"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/processor"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
	"golang.org/x/sync/errgroup"
)

func run(
	ctx context.Context,
	getenv func(string) string,
	logger *slog.Logger,
) error {
	// --- Setup context and config ---
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Info("loading configuration")
	cfg, err := appConfig.LoadConfig(getenv, logger)
	if err != nil {
		return err
	}

	// --- Pprof ---
	go func() {
		logger.Info("Starting pprof server")
		srv := &http.Server{Addr: "6060"}

		go func() {
			<-ctx.Done()
			srv.Shutdown(context.Background())
		}()

		if err := srv.ListenAndServe(); err != nil {
			logger.Warn("Pprof server error", slog.Any("error", err))
		}
	}()

	// --- Init infra clients ---
	logger.Info("connecting to redis cache")
	cacheClient, err := redisInfra.NewRedisCacheClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("couldnt init cache client: %w", err)
	}
	defer cacheClient.Close()

	logger.Info("connecting to redis pubsub")
	pubsubClient, err := redisInfra.NewRedisPubsubClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("couldnt init pubsub client: %w", err)
	}
	defer pubsubClient.Close()

	logger.Info("producer connecting to kafka")
	saramaProducer, err := kafkaInfra.NewKafkaAsyncProducer(cfg)
	if err != nil {
		return err
	}
	defer saramaProducer.Producer.Close()

	// --- Setup channels ---
	rawTradeChan := make(chan marketdata.Trade, cfg.RawTradesChanBuff)
	processedTradesChan := make(chan marketdata.Trade, cfg.ProcTradesChanBuff)
	backgroundTradesChan := make(chan marketdata.Trade, cfg.KafkaChanBuff)

	// --- Setup core components
	aggregator := aggregator.NewTradeAggregator(cfg, logger.With("component", "aggregator"))
	processor := processor.NewTradeProcessor(cacheClient, pubsubClient, logger.With("component", "processor"))

	// --- Start core components
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		err := aggregator.Start(ctx, rawTradeChan, processedTradesChan)
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("aggregator error: %w", err)
		}
		return err
	})

	g.Go(func() error {
		err := processor.Start(ctx, processedTradesChan, backgroundTradesChan)
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("processor error: %w", err)
		}
		return err
	})

	// --- Setup stocks client ---
	logger.Info("connecting to alpaca")
	alpaca, err := alpacaInfra.NewAlpacaClient(ctx, cfg)
	if err != nil {
		return err
	}

	err = alpaca.SubscribeToTrades(func(t marketdata.Trade) {
		logger.Debug("trade received", slog.Any("trade", t))
		rawTradeChan <- t
	})
	if err != nil {
		return err
	}

	// mock trades
	rawTradeChan <- marketdata.Trade{
		ID:        1,
		Symbol:    "MSFT",
		Price:     1,
		Size:      1,
		Timestamp: time.Now(),
	}
	rawTradeChan <- marketdata.Trade{
		ID:        2,
		Symbol:    "MSFT",
		Price:     2,
		Size:      1,
		Timestamp: time.Now(),
	}

	if err = g.Wait(); err != nil {
		logger.Error("goroutine error", slog.Any("error", err))
	} else {
		logger.Info("received shutdown signal")
	}

	if err = alpaca.UnsubscribeFromTrades(); err != nil {
		logger.Warn("failed to unsubscribe from alpaca trades", slog.Any("error", err))
	} else {
		logger.Info("unsubscribed from alpaca trades")
	}

	close(rawTradeChan)
	logger.Info("rawTradeChan closed")

	return err
}

func main() {
	ctx := context.Background()

	logger := getLoggerFromEnv()

	if err := run(ctx, os.Getenv, logger); err != nil {
		logger.Error("application error", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("application shut down gracefully")
}

// util
func getLoggerFromEnv() *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "error":
		logLevel = slog.LevelError
	case "warn":
		logLevel = slog.LevelWarn
	case "debug":
		logLevel = slog.LevelDebug
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	switch strings.ToLower(os.Getenv("LOG_FORMAT")) {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	default:
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	return slog.New(handler)
}

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/aggregator"
	appConfig "github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	alpacaInfra "github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/infrastructure/alpaca"
	kafkaInfra "github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/infrastructure/kafka"
	redisInfra "github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/infrastructure/redis"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/ingestor"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/metrics"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/processor"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/producer"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

func run(
	ctx context.Context,
	getenv func(string) string,
	log *logger.Logger,
) error {
	// --- Setup context and config ---
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info("loading configuration")
	cfg, err := appConfig.LoadConfig(getenv, log)
	if err != nil {
		return err
	}

	// --- Start runtime metrics collector ---
	log.Info("starting metrics collector")
	stopMetricsCollector := metrics.StartRuntimeMetricsCollector(ctx, 15*time.Second)
	defer stopMetricsCollector()

	// --- Setup HTTP server for pprof and metrics ---
	mux := http.NewServeMux()

	// Register pprof handlers (they register with http.DefaultServeMux)
	// Register Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	go func() {
		log.Info("Starting HTTP server for pprof and metrics", logger.String("addr", ":6060"))
		srv := &http.Server{
			Addr:    ":6060",
			Handler: mux,
		}

		go func() {
			<-ctx.Done()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			srv.Shutdown(shutdownCtx)
		}()

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warn("HTTP server error", logger.Error(err))
		}
	}()

	// --- Init infra clients ---
	log.Info("connecting to redis cache")
	cacheClient, err := redisInfra.NewRedisCacheClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("couldnt init cache client: %w", err)
	}
	defer cacheClient.Close()

	log.Info("connecting to redis pubsub")
	pubsubClient, err := redisInfra.NewRedisPubsubClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("couldnt init pubsub client: %w", err)
	}
	defer pubsubClient.Close()

	log.Info("producer connecting to kafka")
	saramaProducer, err := kafkaInfra.NewKafkaAsyncProducer(cfg, log.Component("sarama"))
	if err != nil {
		return err
	}
	defer saramaProducer.Close()

	// --- Setup stocks client ---
	log.Info("connecting to alpaca")
	alpaca, err := alpacaInfra.NewAlpacaClient(ctx, cfg, log.Component("alpaca"))
	if err != nil {
		return err
	}
	defer alpaca.Close()

	// --- Setup channels ---
	rawTradeChan := make(chan marketdata.Trade, cfg.RawTradesChanBuff)
	processedTradesChan := make(chan marketdata.Trade, cfg.ProcTradesChanBuff)
	backgroundTradesChan := make(chan marketdata.Trade, cfg.KafkaChanBuff)

	// Set channel capacity metrics
	metrics.RawTradesChanCapacity.Set(float64(cfg.RawTradesChanBuff))
	metrics.ProcessedTradesChanCapacity.Set(float64(cfg.ProcTradesChanBuff))
	metrics.KafkaChanCapacity.Set(float64(cfg.KafkaChanBuff))

	// --- Setup core components
	ingestor := ingestor.NewTradeIngestor(alpaca, cfg, log.Component("ingestor"))
	aggregator := aggregator.NewTradeAggregator(cfg, log.Component("aggregator"))
	processor := processor.NewTradeProcessor(cacheClient, pubsubClient, log.Component("processor"))
	worker := producer.NewKafkaWorker(saramaProducer, log.Component("worker"))

	// --- Start channel metrics collector ---
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Update channel metrics
				metrics.RawTradesChanSize.Set(float64(len(rawTradeChan)))
				metrics.ProcessedTradesChanSize.Set(float64(len(processedTradesChan)))
				metrics.KafkaChanSize.Set(float64(len(backgroundTradesChan)))
			}
		}
	}()

	// --- Start core components
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		err := worker.Start(ctx, backgroundTradesChan)
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("worker error: %w", err)
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

	g.Go(func() error {
		err := aggregator.Start(ctx, rawTradeChan, processedTradesChan)
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("aggregator error: %w", err)
		}
		return err
	})

	g.Go(func() error {
		err := ingestor.Start(ctx, rawTradeChan)
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("ingestor error: %w", err)
		}
		return err
	})

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
		log.Error("goroutine error", logger.Error(err))
	} else {
		log.Info("received shutdown signal")
	}

	return err
}

func main() {
	ctx := context.Background()

	// Initialize logger
	log := logger.New()
	defer log.Sync()

	if err := run(ctx, os.Getenv, log); err != nil {
		log.Error("application error", logger.Error(err))
		os.Exit(1)
	}
	log.Info("application shut down gracefully")
}

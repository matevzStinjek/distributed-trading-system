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
	log.Info("starting market data ingest service")

	// --- Setup context and config ---
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info("loading configuration")
	cfg, err := appConfig.LoadConfig(getenv, log)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	log.Info("configuration loaded successfully")

	// --- Start runtime metrics collector ---
	log.Debug("starting runtime metrics collector")
	stopMetricsCollector := metrics.StartRuntimeMetricsCollector(ctx, 15*time.Second)
	defer stopMetricsCollector()

	// --- Setup HTTP server for pprof and metrics ---
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	httpSrv := &http.Server{
		Addr:    ":6060",
		Handler: mux,
	}

	go func() {
		log.Info("starting metrics and profiling server", logger.String("addr", httpSrv.Addr))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("metrics server failed", logger.Error(err))
		}
	}()

	// Add graceful shutdown for metrics server
	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		log.Debug("shutting down metrics server")
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Warn("metrics server shutdown error", logger.Error(err))
		}
	}()

	// --- Init infrastructure clients ---
	// Redis cache client
	log.Info("connecting to redis cache")
	cacheClient, err := redisInfra.NewRedisCacheClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize redis cache client: %w", err)
	}
	defer func() {
		log.Debug("closing redis cache connection")
		cacheClient.Close()
	}()
	log.Info("redis cache connection established")

	// Redis pubsub client
	log.Info("connecting to redis pubsub")
	pubsubClient, err := redisInfra.NewRedisPubsubClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize redis pubsub client: %w", err)
	}
	defer func() {
		log.Debug("closing redis pubsub connection")
		pubsubClient.Close()
	}()
	log.Info("redis pubsub connection established")

	// Kafka producer
	log.Info("connecting to kafka", logger.Any("brokers", cfg.KafkaBrokers))
	saramaProducer, err := kafkaInfra.NewKafkaAsyncProducer(cfg, log.Component("kafka"))
	if err != nil {
		return fmt.Errorf("failed to initialize kafka producer: %w", err)
	}
	defer func() {
		log.Debug("closing kafka producer")
		saramaProducer.Close()
	}()
	log.Info("kafka producer initialized", logger.String("topic", cfg.KafkaTopicMarketData))

	// --- Market data client ---
	log.Info("connecting to market data provider")
	alpaca, err := alpacaInfra.NewAlpacaClient(ctx, cfg, log.Component("alpaca"))
	if err != nil {
		return fmt.Errorf("failed to initialize market data client: %w", err)
	}
	defer func() {
		log.Debug("closing market data client")
		alpaca.Close()
	}()
	log.Info("market data client connected", logger.Any("symbols", cfg.Symbols))

	// --- Setup channels ---
	log.Debug("initializing data channels",
		logger.Int("raw_buffer", cfg.RawTradesChanBuff),
		logger.Int("processed_buffer", cfg.ProcTradesChanBuff),
		logger.Int("kafka_buffer", cfg.KafkaChanBuff))

	rawTradeChan := make(chan marketdata.Trade, cfg.RawTradesChanBuff)
	processedTradesChan := make(chan marketdata.Trade, cfg.ProcTradesChanBuff)
	backgroundTradesChan := make(chan marketdata.Trade, cfg.KafkaChanBuff)

	// Set channel capacity metrics
	metrics.RawTradesChanCapacity.Set(float64(cfg.RawTradesChanBuff))
	metrics.ProcessedTradesChanCapacity.Set(float64(cfg.ProcTradesChanBuff))
	metrics.KafkaChanCapacity.Set(float64(cfg.KafkaChanBuff))

	// --- Setup core components
	log.Info("initializing core components")
	ingestorSvc := ingestor.NewTradeIngestor(alpaca, cfg, log.Component("ingestor"))
	aggregatorSvc := aggregator.NewTradeAggregator(cfg, log.Component("aggregator"))
	processorSvc := processor.NewTradeProcessor(cacheClient, pubsubClient, log.Component("processor"))
	workerSvc := producer.NewKafkaWorker(saramaProducer, log.Component("worker"))

	// --- Start channel metrics collector ---
	log.Debug("starting channel metrics collector")
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
	log.Info("starting processing pipeline")
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Info("starting kafka worker")
		err := workerSvc.Start(ctx, backgroundTradesChan)
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("worker error: %w", err)
		}
		log.Info("kafka worker stopped")
		return nil
	})

	g.Go(func() error {
		log.Info("starting trade processor")
		err := processorSvc.Start(ctx, processedTradesChan, backgroundTradesChan)
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("processor error: %w", err)
		}
		log.Info("trade processor stopped")
		return nil
	})

	g.Go(func() error {
		log.Info("starting trade aggregator")
		err := aggregatorSvc.Start(ctx, rawTradeChan, processedTradesChan)
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("aggregator error: %w", err)
		}
		log.Info("trade aggregator stopped")
		return nil
	})

	g.Go(func() error {
		log.Info("starting trade ingestor")
		err := ingestorSvc.Start(ctx, rawTradeChan)
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("ingestor error: %w", err)
		}
		log.Info("trade ingestor stopped")
		return nil
	})

	// For testing and development only - mock trades
	if getenv("ENV") == "development" || getenv("MOCK_TRADES") == "true" {
		log.Info("injecting mock trades for testing")
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
	}

	// Wait for processing to complete or context to be canceled
	if err = g.Wait(); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Info("processing pipeline stopped due to context cancellation")
		} else {
			log.Error("pipeline error", logger.Error(err))
			return err
		}
	}

	log.Info("market data ingest service shutting down gracefully")
	return nil
}

func main() {
	ctx := context.Background()

	// Initialize logger
	log := logger.New()
	defer log.Sync()

	if err := run(ctx, os.Getenv, log); err != nil {
		log.Error("fatal error",
			logger.Error(err),
			logger.String("service", "market-data-ingest"))
		os.Exit(1)
	}
}

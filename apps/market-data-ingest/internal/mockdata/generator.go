package mockdata

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/logger"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/metrics"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type Config struct {
	Symbols             []string
	BaseTradesPerSecond int
	MaxTradesPerSecond  int
	SpikeInterval       int
	SpikeDuration       int
	PriceVolatility     float64
	SizeVariability     float64
	EnableJitter        bool
}

func DefaultConfig() Config {
	return Config{
		Symbols:             []string{"AAPL", "MSFT", "GOOG", "AMZN", "TSLA", "META", "NVDA", "NFLX"},
		BaseTradesPerSecond: 50,
		MaxTradesPerSecond:  500,
		SpikeInterval:       60,
		SpikeDuration:       10,
		PriceVolatility:     0.05,
		SizeVariability:     0.7,
		EnableJitter:        true,
	}
}

type Generator struct {
	config  Config
	logger  *logger.Logger
	nextID  int64
	prices  map[string]float64
	started time.Time
	mu      sync.Mutex
}

func NewGenerator(config Config, log *logger.Logger) *Generator {
	initialPrices := make(map[string]float64, len(config.Symbols))

	basePrices := map[string]float64{
		"AAPL": 180.25, "MSFT": 325.5, "GOOG": 142.7, "AMZN": 178.3,
		"TSLA": 237.45, "META": 324.6, "NVDA": 880.3, "NFLX": 600.75,
		"IBM": 174.2, "INTC": 43.8, "AMD": 172.5, "ORCL": 120.4,
		"CSCO": 45.7, "ADBE": 475.3, "CRM": 272.8, "QCOM": 186.7,
	}

	for _, symbol := range config.Symbols {
		if price, exists := basePrices[symbol]; exists {
			initialPrices[symbol] = price
		} else {
			// For symbols without predefined prices, generate a random price
			initialPrices[symbol] = 50.0 + rand.Float64()*450.0
		}
	}

	return &Generator{
		config:  config,
		logger:  log,
		nextID:  1,
		prices:  initialPrices,
		started: time.Now(),
	}
}

func (g *Generator) Start(ctx context.Context, tradeChan chan<- marketdata.Trade) error {
	g.logger.Info("starting mock data generator",
		logger.Int("base_trades_per_sec", g.config.BaseTradesPerSecond),
		logger.Int("max_trades_per_sec", g.config.MaxTradesPerSecond),
		logger.Any("symbols", g.config.Symbols),
		logger.Int("spike_interval_sec", g.config.SpikeInterval),
		logger.Int("spike_duration_sec", g.config.SpikeDuration))

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	var (
		tradesGenerated int
		lastStatsTime   = time.Now()
		inSpike         = false
		spikeStarted    time.Time
		targetRate      = g.config.BaseTradesPerSecond
	)

	// Start a separate goroutine for managing spikes
	go func() {
		spikeTicker := time.NewTicker(time.Duration(g.config.SpikeInterval) * time.Second)
		defer spikeTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-spikeTicker.C:
				if !inSpike {
					g.mu.Lock()
					inSpike = true
					spikeStarted = time.Now()
					targetRate = g.config.MaxTradesPerSecond
					g.mu.Unlock()

					metrics.MockTrafficSpikeActive.Set(1) // Set spike active indicator

					g.logger.Info("traffic spike started",
						logger.Int("target_rate", targetRate),
						logger.Duration("planned_duration", time.Duration(g.config.SpikeDuration)*time.Second))

					// Set timer to end the spike
					time.AfterFunc(time.Duration(g.config.SpikeDuration)*time.Second, func() {
						g.mu.Lock()
						inSpike = false
						targetRate = g.config.BaseTradesPerSecond
						spikeDuration := time.Since(spikeStarted)
						g.mu.Unlock()

						metrics.MockTrafficSpikeActive.Set(0) // Reset spike active indicator

						g.logger.Info("traffic spike ended",
							logger.Int("target_rate", targetRate),
							logger.Duration("actual_duration", spikeDuration))
					})
				}
			}
		}
	}()

	// Track how many trades we should generate in the current window
	// to achieve the target rate
	tradesPerTick := 0
	accumulatedTradeDebt := 0.0

	// Main generation loop
	for {
		select {
		case <-ctx.Done():
			g.logger.Info("mock data generator shutting down",
				logger.Int("total_trades_generated", tradesGenerated))
			return nil

		case <-ticker.C:
			// Calculate how many trades to generate in this tick to meet the target rate
			g.mu.Lock()
			currentTargetRate := targetRate
			g.mu.Unlock()

			// Determine trades per tick (10ms tick = 100 ticks per second)
			tradesPerTick = 0
			targetTradesPerTick := float64(currentTargetRate) / 100.0

			// Account for fractional trades using debt system
			accumulatedTradeDebt += targetTradesPerTick
			if accumulatedTradeDebt >= 1.0 {
				tradesPerTick = int(accumulatedTradeDebt)
				accumulatedTradeDebt -= float64(tradesPerTick)
			}

			// Add jitter if enabled (±20% variation)
			if g.config.EnableJitter && tradesPerTick > 0 {
				jitterFactor := 0.8 + rand.Float64()*0.4 // 0.8-1.2
				adjustedTrades := float64(tradesPerTick) * jitterFactor
				tradesPerTick = int(math.Max(1, adjustedTrades))
			}

			// Generate trades
			for i := 0; i < tradesPerTick; i++ {
				trade := g.generateTrade()

				select {
				case tradeChan <- trade:
					tradesGenerated++
					metrics.MockTradesGeneratedTotal.Inc()
					metrics.MockTradesReceivedTotal.Inc()
				default:
					// Channel full, drop the trade
					metrics.MockTradesDroppedTotal.Inc()
					g.logger.Warn("trade channel full, dropping mock trade",
						logger.String("symbol", trade.Symbol))
				}
			}

			// Log statistics periodically
			if time.Since(lastStatsTime) >= 10*time.Second {
				tradesPerSecond := float64(tradesGenerated) / time.Since(lastStatsTime).Seconds()
				metrics.MockTradeRateGauge.Set(tradesPerSecond)
				g.logStats(tradesGenerated, lastStatsTime)
				lastStatsTime = time.Now()
				tradesGenerated = 0 // Reset counter for the next period
			}
		}
	}
}

// generateTrade creates a single mock trade
func (g *Generator) generateTrade() marketdata.Trade {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Select a random symbol
	symbol := g.config.Symbols[rand.Intn(len(g.config.Symbols))]

	// Get the current price for this symbol
	currentPrice := g.prices[symbol]

	// Calculate price change based on volatility
	// Use a random walk with mean reversion
	priceChange := (rand.Float64()*2 - 1) * g.config.PriceVolatility * currentPrice

	// Add some mean reversion to prevent prices from drifting too far
	basePrice := g.prices[symbol]
	meanReversionFactor := 0.05 // 5% reversion to initial price
	priceChange += (basePrice - currentPrice) * meanReversionFactor

	// Update the price
	newPrice := math.Max(0.01, currentPrice+priceChange)
	g.prices[symbol] = newPrice

	// Generate a variable trade size based on configuration
	baseSize := 100
	if g.config.SizeVariability > 0 {
		// Generate sizes with power-law distribution (more small trades, fewer large trades)
		sizeMultiplier := math.Pow(rand.Float64(), 2)*9 + 1 // 1-10x multiplier, biased toward smaller sizes
		size := int(float64(baseSize) * sizeMultiplier * g.config.SizeVariability)
		baseSize = int(math.Max(1, float64(size)))
	}

	// Add timestamp jitter to simulate slightly out-of-order data
	timestampJitter := time.Duration(0)
	if g.config.EnableJitter {
		// Add up to 100ms of jitter
		timestampJitter = time.Duration(rand.Intn(100)) * time.Millisecond
	}

	// Create the trade
	trade := marketdata.Trade{
		ID:        g.nextID,
		Symbol:    symbol,
		Price:     newPrice,
		Size:      uint32(baseSize),
		Timestamp: time.Now().Add(-timestampJitter),
	}

	g.nextID++
	return trade
}

// logStats logs statistics about the generator
func (g *Generator) logStats(tradesGenerated int, since time.Time) {
	duration := time.Since(since)
	tradesPerSecond := float64(tradesGenerated) / duration.Seconds()

	g.logger.Info("mock data generator statistics",
		logger.Int("trades_generated", tradesGenerated),
		logger.Float64("trades_per_second", tradesPerSecond),
		logger.Duration("period", duration),
		logger.Any("current_prices", g.prices))

	// Add milestone log for dashboard visibility
	g.logger.Info("mock trade processing milestone",
		logger.Int("trades_received", tradesGenerated),
		logger.Float64("trades_per_second", tradesPerSecond))
}

package metrics

import (
	"context"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// StartRuntimeMetricsCollector starts a goroutine that periodically collects and updates runtime metrics.
// It returns a function that can be called to stop the collector.
func StartRuntimeMetricsCollector(ctx context.Context, interval time.Duration) func() {
	if interval <= 0 {
		interval = 15 * time.Second
	}

	// Create a separate context with cancel to control the collector
	collectorCtx, cancel := context.WithCancel(context.Background())

	// Start the collector goroutine
	go collectRuntimeMetrics(collectorCtx, interval)

	// Return a function that can be used to stop the collector
	return func() {
		cancel()
	}
}

// collectRuntimeMetrics periodically updates runtime metrics
func collectRuntimeMetrics(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updateRuntimeMetrics()
		}
	}
}

// updateRuntimeMetrics updates all runtime metrics with current values
func updateRuntimeMetrics() {
	// Update goroutine count
	GoroutinesCount.Set(float64(runtime.NumGoroutine()))

	// Update memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	MemoryAllocBytes.Set(float64(memStats.Alloc))
	HeapAllocBytes.Set(float64(memStats.HeapAlloc))
	HeapObjectsCount.Set(float64(memStats.HeapObjects))
	GCPauseNanosTotal.Set(float64(memStats.PauseTotalNs))
}

// UpdateChannelMetrics updates metrics for a given channel
func UpdateChannelMetrics(chanLen, chanCap int, sizeGauge, capGauge prometheus.Gauge) {
	sizeGauge.Set(float64(chanLen))
	capGauge.Set(float64(chanCap))
}

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// MockTradesGeneratedTotal counts the total number of mock trades generated
	MockTradesGeneratedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "market_data_ingest_mock_trades_generated_total",
		Help: "The total number of mock trades generated",
	})

	// MockTradesDroppedTotal counts trades dropped due to backpressure
	MockTradesDroppedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "market_data_ingest_mock_trades_dropped_total",
		Help: "The total number of mock trades dropped due to backpressure",
	})

	// MockTradeRateGauge tracks the current rate of mock trades generation
	MockTradeRateGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "market_data_ingest_mock_trades_rate",
		Help: "The current rate of mock trades being generated per second",
	})

	// MockTrafficSpikeActive indicates if a traffic spike is currently active
	MockTrafficSpikeActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "market_data_ingest_mock_traffic_spike_active",
		Help: "Indicates whether a mock traffic spike is currently active (1) or not (0)",
	})
)

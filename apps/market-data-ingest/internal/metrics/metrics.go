package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// System metrics
	GoroutinesCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_goroutines_count",
		Help: "The current number of goroutines",
	})

	MemoryAllocBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_memory_alloc_bytes",
		Help: "Current memory allocation in bytes",
	})

	HeapAllocBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_heap_alloc_bytes",
		Help: "Current heap allocation in bytes",
	})

	HeapObjectsCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_heap_objects_count",
		Help: "Current number of allocated heap objects",
	})

	GCPauseNanosTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_gc_pause_nanos_total",
		Help: "Total time spent in GC pause in nanoseconds",
	})

	// Ingestor metrics
	TradesReceivedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "app_trades_received_total",
		Help: "Total number of trades received from the source",
	})

	// Mock ingestor metrics
	MockTradesReceivedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "app_mock_trades_received_total",
		Help: "Total number of mock trades received from the generator",
	})

	// Aggregator metrics
	TradesAggregatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "app_trades_aggregated_total",
		Help: "Total number of trades sent after aggregation",
	})

	AggregatorMapSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_aggregator_map_size",
		Help: "Current size of the aggregator map",
	})

	AggregationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "app_aggregation_duration_seconds",
		Help:    "Duration of trade aggregation operations",
		Buckets: prometheus.DefBuckets,
	})

	// Processor metrics
	TradesProcessedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "app_trades_processed_total",
		Help: "Total number of trades processed",
	})

	RedisSetTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "app_redis_set_total",
		Help: "Total number of Redis SET operations",
	})

	RedisSetErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "app_redis_set_errors_total",
		Help: "Total number of Redis SET errors",
	})

	RedisPublishTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "app_redis_publish_total",
		Help: "Total number of Redis PUBLISH operations",
	})

	RedisPublishErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "app_redis_publish_errors_total",
		Help: "Total number of Redis PUBLISH errors",
	})

	RedisOperationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "app_redis_operation_duration_seconds",
		Help:    "Duration of Redis operations",
		Buckets: prometheus.DefBuckets,
	})

	// Kafka metrics
	KafkaPublishTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "app_kafka_publish_total",
		Help: "Total number of Kafka publish operations",
	})

	KafkaPublishErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "app_kafka_publish_errors_total",
		Help: "Total number of Kafka publish errors",
	})

	KafkaRetriesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "app_kafka_retries_total",
		Help: "Total number of Kafka publish retries",
	})

	KafkaOperationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "app_kafka_operation_duration_seconds",
		Help:    "Duration of Kafka operations",
		Buckets: prometheus.DefBuckets,
	})

	// Channel metrics
	RawTradesChanSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_raw_trades_channel_size",
		Help: "Current size of the raw trades channel",
	})

	RawTradesChanCapacity = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_raw_trades_channel_capacity",
		Help: "Capacity of the raw trades channel",
	})

	ProcessedTradesChanSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_processed_trades_channel_size",
		Help: "Current size of the processed trades channel",
	})

	ProcessedTradesChanCapacity = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_processed_trades_channel_capacity",
		Help: "Capacity of the processed trades channel",
	})

	KafkaChanSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_kafka_channel_size",
		Help: "Current size of the Kafka channel",
	})

	KafkaChanCapacity = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_kafka_channel_capacity",
		Help: "Capacity of the Kafka channel",
	})
)

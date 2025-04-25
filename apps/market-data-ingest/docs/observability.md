# Observability in Market Data Ingest Service

This document outlines the observability setup for the Market Data Ingest service, covering metrics collection with Prometheus and log aggregation with Loki.

## Overview

The observability stack consists of:

- **Prometheus**: For metrics collection and storage
- **Loki**: For log aggregation and querying
- **Promtail**: For log collection and shipping to Loki
- **Grafana**: For visualization of both metrics and logs

## Metrics

### Available Metrics

The service exposes the following metrics categories:

1. **System Metrics**
   - `app_goroutines_count`: Number of active goroutines
   - `app_memory_alloc_bytes`: Current memory allocation
   - `app_heap_alloc_bytes`: Current heap allocation
   - `app_heap_objects_count`: Number of allocated heap objects
   - `app_gc_pause_nanos_total`: Total time spent in GC pause

2. **Business Metrics**
   - `app_trades_received_total`: Total number of trades received
   - `app_trades_aggregated_total`: Total number of trades after aggregation
   - `app_trades_processed_total`: Total number of trades processed
   - `app_aggregator_map_size`: Current size of the aggregator map

3. **Redis Metrics**
   - `app_redis_set_total`: Total Redis SET operations
   - `app_redis_set_errors_total`: Total Redis SET errors
   - `app_redis_publish_total`: Total Redis PUBLISH operations
   - `app_redis_publish_errors_total`: Total Redis PUBLISH errors
   - `app_redis_operation_duration_seconds`: Histogram of Redis operation durations

4. **Kafka Metrics**
   - `app_kafka_publish_total`: Total Kafka publish operations
   - `app_kafka_publish_errors_total`: Total Kafka publish errors
   - `app_kafka_retries_total`: Total Kafka publish retries
   - `app_kafka_operation_duration_seconds`: Histogram of Kafka operation durations

5. **Channel Metrics**
   - `app_raw_trades_channel_size`: Current size of the raw trades channel
   - `app_raw_trades_channel_capacity`: Capacity of the raw trades channel
   - `app_processed_trades_channel_size`: Current size of the processed trades channel
   - `app_processed_trades_channel_capacity`: Capacity of the processed trades channel
   - `app_kafka_channel_size`: Current size of the Kafka channel
   - `app_kafka_channel_capacity`: Capacity of the Kafka channel

### Accessing Metrics

Metrics are exposed via the `/metrics` endpoint on port 6060. In the Docker environment, this is mapped to port 15060 on the host.

## Logging

The service uses structured logging with `slog` that outputs logs in either text or JSON format based on the `LOG_FORMAT` environment variable.

### Log Levels

- `error`: Critical errors that require immediate attention
- `warn`: Warning conditions that should be addressed
- `info`: Informational messages about normal operation
- `debug`: Detailed debugging information

### Log Collection

Logs are collected using Promtail and sent to Loki for storage and querying. The setup targets Docker logs from the market-data-ingest service.

### Log Querying

In Grafana, you can query logs using LogQL. Some useful queries:

- View all logs: `{container=~".*market-data-ingest.*"}`
- View error logs: `{container=~".*market-data-ingest.*"} |~ "level=error"`
- View component-specific logs: `{container=~".*market-data-ingest.*"} |~ "component=ingestor"`
- Count errors over time: `sum(count_over_time({container=~".*market-data-ingest.*"} |~ "level=error"[1m]))`

## Dashboards

A comprehensive Grafana dashboard has been created for monitoring the service, containing:

1. System metrics panels
2. Business metrics panels
3. Performance metrics panels
4. Component-specific log panels
5. Log frequency visualization

Access the dashboard at: http://localhost:18000 (user: admin, password: admin)

## Adding Metrics

To add new metrics:

1. Define metrics in `internal/metrics/metrics.go`
2. Update component code to collect and report metrics
3. Add the new metrics to the Grafana dashboard

## Troubleshooting

Common observability issues:

1. **Metrics not appearing in Prometheus**
   - Check if the service is exposing the `/metrics` endpoint
   - Verify Prometheus configuration in `prometheus.yml`
   - Check Prometheus targets in the Prometheus UI

2. **Logs not appearing in Loki**
   - Check Promtail configuration
   - Verify that Docker logs are being generated
   - Check Promtail and Loki logs for errors

3. **Dashboard visualizations not working**
   - Verify data source configurations in Grafana
   - Check query expressions for syntax errors
   - Ensure time ranges are appropriate

## Reference

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Loki Documentation](https://grafana.com/docs/loki/latest/)
- [Grafana Documentation](https://grafana.com/docs/grafana/latest/)
- [LogQL Query Language](https://grafana.com/docs/loki/latest/logql/) 
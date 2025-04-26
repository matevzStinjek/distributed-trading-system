# Testing Strategy for Market Data Ingest Service

This document outlines the testing approach for the Market Data Ingest service.

## Overview

The testing strategy focuses on:

1. Unit tests for individual components
2. Integration tests for verifying component interaction
3. Mock implementations of external dependencies

## Mock Implementations

The service interacts with several external systems:

- **Alpaca** (market data provider)
- **Redis** (cache and pub/sub)
- **Kafka** (message broker)

Mock implementations of these dependencies have been created in `internal/mocks/mocks.go` to facilitate testing without requiring actual infrastructure.

## Test Coverage

### Core Components

1. **Ingestor**
   - Tests in `internal/ingestor/ingestor_test.go`
   - Verifies subscription to market data symbols
   - Tests error handling during subscription
   - Validates graceful shutdown

2. **Processor**
   - Tests in `internal/processor/processor_test.go`
   - Validates caching of latest trades
   - Tests publication to Redis pub/sub
   - Verifies error handling for cache and pub/sub operations
   - Confirms proper forwarding of trades to subsequent stages

3. **Kafka Worker**
   - Tests in `internal/producer/kafka_worker_test.go`
   - Validates publishing trades to Kafka
   - Tests error handling during Kafka production
   - Verifies proper message content

4. **Integration**
   - Tests in `internal/integration_test.go`
   - Verifies end-to-end flow through all components
   - Ensures data makes it from market data source to Kafka
   - Validates intermediate state in cache and pub/sub

## Running Tests

To run all tests:

```bash
cd apps/market-data-ingest
go test ./...
```

To run tests with coverage report:

```bash
cd apps/market-data-ingest
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

To run a specific test:

```bash
cd apps/market-data-ingest
go test ./internal/ingestor -run TestTradeIngestor_Start
```

## Future Test Improvements

1. **Benchmarking**: Add benchmark tests for critical paths to monitor performance
2. **Chaos Testing**: Test resilience by simulating random failures in dependencies
3. **End-to-End Tests**: Add tests with real infrastructure in a test environment 
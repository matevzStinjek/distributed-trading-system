# Technical Architecture Document: Mock Stock Trading System

**Version:** 1.0
**Date:** 2025-04-01
**Related Requirements:** Requirements Document V1.0 (ID: `trading_reqs_doc_v1`)

**1. Introduction**

This document outlines the technical architecture for the Mock Stock Trading & Portfolio Tracking System. It builds upon the requirements document, specifying technology choices and design patterns to realize the system's functionality. The architecture emphasizes the use of distributed systems concepts and the chosen technology stack (Go, Elixir, Kafka, Redis, Flink, Elasticsearch, DynamoDB, Clickhouse, AWS API Gateway) to provide a valuable learning experience.

**2. System Architecture Overview**

The system employs a microservices architecture centered around an event streaming backbone (Kafka). User interactions flow through an API Gateway to backend services written primarily in Go. Real-time market data processing and broadcasting are handled by an Elixir service. Stateful stream processing for order execution and analytics aggregation is performed by Flink jobs. Data is persisted across specialized stores: DynamoDB for core portfolio state, Redis for caching and pub/sub, Elasticsearch for trade history search, and Clickhouse for analytics.

**(Conceptual Diagram Description)**

- **External Users/Feed:** Interact via API Gateway (requests) or WebSockets (prices). Market Feed pushes data into Kafka.
- **API Gateway (AWS API Gateway):** Routes external HTTP requests to specific backend services (Order, Portfolio, Query).
- **Backend Services (Go):**
  - `Order Service`: Handles order placement requests, validates (using Portfolio data), publishes valid orders to Kafka (`trade_orders`).
  - `Portfolio Service`: Handles portfolio read requests, interacts with DynamoDB/Redis.
  - `Query Service`: Handles history/analytics read requests, interacts with Elasticsearch/Clickhouse.
- **Real-time Service (Elixir):**
  - `Market Data Service`: Consumes `market_data` from Kafka, updates Redis Cache, publishes to Redis Pub/Sub, manages WebSockets for price broadcasts.
- **Event Bus (Kafka):** Topics: `market_data`, `trade_orders`, `trade_executed`. Acts as the central nervous system.
- **Stream Processing (Flink):**
  - `Trade Execution Job`: Consumes `trade_orders`, looks up prices (Redis), executes trades, updates DynamoDB portfolios atomically, publishes `trade_executed`.
  - `Persistence Jobs` (Can be combined or separate Flink jobs/consumers): Consume `trade_executed`, sink data to Elasticsearch (history) and Clickhouse (analytics aggregation).
- **Data Stores:**
  - `DynamoDB`: Stores user portfolios (cash, stock holdings). Primary source of truth for user state.
  - `Redis`: Caches latest market prices; used for Pub/Sub for price broadcasts.
  - `Elasticsearch`: Stores executed trade history for searching.
  - `Clickhouse`: Stores aggregated trade analytics.

**3. Core Architectural Decisions**

**3.1. Data Consistency and Storage**

- **Portfolios (User Cash & Holdings):**

  - _Storage:_ **DynamoDB**. Key: `userId`. Attributes: `cash_balance`, `holdings` (Map: `symbol` -> `quantity`).
  - _Rationale:_ Highly scalable, managed NoSQL store. Supports conditional writes essential for atomic updates (check balance/holdings before debit/credit). Offers tunable consistency (Strongly Consistent Reads available if needed, Eventually Consistent Reads for performance). Fits user-centric data model well.
  - _Consistency Model:_ **Strong Consistency for Writes** enforced via DynamoDB's conditional update expressions (e.g., `UPDATE ... SET cash = :new WHERE cash = :expected AND holdings.:symbol >= :sell_qty`). **Eventually Consistent Reads** preferred for display (Portfolio Service) for better performance/cost, unless strong consistency is explicitly required for a critical read-before-write scenario not covered by conditional writes.
  - _State Management:_ Portfolio state is directly managed within DynamoDB.

- **Trade Orders (In-flight):**

  - _Storage:_ **Kafka topic (`trade_orders`)**.
  - _Rationale:_ Provides durability and acts as a buffer. Decouples order placement from execution. Allows reliable processing by Flink.
  - _Consistency Model:_ Eventual processing. Kafka provides durability through replication and ordering within a partition.
  - _Partitioning:_ Partition by `userId` to enable sequential processing per user in Flink.

- **Executed Trades (History):**

  - _Storage:_ **Elasticsearch**. Index per time period (e.g., monthly) potentially.
  - _Rationale:_ Optimized for full-text search and filtering required by FR5.2. Scales horizontally for writes and reads.
  - _Consistency Model:_ **Eventual Consistency**. Data is available for search shortly after the `trade_executed` event is processed.
  - _Event Sourcing:_ Populated by consuming `trade_executed` events from Kafka (likely via a Flink sink).

- **Aggregated Analytics (Volume, etc.):**

  - _Storage:_ **Clickhouse**. Table designed for trade data aggregation.
  - _Rationale:_ High-performance columnar database optimized for OLAP queries (FR5.3). Efficiently handles aggregations over large datasets.
  - _Consistency Model:_ **Eventual Consistency**. Aggregates updated as `trade_executed` events are processed by Flink.

- **Market Prices (Latest):**
  - _Storage:_ **Redis Cache**. Key: `price:{symbol}`. Value: latest price.
  - _Rationale:_ In-memory store provides extremely low latency for price lookups needed during validation, execution (FR1.3), and broadcasting.
  - _Consistency Model:_ **Eventual Consistency**. Price in cache reflects the latest processed `market_data` event, which might have a minor lag from the absolute latest event. TTL can manage stale data if necessary.

**3.2. Message Flow Architecture**

- **Event Streaming:**

  - _Platform:_ **Kafka**.
  - _Rationale:_ Scalable, durable, fault-tolerant message bus standard for event-driven architectures. Decouples producers and consumers effectively.
  - _Topics:_
    - `market_data`: Raw price updates from the simulated feed. Partition by `symbol`.
    - `trade_orders`: Validated orders ready for execution. Partition by `userId`.
    - `trade_executed`: Events detailing completed trades. Partition by `userId`.
  - _Consumer Groups:_ Each distinct consumer application (Elixir Service, Flink Execution Job, Flink Persistence Jobs) uses its own `group.id` to manage offsets independently.

- **Real-time Updates (Price Broadcast):**

  - _Pattern:_ **Kafka -> Elixir Service -> Redis Pub/Sub -> Elixir Service (WebSockets) -> Clients**.
  - _Rationale:_ Leverages Kafka for durable ingestion, Elixir for efficient WebSocket management, and Redis Pub/Sub for fast, low-latency fan-out of price updates to potentially many Elixir instances handling client connections. This avoids overwhelming Kafka brokers with potentially thousands of direct WebSocket consumers.
  - _Redis Channels:_ e.g., `price_updates:{symbol}` or a general `price_updates` channel carrying `{symbol, price}` payloads.

- **Handling Concurrent Operations:**

  - _Portfolio Updates:_ **Optimistic Concurrency Control** using DynamoDB conditional writes. Flink job must handle potential `ConditionalCheckFailedException` (e.g., by retrying the read-process-write cycle or failing the order).
  - _Order Processing:_ Kafka partitioning by `userId` ensures orders from the _same user_ are processed sequentially by a single Flink task instance, preventing race conditions within the execution logic for that user's state. Concurrency happens _across_ users.

- **Message Ordering:**
  - Guaranteed within a Kafka partition. Critical for `trade_orders` (sequential processing per user). Less critical but present for `market_data` (per symbol) and `trade_executed` (per user).

**3.3. Service Architecture**

- **Service Boundaries & Responsibilities:** (As described in System Architecture Overview section 2)

  - Clear separation based on domain logic (Order, Portfolio, Market Data, Query) and technical capability (Stream Processing, API Gateway).

- **Language Choices & Rationale:**

  - **Go:** For stateless HTTP API services (Order, Portfolio, Query). Chosen for performance, strong concurrency support, simple deployment, and suitability for typical web backend tasks.
  - **Elixir:** For the stateful `Market Data Service`. Chosen for its exceptional ability to handle massive concurrency (WebSocket connections) and fault tolerance (BEAM VM), making it ideal for real-time components.
  - **Flink (Java/Scala):** Framework choice dictates JVM language. Chosen for its robust stateful stream processing capabilities, connectors, and exactly-once processing potential.

- **API Patterns:**

  - **External:** RESTful HTTP/JSON via AWS API Gateway. WebSockets for real-time price updates managed by the Elixir service.
  - **Internal (Synchronous):** REST/HTTP or gRPC for direct requests where needed (e.g., Order Service potentially calling Portfolio Service for validation, though direct Redis/DynamoDB access might be preferred).
  - **Internal (Asynchronous):** Kafka is the primary mechanism for communication between stages (Order Placement -> Execution -> Persistence).

- **Stateful vs Stateless Decisions:**
  - **Stateless:** Go services are designed stateless; state lives in external stores (DynamoDB, Redis) or Kafka. Enables easier scaling and resilience.
  - **Stateful:** Elixir service (manages WebSocket connections). Flink jobs (manage processing state, aggregations via Flink's state backends, potentially RocksDB on local disk checkpointed to S3). Redis, DynamoDB, Kafka, Elasticsearch, Clickhouse are inherently stateful components.

**3.4. Critical Distributed Systems Patterns**

- **Handling Partial Failures:**

  - _Idempotency:_ Order placement API should be idempotent. Use a client-generated idempotency key or check for duplicate `orderId` before processing/publishing to Kafka. Flink sinks should be configured for idempotency (using transactional or idempotent producers/connectors where available).
  - _Retries:_ Services making synchronous calls (e.g., to DynamoDB, Redis) must implement retry logic with exponential backoff and jitter.
  - _Circuit Breakers:_ Consider libraries (e.g., go-circuitbreaker) in Go services to prevent hammering a failing downstream dependency.
  - _Timeouts:_ Configure sensible timeouts for all network calls.
  - _Managed Services:_ Leverage high availability features of AWS services (API Gateway, DynamoDB, ElastiCache for Redis, MSK for Kafka, Managed Flink/EMR, OpenSearch/Managed Clickhouse).
  - _Flink Fault Tolerance:_ Rely on Flink's checkpointing mechanism for state recovery and Kafka consumer offset management for resuming processing after failure. Ensure checkpoints are stored reliably (e.g., S3).
  - _Potential Failure Mode 1:_ `Order Service` publishes to Kafka, then crashes before sending HTTP 200 OK. Client retries. _Mitigation:_ Idempotent order submission check.
  - _Potential Failure Mode 2:_ `Trade Execution (Flink)` reads order, updates DynamoDB, crashes before committing Kafka offset/publishing `trade_executed`. _Mitigation:_ Flink's exactly-once semantics (EOS) using checkpointing and transactional/idempotent sinks ensure atomicity across Kafka commit, state update, and output. DynamoDB update itself uses conditional writes for its own atomicity.

- **Concurrent Operation Management:**

  - Primary mechanisms: DynamoDB conditional writes (OCC) and Kafka partitioning by `userId`.

- **Data Partition Strategy:**

  - _DynamoDB:_ Key = `userId`.
  - _Kafka:_ Key = `userId` for `trade_orders`, `trade_executed`. Key = `symbol` for `market_data`.
  - _Elasticsearch/Clickhouse:_ Leverage built-in sharding. Consider time-based indices for Elasticsearch if data volume grows large.

- **Minimum Viable Observability (for Learning/Debugging):**
  - **Logging:** Structured JSON logs from all services/jobs. Include `traceId`, `userId`, `orderId`, `symbol` where applicable. Centralize logs (e.g., CloudWatch Logs). Log key events (API entry/exit, message publish/consume, DB interactions, state changes, errors).
  - **Tracing:** Implement distributed tracing using OpenTelemetry SDKs. Propagate trace context across service calls (HTTP headers) and Kafka messages (headers). Visualize traces (e.g., AWS X-Ray, Jaeger). Essential for understanding request flow and pinpointing latency/errors.
  - **Metrics:** Basic RED metrics (Rate, Errors, Duration) for API endpoints (API Gateway/Go/Elixir services). Kafka consumer lag per partition/group. Flink job health metrics (checkpoint size/duration, uptime, records processed). Database/Cache metrics (latency, errors/throttling, connections). Expose via Prometheus endpoint or CloudWatch Metrics.

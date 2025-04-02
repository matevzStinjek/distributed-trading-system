## Technical Specification: Mock Stock Trading & Portfolio Tracking System

**Version:** 1.0
**Date:** 2025-04-02

**Document Purpose:** This document integrates the archived requirements, architectural decisions, and system design details for the Mock Stock Trading & Portfolio Tracking System. It serves as the single source of truth for technical specifications, ensuring clear traceability from requirements to implementation guidance.

---

**Table of Contents:**

1.  System Overview and Scope
2.  Core Requirements
    - 2.1 Functional Requirements
    - 2.2 Non-Functional Requirements
    - 2.3 System Invariants and Consistency Requirements
3.  Technical Architecture
    - 3.1 Architecture Overview
    - 3.2 Core Architectural Decisions
    - 3.3 Technology Stack Rationale
    - 3.4 Service Architecture
4.  Distributed Systems Patterns
    - 4.1 Event Streaming Backbone (Kafka)
    - 4.2 Real-time Updates (Elixir & Redis Pub/Sub)
    - 4.3 Data Consistency Patterns (Conditional Writes, Event Sourcing)
    - 4.4 Handling Partial Failures (Idempotency, Retries, Flink EOS)
    - 4.5 Concurrent Operation Management
    - 4.6 Data Partitioning Strategy
    - 4.7 System Boundaries and Failure Domains
5.  Implementation Guidance
    - 5.1 Service Specifications (Go, Elixir, Flink)
    - 5.2 Data Flow Specifications
    - 5.3 Event Schemas (Kafka Topics)
    - 5.4 Storage Schemas (DynamoDB, Redis, Elasticsearch, Clickhouse)
    - 5.5 API Specifications (REST, WebSocket)
    - 5.6 Schema Evolution and Versioning
6.  Testing and Verification Approach
    - 6.1 Distributed Systems Testing Requirements
    - 6.2 Observability Requirements (Logging, Tracing, Metrics)
    - 6.3 Recovery Procedures
7.  Requirements Traceability Matrix

---

### 1. System Overview and Scope

- **Purpose:** To serve as a practical learning platform for distributed, horizontally scaled systems by simulating core aspects of a stock trading system using simplified logic.
- **System Overview:** Allows registered users to simulate trading mock stocks using mock currency. Users view price streams, manage portfolios, place orders, and review trade history. The system processes orders based on simulated market prices and available funds/shares, updating portfolios atomically. Basic analytics on market activity are provided. Focus is on demonstrating distributed patterns over financial accuracy.
- **Scope:**
  - **In Scope:** Simulated real-time market data processing, near real-time price display, user portfolio management (mock cash/stock), mock market order placement (buy/sell), order validation, simulated order execution, trade history recording/viewing, basic aggregated analytics.
  - **Out of Scope:** Real money/markets, user registration/authentication, complex order types, market making, dividends/splits, regulatory compliance, sophisticated analytics/charting.
- **Target Audience:** System architects and software developers.

### 2. Core Requirements

#### 2.1 Functional Requirements

_(Referenced from Requirements Document)_

- **FR1: Market Data Simulation & Display**
  - FR1.1: Ingest and process continuous simulated stock price updates (symbol, price).
  - FR1.2: Provide near real-time price observation for users.
  - FR1.3: Maintain and expose the latest known price for each symbol.
- **FR2: User Portfolio Management**
  - FR2.1: Securely store and manage user portfolios (cash balance, stock quantities).
  - FR2.2: Allow users to query their current portfolio status.
  - FR2.3: Update portfolios accurately and reliably upon trade execution.
- **FR3: Trade Order Placement**
  - FR3.1: Allow authenticated users to submit buy orders (symbol, quantity).
  - FR3.2: Allow authenticated users to submit sell orders (symbol, quantity).
  - FR3.3: **Buy Order Validation:** Verify sufficient cash balance based on latest known price before acceptance.
  - FR3.4: **Sell Order Validation:** Verify sufficient stock holdings before acceptance.
  - FR3.5: Reject invalid orders immediately with reason.
  - FR3.6: Accept valid orders, queue for execution, confirm acceptance to user.
- **FR4: Trade Order Execution Simulation**
  - FR4.1: Process accepted orders in a timely manner (strict FIFO desirable but not mandatory).
  - FR4.2: Use reasonably current market price for execution; slippage simulation not required.
  - FR4.3: **Atomic Portfolio Update:** Atomically update cash and stock quantity upon successful execution.
  - FR4.4: Create a persistent trade record for each executed trade (user, symbol, quantity, side, price, timestamp).
- **FR5: Trade History & Analytics**
  - FR5.1: Allow users to retrieve their chronological trade history.
  - FR5.2: Allow basic filtering of trade history (e.g., by symbol).
  - FR5.3: Calculate and expose basic aggregated analytics (e.g., total volume per symbol per period).

#### 2.2 Non-Functional Requirements

_(Referenced from Requirements Document)_

- **NFR1: Data Consistency**
  - NFR1.1: **Strong Consistency Required** for order validation/acceptance and atomic portfolio updates. Partial updates are unacceptable.
  - NFR1.2: **Eventual Consistency Acceptable** for real-time price display (sub-second delay ok), trade history view (few seconds delay ok), aggregated analytics, and portfolio _view_ reads (low latency desirable).
- **NFR2: Unacceptable State Violations**
  - Prevent negative cash balance.
  - Prevent negative stock holdings.
  - Prevent selling more shares than owned _at execution time_.
  - Prevent buy execution causing negative cash based on _actual execution price_.
  - Prevent permanent loss of accepted orders.
  - Prevent inconsistent portfolio state (e.g., cash deducted, shares not credited).
- **NFR3: Availability**
  - High availability for order placement (FR3) and portfolio viewing (FR2.2).
  - Moderate availability for price display (FR1.2) and history/analytics (FR5).
- **NFR4: Performance (Target Guidelines)**
  - Price broadcast latency: Median < 500ms.
  - Order validation latency: P99 < 500ms.
  - Portfolio view latency: P99 < 500ms.
  - Trade execution latency: P99 < 5 seconds (acceptance to update completion).
  - Trade history query latency: P99 < 1 second.
  - Analytics query latency: P99 < 2 seconds.
- **NFR5: Scalability**
  - Design for horizontal scalability: tens of thousands of users, thousands of symbols, high volume of price updates, orders, and trades.

#### 2.3 System Invariants and Consistency Requirements

_(Derived from Requirements and System Design)_

- **Portfolio Invariants:**
  - Non-negative cash balance.
  - Non-negative holdings for any stock.
  - Sum of executed trades must align with portfolio balance changes.
  - Holdings must be consistent with trade history.
- **Execution Invariants:**
  - No duplicate order executions.
  - No lost accepted orders.
  - No phantom trades (trades without corresponding order/execution).
  - Complete audit trail of all portfolio changes.
- **Event Stream Invariants:**
  - No gaps in critical event sequences (e.g., within a user's order stream).
  - Causal ordering maintained where necessary (e.g., order accepted before executed).
  - Complete execution chains (Order -> Validation -> Execution -> Persistence).

### 3. Technical Architecture

_(Referenced from Architecture Document)_

#### 3.1 Architecture Overview

- Microservices architecture centered around Kafka event streaming.
- API Gateway (AWS API Gateway) handles user interactions.
- Backend services (Go) for core logic (Order, Portfolio, Query).
- Real-time service (Elixir) for market data processing and WebSocket broadcasting.
- Stateful stream processing (Flink) for order execution and analytics.
- Specialized data stores: DynamoDB (portfolios), Redis (cache/pub-sub), Elasticsearch (history), Clickhouse (analytics).

#### 3.2 Core Architectural Decisions

- **Data Consistency & Storage:**
  - Portfolios: DynamoDB (Strongly Consistent Writes via Conditional Updates, Eventually Consistent Reads preferred for display). Rationale: Scalability, managed NoSQL, conditional writes for atomicity (NFR1.1, NFR2, FR4.3).
  - In-flight Orders: Kafka `trade_orders` topic (partitioned by `userId`). Rationale: Durability, buffering, decoupling, reliable sequential processing per user.
  - Trade History: Elasticsearch. Rationale: Optimized for search/filtering (FR5.1, FR5.2), scalable. Populated via `trade_executed` events (Eventual Consistency - NFR1.2).
  - Aggregated Analytics: Clickhouse. Rationale: Columnar DB optimized for OLAP (FR5.3). Populated via `trade_executed` events (Eventual Consistency - NFR1.2).
  - Market Prices (Cache): Redis. Rationale: Low-latency lookups (FR1.3, FR3.3, FR4.2). (Eventual Consistency - NFR1.2).
- **Message Flow:**
  - Primary communication: Kafka event streaming (`market_data`, `trade_orders`, `trade_executed`). Rationale: Decoupling, scalability, fault tolerance.
  - Real-time Price Broadcast: Kafka -> Elixir -> Redis Pub/Sub -> Elixir (WebSockets) -> Clients. Rationale: Durable ingestion, efficient connection handling, low-latency fan-out (FR1.2, NFR4).

#### 3.3 Technology Stack Rationale

_(Referenced from Architecture Document)_

- **AWS API Gateway:** Managed routing, security, rate limiting for external APIs.
- **Go:** Stateless HTTP services (Order, Portfolio, Query) - performance, concurrency, simple deployment.
- **Elixir:** Stateful Market Data Service - massive concurrency (WebSockets), fault tolerance (BEAM).
- **Kafka:** Event bus - scalable, durable, standard for event-driven systems.
- **Flink:** Stateful stream processing (Trade Execution, Persistence) - robust, exactly-once potential, connectors.
- **DynamoDB:** Portfolio store - scalable NoSQL, conditional writes for consistency.
- **Redis:** Caching & Pub/Sub - low latency in-memory operations.
- **Elasticsearch:** Trade history store - optimized for search.
- **Clickhouse:** Analytics store - optimized for OLAP queries.

#### 3.4 Service Architecture

- **Boundaries:** Clear separation based on domain logic and technical capability.
- **API Patterns:** External (REST/WebSocket), Internal Async (Kafka), Internal Sync (REST/gRPC if needed, but Kafka preferred).
- **State Management:** Go services stateless; Elixir manages WebSocket state; Flink manages processing state; Data stores hold persistent state.

### 4. Distributed Systems Patterns

_(Referenced from Architecture and System Design)_

#### 4.1 Event Streaming Backbone (Kafka)

- Acts as the central nervous system, decoupling producers and consumers.
- Topics: `market_data` (key: symbol), `trade_orders` (key: userId), `trade_executed` (key: userId).
- Uses consumer groups for independent processing.

#### 4.2 Real-time Updates (Elixir & Redis Pub/Sub)

- Pattern: Kafka -> Elixir Service -> Redis Pub/Sub -> Elixir Service (WebSockets) -> Clients.
- Leverages Elixir's concurrency for WebSockets and Redis Pub/Sub for efficient fan-out.

#### 4.3 Data Consistency Patterns (Conditional Writes, Event Sourcing)

- **Portfolio Updates:** DynamoDB conditional writes used by Flink Execution Job to ensure atomicity and check balances/holdings concurrent-safely (addresses NFR1.1, NFR2, FR4.3).
- **Event Sourcing (Implicit):** System state changes (portfolio, history, analytics) are driven by consuming events (`trade_executed`) from Kafka, ensuring eventual consistency for read models (NFR1.2).

#### 4.4 Handling Partial Failures

- **Idempotency:** Order placement API (requires client key or check), Flink sinks (via EOS configuration).
- **Retries:** Implemented in services calling dependencies (e.g., Go services calling DynamoDB/Redis) with backoff/jitter.
- **Flink Fault Tolerance:** Checkpointing (to S3) and Kafka offset management for state recovery and exactly-once semantics (EOS) where configured. Addresses potential failures during execution (NFR2).
- **Circuit Breakers:** Recommended for Go services calling downstream dependencies.
- **Timeouts:** Configured for all network calls.

#### 4.5 Concurrent Operation Management

- **Primary Mechanisms:** DynamoDB conditional writes (Optimistic Concurrency Control - OCC) for portfolio updates; Kafka partitioning by `userId` for sequential order processing per user in Flink. Addresses NFR1.1 and prevents races within a user's operations.

#### 4.6 Data Partitioning Strategy

- **DynamoDB:** Partition Key `userId`.
- **Kafka:** Partition Key `symbol` for `market_data`; `userId` for `trade_orders` and `trade_executed`.
- **Elasticsearch/Clickhouse:** Utilize built-in sharding; consider time-based indices for Elasticsearch.

#### 4.7 System Boundaries and Failure Domains

- Services designed for independent failure modes and scaling.
- Bulkheads isolate Market Data, Order Processing, Analytics, and Client Connections.
- Clear cross-boundary communication patterns (retries, timeouts, circuit breaking) defined.

### 5. Implementation Guidance

_(Referenced primarily from System Design Document)_

#### 5.1 Service Specifications

- **API Gateway (AWS API Gateway):** Configuration defines routes, validation, auth, rate limits, integrations (Lambda/HTTP Proxy).
- **Order Service (Go):** HTTP server, validation logic, Kafka producer, Redis/DynamoDB clients. Handles `POST /orders`, validates against portfolio/price, publishes to `trade_orders`. Stateless.
- **Portfolio Service (Go):** HTTP server, Redis/DynamoDB clients. Handles `GET /portfolio/{userId}`. Stateless.
- **Query Service (Go):** HTTP server, Elasticsearch/Clickhouse clients. Handles `GET /trades/search`, `GET /analytics/volume`. Stateless.
- **Market Data Service (Elixir):** OTP App, Kafka consumer (`market_data`), Redis client (cache SET, Pub/Sub PUBLISH), WebSocket handler (Phoenix Channels). Consumes market data, updates cache, broadcasts via WebSockets. Stateful (WebSocket connections).
- **Trade Execution Service (Flink Job - Java/Scala):** DataStream API, Kafka source (`trade_orders`), Redis lookup (price), DynamoDB sink (conditional update), Kafka sink (`trade_executed`). Implements core execution logic with EOS. Stateful (Flink state).
- **Persistence Services (Flink Jobs/Consumers):** Kafka source (`trade_executed`), Elasticsearch/Clickhouse sinks. Potentially stateful if aggregating within Flink.

#### 5.2 Data Flow Specifications

- **Market Data Flow:** External Feed -> Kafka (`market_data`) -> Elixir Service -> Redis Cache (SET) & Redis Pub/Sub -> Elixir Service (WebSocket Push) -> Client.
- **Order Placement Flow:** Client -> API Gateway -> Order Service (Validate: Redis Price, DynamoDB Portfolio) -> Kafka (`trade_orders`) -> Client (Accepted/Rejected).
- **Order Execution Flow:** Kafka (`trade_orders`) -> Flink Execution Job (Lookup: Redis Price; Update: DynamoDB Portfolio - Conditional Write) -> Kafka (`trade_executed`).
- **Persistence Flow:** Kafka (`trade_executed`) -> Flink Persistence Job(s) -> Elasticsearch & Clickhouse.

#### 5.3 Event Schemas (Kafka Topics)

- **Common Fields:** `eventId`, `eventTimestamp`, `sourceService`, `schemaVersion`.
- **`market_data`:** `payload: { symbol, price, feedTimestamp }`.
- **`trade_orders`:** `payload: { orderId, userId, symbol, quantity, side, orderTimestamp }`, optional `traceContext`.
- **`trade_executed`:** `payload: { orderId, userId, symbol, quantity, side, status ("EXECUTED", "FAILED_..."), executionPrice?, executionTimestamp, failureReason? }`, optional `traceContext`.

#### 5.4 Storage Schemas

- **DynamoDB (`Portfolios`):** PK: `userId` (String). Attributes: `cashBalance` (Number/Decimal), `holdings` (Map<String, Number/Decimal>), `lastUpdatedTimestamp`, `version` (Optional).
- **Redis Cache (Prices):** Key: `price:{symbol}` (String). Value: Price (String).
- **Elasticsearch (`Trades`):** Time-based indices (e.g., `trading_trades-{yyyy-MM}`). Fields: `eventId`, `orderId`, `userId`, `symbol`, `quantity`, `side`, `status`, `executionPrice`, `executionTimestamp`, `failureReason` (Typed mapping defined).
- **Clickhouse (`Trades`):** Table: `trading_trades`. Engine: `MergeTree`. Schema includes typed columns matching `trade_executed` payload, optimized for aggregation.

#### 5.5 API Specifications

- **Base URL:** `https://api.mocktrading.example.com/v1`.
- **Auth:** Handled by Gateway (e.g., JWT).
- **Rate Limiting:** Handled by Gateway.
- **Order API (`POST /orders`):** Accepts order details, returns `202 Accepted` with `orderId` or error (400, 402, 409, 422, 500, 503). Idempotency key recommended.
- **Portfolio API (`GET /portfolio/{userId}`):** Returns portfolio details or error (401, 403, 404, 503).
- **Trade History API (`GET /trades/search`):** Accepts filters (`userId`, `symbol`, date range, pagination), returns trade list or error (400, 401, 403, 500).
- **Analytics API (`GET /analytics/volume`):** Accepts filters (`symbol`, `timeWindow`), returns aggregated volume data or error (400, 500).
- **Market Data API (`wss://.../marketdata`):** WebSocket for subscribing to `prices:{symbol}` topics and receiving `price_update` events.

#### 5.6 Schema Evolution and Versioning

- Include `schemaVersion` in Kafka events.
- Prioritize backward compatibility for event schema changes (e.g., adding optional fields).
- Plan migration strategies for state schemas (DynamoDB, ES, Clickhouse) ensuring consistency.

### 6. Testing and Verification Approach

_(Referenced from System Design Document)_

#### 6.1 Distributed Systems Testing Requirements

- **Consistency Testing:** Verify portfolio state under concurrency, event ordering, state after failures.
- **Performance Testing:** Measure latency, throughput, behavior under load, recovery time.
- **Failure Testing (Chaos Engineering):** Simulate component failures, network partitions; verify recovery and data consistency.

#### 6.2 Observability Requirements

- **Logging:** Structured JSON logs, centralized, with correlation IDs (`traceId`, `userId`, `orderId`). Log key events, state changes, errors.
- **Tracing:** Distributed tracing (OpenTelemetry), context propagation (HTTP/Kafka headers), visualization (X-Ray/Jaeger). Track end-to-end flow, latency breakdowns.
- **Metrics:** RED metrics for APIs, Kafka consumer lag, Flink job health, DB/Cache performance (latency, errors, connections). Expose via Prometheus/CloudWatch.

#### 6.3 Recovery Procedures

- Define service recovery order, state reconstruction methods, data reconciliation processes.
- Require point-in-time recovery for state stores where feasible, event replay capabilities (Kafka), clear consistency verification post-recovery.

### 7. Requirements Traceability Matrix

| Requirement ID | Brief Description                            | Architectural Decision / Pattern                             | Implementation Component(s)                               | Verification Method                      |
| :------------- | :------------------------------------------- | :----------------------------------------------------------- | :-------------------------------------------------------- | :--------------------------------------- |
| FR1.1          | Ingest Market Data                           | Kafka `market_data` topic                                    | Market Data Service (Elixir) - Kafka Consumer             | Integration Test, Metrics (Lag)          |
| FR1.2, NFR4    | Near Real-time Price Display (<500ms)        | Kafka->Elixir->Redis PubSub->WebSocket                       | Market Data Service (Elixir), Redis, Client UI            | Performance Test, Tracing                |
| FR1.3, NFR1.2  | Access Latest Price (Eventually Const.)      | Redis Cache `price:{symbol}`                                 | Market Data Service (Set), Order Svc (Get), Flink (Get)   | Integration Test, Unit Test              |
| FR2.1, FR2.3   | Store/Manage Portfolio                       | DynamoDB `Portfolios` Table                                  | Portfolio Service (Read), Flink Exec Job (Write)          | Integration Test                         |
| FR2.2, NFR4    | View Portfolio (<500ms, Eventually Const.)   | DynamoDB Read (Eventually Consistent) via Portfolio Service  | Portfolio Service (Go), DynamoDB                          | Performance Test, Unit Test              |
| FR3.1-FR3.6    | Place/Validate Order (<500ms validation)     | API Gateway -> Order Service -> Kafka `trade_orders`         | API Gateway, Order Service (Go), Redis, DynamoDB, Kafka   | Integration Test, Performance Test       |
| FR3.3, FR3.4   | Order Validation (Funds/Shares)              | Order Service reads Redis (Price), DynamoDB (Portfolio)      | Order Service (Go)                                        | Unit Test, Integration Test              |
| FR4.1, FR4.2   | Execute Order (Timely, Current Price)        | Flink Job consumes `trade_orders`, reads Redis Price         | Trade Execution Job (Flink), Redis                        | Integration Test                         |
| FR4.3, NFR1.1  | Atomic Portfolio Update (Strong Const.)      | Flink Job uses DynamoDB Conditional Writes                   | Trade Execution Job (Flink), DynamoDB                     | Integration Test (Concurrency), Logs     |
| NFR2           | Prevent Invalid States                       | DynamoDB Conditional Writes, Validation Logic                | Order Service (Go), Trade Execution Job (Flink), DynamoDB | Consistency Test, Unit Test              |
| FR4.4          | Create Trade Record                          | Flink Job publishes `trade_executed` event                   | Trade Execution Job (Flink), Kafka                        | Integration Test                         |
| FR5.1, NFR1.2  | View Trade History (Eventually Const.)       | Persist `trade_executed` to Elasticsearch via Flink/Consumer | Persistence Job (Flink/Consumer), Query Service (Go), ES  | Integration Test, Performance Test       |
| FR5.3, NFR1.2  | Calculate Agg. Analytics (Eventually Const.) | Persist/Aggregate `trade_executed` to Clickhouse             | Persistence Job (Flink/Consumer), Query Service (Go), CH  | Integration Test, Performance Test       |
| NFR3           | Availability                                 | Managed Services (AWS), Flink HA, Service Replication        | All Components, AWS Infrastructure                        | Failure Testing, Monitoring              |
| NFR5           | Scalability                                  | Microservices, Kafka, Flink, NoSQL, Horizontal Scaling       | All Components                                            | Load Testing, Architecture Review        |
|                | Idempotency                                  | Client Key / Flink EOS Sinks                                 | Order Service (Go), Flink Jobs, Client                    | Integration Test, Failure Testing        |
|                | Observability                                | Logging, Tracing (OpenTelemetry), Metrics                    | All Components                                            | Manual Inspection, Monitoring Dashboards |

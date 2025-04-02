# System Design Document: Mock Stock Trading System

**Version:** 1.0
**Date:** 2025-04-02
**Related Architecture:** Technical Architecture Document V1.0 (ID: `trading_arch_doc_v1`)

**1. Introduction**

This document provides detailed specifications for the implementation of the Mock Stock Trading System. It elaborates on the architectural decisions outlined in the Technical Architecture Document, defining concrete service structures, data flows, schemas, APIs, and error handling procedures necessary for development.

**2. Service Specifications**

**2.1. API Gateway Service (AWS API Gateway)**

- **Internal Structure:** Configuration within AWS API Gateway. Defines routes, request validation (basic schema), authentication method (e.g., JWT authorizer - implementation details out of scope per requirements), rate limiting, and integration points (Lambda proxy, HTTP proxy) with backend services.
- **Key Data Structures:** N/A (Configuration based).
- **Interface Definitions:** Exposes the public REST API endpoints defined in Section 6.
- **Error Handling:** Maps backend errors to appropriate HTTP status codes (e.g., 4xx for client errors, 5xx for server errors). Handles authentication/authorization errors (401/403). Enforces rate limits (429).
- **Local State:** None.
- **External Dependencies:** Order Service, Portfolio Service, Query Service, Authentication Service (logical).

**2.2. Order Service (Go)**

- **Internal Structure:**
  - HTTP server (e.g., `net/http` or framework like Gin/Echo).
  - Handler for `POST /orders`.
  - Validation logic module.
  - Kafka producer client (`confluent-kafka-go` or similar).
  - Redis client (`go-redis`).
  - DynamoDB client (`aws-sdk-go-v2`).
  - Structured logging setup (`zerolog`, `zap`).
  - OpenTelemetry integration for tracing.
- **Key Data Structures (Go examples):**

  ```go
  type PlaceOrderRequest struct {
      UserID         string  `json:"userId" validate:"required"`
      Symbol         string  `json:"symbol" validate:"required,alphanum,uppercase"`
      Quantity       float64 `json:"quantity" validate:"required,gt=0"` // Use decimal types in real impl
      Side           string  `json:"side" validate:"required,oneof=BUY SELL"`
      IdempotencyKey string  `header:"X-Idempotency-Key"` // Optional but recommended
  }

  type PortfolioData struct { // Simplified view needed for validation
      CashBalance float64
      Holdings    map[string]float64
  }

  // Kafka Event structure defined in Section 4
  ```

- **Interface Definitions:**
  - _Consumes:_ HTTP `POST /orders` (Section 6.1).
  - _Produces:_ Kafka event to `trade_orders` topic (Section 4.2).
  - _Interacts With:_ Redis (Get latest price), DynamoDB (Get portfolio for validation).
- **Error Handling Specifics:**
  - Request Validation Fail (e.g., missing field, invalid side): HTTP 400 Bad Request with details.
  - Idempotency Key Conflict (if implemented and key reused with different payload): HTTP 409 Conflict.
  - Insufficient Funds (Buy order): HTTP 402 Payment Required.
  - Insufficient Shares (Sell order): HTTP 422 Unprocessable Entity.
  - Price Fetch Error (Redis): Log error, return HTTP 503 Service Unavailable (transient dependency issue).
  - Portfolio Fetch Error (DynamoDB): Log error, return HTTP 503 Service Unavailable.
  - Kafka Publish Error: Log error, implement retry (with backoff), if persistent failure return HTTP 500 Internal Server Error.
  - Panic Recovery: Middleware to catch panics, log stack trace, return HTTP 500.
- **Local State:** None (Stateless).
- **External Dependencies:** API Gateway, Kafka, Redis, DynamoDB, Logging/Tracing systems.

**2.3. Portfolio Service (Go)**

- **Internal Structure:**
  - HTTP server.
  - Handler for `GET /portfolio/{userId}`.
  - Redis client.
  - DynamoDB client.
  - Logging & Tracing.
- **Key Data Structures (Go examples):**
  ```go
  type PortfolioResponse struct {
      UserID       string             `json:"userId"`
      CashBalance  float64            `json:"cashBalance"` // Use decimal types
      Holdings     map[string]float64 `json:"holdings"`    // Map symbol to quantity
      LastUpdated  time.Time          `json:"lastUpdated"` // From DB or Cache meta
  }
  ```
- **Interface Definitions:**
  - _Consumes:_ HTTP `GET /portfolio/{userId}` (Section 6.2).
  - _Produces:_ HTTP Response with portfolio data.
  - _Interacts With:_ DynamoDB (primary source), Redis (optional caching layer for portfolios).
- **Error Handling Specifics:**
  - User Not Found (DynamoDB): HTTP 404 Not Found.
  - DynamoDB/Redis Read Error: Log error, return HTTP 503 Service Unavailable.
- **Local State:** None (Stateless).
- **External Dependencies:** API Gateway, DynamoDB, Redis (optional cache), Logging/Tracing systems.

**2.4. Query Service (Go)**

- **Internal Structure:**
  - HTTP server.
  - Handler for `GET /trades/search`.
  - Handler for `GET /analytics/volume`.
  - Elasticsearch client (`elastic/go-elasticsearch`).
  - Clickhouse client (`ClickHouse/clickhouse-go`).
  - Logging & Tracing.
- **Key Data Structures (Go examples):**

  ```go
  // For /trades/search
  type TradeHistoryQuery struct {
      UserID string `query:"userId" validate:"required"`
      Symbol string `query:"symbol"`
      // Add date range, pagination etc.
  }
  type TradeRecord struct { // Matches Elasticsearch structure (Section 5.3)
      // ... fields ...
  }
  type TradeHistoryResponse struct {
      Trades []TradeRecord `json:"trades"`
      // Add pagination info
  }

  // For /analytics/volume
  type VolumeAnalyticsQuery struct {
      Symbol     string `query:"symbol"`
      TimeWindow string `query:"timeWindow" validate:"oneof=1h 24h 7d"` // Example
  }
  type VolumeDataPoint struct {
      Symbol string  `json:"symbol"`
      Volume float64 `json:"volume"` // Use decimal types
      Period string  `json:"period"`
  }
  type VolumeAnalyticsResponse struct {
      Data []VolumeDataPoint `json:"data"`
  }
  ```

- **Interface Definitions:**
  - _Consumes:_ HTTP `GET /trades/search`, `GET /analytics/volume` (Section 6.3, 6.4).
  - _Produces:_ HTTP responses with history or analytics data.
  - _Interacts With:_ Elasticsearch, Clickhouse.
- **Error Handling Specifics:**
  - Invalid Query Params: HTTP 400 Bad Request.
  - Elasticsearch Query Error: Log error, return HTTP 500 Internal Server Error.
  - Clickhouse Query Error: Log error, return HTTP 500 Internal Server Error.
- **Local State:** None (Stateless).
- **External Dependencies:** API Gateway, Elasticsearch, Clickhouse, Logging/Tracing systems.

**2.5. Market Data Service (Elixir)**

- **Internal Structure:**
  - OTP Application.
  - Kafka Consumer (`brod` or `kafka_ex`). GenServer managing consumption from `market_data`.
  - Redis Client (`redix`). Used for `SET` price and `PUBLISH` updates.
  - WebSocket Handler (`Phoenix Channels` or Cowboy handler). Manages client connections and subscriptions.
  - Internal PubSub Subscriber (e.g., `Phoenix.PubSub` or Redis subscriber) to receive price updates published via Redis Pub/Sub.
  - Supervisor tree for fault tolerance.
  - Logging & Tracing integration.
- **Key Data Structures (Elixir examples):**

  ```elixir
  # Kafka message (decoded)
  defstruct [:symbol, :price, :timestamp]

  # Internal representation / Redis PubSub message
  defstruct [:symbol, :price]

  # WebSocket message to client
  %{event: "price_update", topic: "prices:MOCKAAPL", payload: %{symbol: "MOCKAAPL", price: 150.75}}
  ```

- **Interface Definitions:**
  - _Consumes:_ Kafka events from `market_data` (Section 4.1), WebSocket connection requests, WebSocket channel subscriptions (e.g., `prices:{symbol}`).
  - _Produces:_ Updates to Redis Cache (Section 5.2), messages to Redis Pub/Sub (`price_updates` channel), WebSocket messages to clients.
  - _Interacts With:_ Kafka, Redis (Cache & Pub/Sub).
- **Error Handling Specifics:**
  - Kafka Consumer Error: Log error, rely on supervisor to restart consumer process.
  - Redis Connection Error: Log error, attempt reconnection with backoff. May degrade service (no price updates).
  - WebSocket Handler Error: Log error, connection likely terminates, client needs to reconnect.
  - Message Processing Error (e.g., bad format from Kafka): Log error, skip message (or send to Dead Letter Queue).
- **Local State:** WebSocket connection state (managed by Phoenix Channels/WebSocket handler). Consumer offsets (managed by Kafka client library/broker).
- **External Dependencies:** Kafka, Redis, Logging/Tracing systems.

**2.6. Trade Execution Service (Flink Job - Java/Scala)**

- **Internal Structure:**
  - Flink Streaming Job (DataStream API).
  - Kafka Source Connector (`FlinkKafkaConsumer`) for `trade_orders`.
  - Redis Lookup (e.g., Async I/O with a Redis client like Lettuce) or Rich Function accessing Redis client for price fetching.
  - DynamoDB Sink (`DynamoDbSink` or custom RichSinkFunction using AWS SDK) for portfolio updates. Must use conditional writes.
  - Kafka Sink Connector (`FlinkKafkaProducer`) for `trade_executed`.
  - State Management (if needed, e.g., for complex user-specific logic, but direct DB update might suffice).
  - Configuration for Checkpointing (Interval, Mode: EXACTLY_ONCE, State Backend: e.g., RocksDB, Checkpoint Storage: e.g., S3).
  - Logging & Tracing integration (via Flink's logging framework, potentially passing trace headers).
- **Key Data Structures (Java examples):**

  ```java
  // Input from trade_orders Kafka topic
  public class TradeOrderEvent {
      public String eventId;
      public String orderId;
      public String userId;
      public String symbol;
      public double quantity; // Use BigDecimal
      public String side; // "BUY" or "SELL"
      public long timestamp;
      // Include trace headers if possible
  }

  // Output to trade_executed Kafka topic
  public class TradeExecutedEvent {
      public String eventId; // New event ID
      public String orderId; // Original order ID
      public String userId;
      public String symbol;
      public double quantity; // Use BigDecimal
      public String side;
      public double executionPrice; // Use BigDecimal
      public long executionTimestamp;
      public String status; // "EXECUTED", "FAILED_INSUFFICIENT_FUNDS", "FAILED_INSUFFICIENT_SHARES"
      // Include trace headers
  }
  ```

- **Interface Definitions:**
  - _Consumes:_ Kafka events from `trade_orders` (Section 4.2).
  - _Produces:_ Kafka events to `trade_executed` (Section 4.3), updates to DynamoDB Portfolio table (Section 5.1).
  - _Interacts With:_ Kafka, Redis (price lookup), DynamoDB (portfolio update).
- **Error Handling Specifics:**
  - Deserialization Error (Kafka Source): Log error, potentially skip bad record or fail job depending on config.
  - Price Lookup Error (Redis): Retry (Async I/O helps), if persistent, fail the order (publish FAILED event to `trade_executed`), log error.
  - DynamoDB Conditional Write Failure (`ConditionalCheckFailedException`): Indicates concurrent modification or insufficient balance/shares. Publish FAILED event (`FAILED_INSUFFICIENT_FUNDS`/`_SHARES`) to `trade_executed`, log event. Do **not** retry automatically without re-reading state.
  - DynamoDB Other Error: Retry with backoff, if persistent, fail the Flink job. Relies on Flink checkpointing/restart.
  - Kafka Sink Error: Configure sink for retries. Use transactional/idempotent producer for EOS. If persistent, fail the job.
  - Job Failure/Restart: Flink restarts from last successful checkpoint, reprocessing data as needed (source offsets reset).
- **Local State:** Managed by Flink's state backends if stateful operators are used. Consumer offsets managed via checkpoints.
- **External Dependencies:** Kafka, Redis, DynamoDB, Checkpoint Storage (e.g., S3), Logging/Tracing systems.

**2.7. Persistence Services (Flink Jobs / Kafka Consumers)**

- **Structure:** Similar to Trade Execution Service, but simpler.
  - Kafka Source (`trade_executed`).
  - Elasticsearch Sink (`ElasticsearchSink`).
  - Clickhouse Sink (e.g., `JdbcSink` or dedicated Clickhouse connector). May involve aggregation logic within Flink if sinking pre-aggregated data.
  - Checkpointing configured.
- **Interfaces:** Consume `trade_executed`, Produce data to Elasticsearch/Clickhouse.
- **Error Handling:** Sink-specific errors (connection, write failures). Configure retries. For persistent errors, fail the job or implement Dead Letter Queue for failed records. EOS configuration recommended.
- **State:** Stateful if performing aggregations for Clickhouse within Flink, otherwise stateless sinks.

**3. Data Flow Specifications**

**3.1. Market Data Flow**

- **Sequence:**

  ```mermaid
  sequenceDiagram
      participant Feed as External Feed
      participant KafkaMarket as Kafka (market_data)
      participant ElixirSvc as Market Data Service (Elixir)
      participant RedisCache as Redis Cache
      participant RedisPubSub as Redis Pub/Sub
      participant ClientWS as Client (WebSocket)

      Feed->>KafkaMarket: Publish {symbol, price, ts}
      ElixirSvc->>KafkaMarket: Consume message
      ElixirSvc->>RedisCache: SET price:{symbol} = price
      ElixirSvc->>RedisPubSub: PUBLISH price_updates {symbol, price}
      ElixirSvc->>RedisPubSub: Subscribe price_updates
      RedisPubSub-->>ElixirSvc: Receive {symbol, price}
      ElixirSvc->>ClientWS: Push WebSocket {event: "price_update", ...}
  ```

- **Data Transformation:** Raw feed data (format TBD) -> Kafka JSON message (Schema 4.1) -> Internal Elixir struct -> Redis SET command -> Redis PUBLISH command -> WebSocket JSON message.
- **Validation:** Basic schema validation on Kafka message consumption.
- **Error Handling:** Kafka consumer errors handled by supervisor restart. Redis errors logged, may cause temporary stale prices or lack of broadcast. WebSocket errors terminate individual client connection.
- **Retry:** Redis operations can be retried with backoff. Kafka consumption retries handled by consumer group rebalance/restart.

**3.2. Order Placement Flow**

- **Sequence:**

  ```mermaid
  sequenceDiagram
      participant Client
      participant APIGW as API Gateway
      participant OrderSvc as Order Service (Go)
      participant RedisCache as Redis Cache
      participant DynamoDB
      participant KafkaOrders as Kafka (trade_orders)

      Client->>APIGW: POST /orders (payload, idempotencyKey?)
      APIGW->>OrderSvc: Forward Request
      OrderSvc->>OrderSvc: Validate Request Schema & Idempotency Key
      alt Validation OK
          OrderSvc->>RedisCache: GET price:{symbol}
          RedisCache-->>OrderSvc: Latest Price
          OrderSvc->>DynamoDB: GET Portfolio (userId) - Strongly Consistent Read? or Eventually Consistent?
          DynamoDB-->>OrderSvc: Portfolio Data (cash, holdings)
          OrderSvc->>OrderSvc: Validate Funds/Shares vs Price*Qty
          alt Funds/Shares OK
               OrderSvc->>KafkaOrders: Publish trade_order event (Schema 4.2)
               KafkaOrders-->>OrderSvc: Ack (Sync/Async)
               OrderSvc-->>APIGW: HTTP 202 Accepted {orderId}
          else Insufficient Funds/Shares
               OrderSvc-->>APIGW: HTTP 402/422 Error
          end
      else Validation Fail
          OrderSvc-->>APIGW: HTTP 400/409 Error
      end
      APIGW-->>Client: HTTP Response
  ```

- **Data Transformation:** HTTP JSON Request -> Go `PlaceOrderRequest` struct -> Kafka `TradeOrderEvent` JSON (Schema 4.2).
- **Validation:**
  1.  HTTP Request Schema (Types, required fields, enums).
  2.  Idempotency Key check (if present).
  3.  Fetch latest price from Redis.
  4.  Fetch portfolio from DynamoDB (Consider read consistency needs - likely eventually consistent is okay, rely on Flink + conditional writes for final check).
  5.  Check `cash >= price * quantity` (Buy) or `holdings[symbol] >= quantity` (Sell). Allow for small buffer/margin? (Keep simple: No).
- **Error Handling:** As specified in Order Service (Section 2.2). 4xx errors for validation/business rule failures, 5xx for internal/dependency issues.
- **Retry:** Kafka publish errors retried internally by Order Service. Client responsible for retrying on 5xx or timeouts (using idempotency key).

**3.3. Order Execution Flow**

- **Sequence:**

  ```mermaid
  sequenceDiagram
      participant KafkaOrders as Kafka (trade_orders)
      participant FlinkExec as Trade Execution Job (Flink)
      participant RedisCache as Redis Cache
      participant DynamoDB
      participant KafkaExecuted as Kafka (trade_executed)

      FlinkExec->>KafkaOrders: Consume trade_order event
      FlinkExec->>RedisCache: GET price:{symbol}
      RedisCache-->>FlinkExec: Execution Price
      FlinkExec->>DynamoDB: Conditional UPDATE Portfolio (userId) based on side, quantity, price
      alt Conditional Update OK
          DynamoDB-->>FlinkExec: Success
          FlinkExec->>KafkaExecuted: Publish trade_executed event (EXECUTED) (Schema 4.3)
      else Conditional Update Fails (e.g., balance check fail)
          DynamoDB-->>FlinkExec: ConditionalCheckFailedException
          FlinkExec->>KafkaExecuted: Publish trade_executed event (FAILED_INSUFFICIENT_...) (Schema 4.3)
      end
  ```

- **Data Transformation:** Kafka `TradeOrderEvent` (Schema 4.2) -> Internal Flink representation -> DynamoDB UpdateItemInput -> Kafka `TradeExecutedEvent` (Schema 4.3).
- **Validation:** Primary validation is the DynamoDB conditional write, which re-checks balance/holdings atomically during the update attempt.
- **Error Handling:** As specified in Trade Execution Service (Section 2.6). Conditional failures result in FAILED status events. Dependency errors (Redis, Kafka Sink) retried; persistent errors fail the job for Flink HA restart.
- **Retry:** Flink handles source/sink retries based on config. DynamoDB conditional failures are _not_ retried automatically by the job logic (they represent a business-level failure).

**4. Event Schemas (Kafka Topics)**

- **Versioning Approach:** Include a `schemaVersion` field (e.g., "1.0") in all events. Handle schema evolution carefully (e.g., adding optional fields is backward compatible).
- **Common Fields:** `eventId` (UUID), `eventTimestamp` (ISO 8601 UTC), `sourceService` (e.g., "OrderService", "FlinkExecutionJob"), `schemaVersion`.

**4.1. `market_data` Topic**

- **Structure (JSON):**
  ```json
  {
    "eventId": "uuid-...",
    "eventTimestamp": "2025-04-02T10:30:00.123Z",
    "sourceService": "MarketFeedSimulator",
    "schemaVersion": "1.0",
    "payload": {
      "symbol": "MOCKAAPL",
      "price": 150.75, // Use string for precision if needed
      "feedTimestamp": "2025-04-02T10:29:59.995Z" // Timestamp from source
    }
  }
  ```
- **Required:** `eventId`, `eventTimestamp`, `sourceService`, `schemaVersion`, `payload.symbol`, `payload.price`, `payload.feedTimestamp`.
- **Optional:** None defined.

**4.2. `trade_orders` Topic**

- **Structure (JSON):**
  ```json
  {
    "eventId": "uuid-...",
    "eventTimestamp": "2025-04-02T10:35:01.456Z",
    "sourceService": "OrderService",
    "schemaVersion": "1.0",
    "payload": {
      "orderId": "order-uuid-...", // Generated by OrderService
      "userId": "user-123",
      "symbol": "MOCKGOOG",
      "quantity": 10.0, // Use string for precision
      "side": "BUY", // "BUY" or "SELL"
      "orderTimestamp": "2025-04-02T10:35:01.450Z" // When order was placed
    },
    "traceContext": {
      /* Optional: OpenTelemetry trace headers */
    }
  }
  ```
- **Required:** All fields within `payload`, plus common fields.
- **Optional:** `traceContext`.

**4.3. `trade_executed` Topic**

- **Structure (JSON):**
  ```json
  {
    "eventId": "uuid-...",
    "eventTimestamp": "2025-04-02T10:35:03.789Z", // Flink processing time
    "sourceService": "FlinkExecutionJob",
    "schemaVersion": "1.0",
    "payload": {
      "orderId": "order-uuid-...", // From incoming event
      "userId": "user-123",
      "symbol": "MOCKGOOG",
      "quantity": 10.0, // Use string for precision
      "side": "BUY",
      "status": "EXECUTED", // "EXECUTED", "FAILED_INSUFFICIENT_FUNDS", "FAILED_INSUFFICIENT_SHARES", "FAILED_OTHER"
      "executionPrice": 1950.25, // Use string for precision. Null if status != EXECUTED
      "executionTimestamp": "2025-04-02T10:35:03.780Z", // When Flink processed
      "failureReason": null // String description if status starts with FAILED_
    },
    "traceContext": {
      /* Optional: OpenTelemetry trace headers */
    }
  }
  ```
- **Required:** All fields within `payload` except `executionPrice` and `failureReason` (which depend on `status`), plus common fields.
- **Optional:** `traceContext`, `executionPrice`, `failureReason`.

**5. Storage Schemas**

**5.1. DynamoDB: `Portfolios` Table**

- **Table Name:** `trading_portfolios`
- **Primary Key:** `userId` (String) - Partition Key
- **Attributes:**
  - `userId` (String)
  - `cashBalance` (Number) - Use Decimal type if supported by SDK/mapper, otherwise store as scaled integer (e.g., cents) or rely on Number precision. _Crucial: Ensure consistent handling._
  - `holdings` (Map<String, Number>) - Map where key is `symbol` (String), value is quantity (Number - same precision concerns as cashBalance).
  - `lastUpdatedTimestamp` (String - ISO 8601 or Number - Epoch millis)
  - `version` (Number) - Optional, for application-level optimistic locking if needed beyond conditional writes.
- **Indexes:** None required initially (Primary Key lookup is sufficient).
- **Query Patterns:** GetItem by `userId`, Conditional UpdateItem by `userId`.
- **Example Record (JSON representation):**
  ```json
  {
    "userId": { "S": "user-123" },
    "cashBalance": { "N": "10000.50" }, // Storing as string Number for precision
    "holdings": {
      "M": {
        "MOCKAAPL": { "N": "50.0" },
        "MOCKGOOG": { "N": "25.5" }
      }
    },
    "lastUpdatedTimestamp": { "S": "2025-04-02T09:00:00Z" },
    "version": { "N": "1" }
  }
  ```

**5.2. Redis Cache: Market Prices**

- **Structure:** Simple Key-Value.
- **Key:** `price:{symbol}` (e.g., `price:MOCKAAPL`)
- **Value:** String representation of the latest price (e.g., "150.75"). Using String avoids floating point issues.
- **TTL:** Optional, e.g., 24 hours, to prune symbols not seen recently.
- **Query Patterns:** `GET price:{symbol}`. `SET price:{symbol} value [EX ttl]`.

**5.3. Elasticsearch: `Trades` Index**

- **Index Name:** `trading_trades-{yyyy-MM}` (e.g., `trading_trades-2025-04`) - Time-based indices. Use alias (e.g., `trading_trades`) for querying across indices.
- **Document Structure (Mapping):**
  ```json
  {
    "properties": {
      "eventId": { "type": "keyword" },
      "orderId": { "type": "keyword" },
      "userId": { "type": "keyword" },
      "symbol": { "type": "keyword" },
      "quantity": { "type": "double" }, // Or scaled_float
      "side": { "type": "keyword" }, // BUY, SELL
      "status": { "type": "keyword" }, // EXECUTED, FAILED_...
      "executionPrice": { "type": "double" }, // Or scaled_float
      "executionTimestamp": {
        "type": "date",
        "format": "strict_date_optional_time||epoch_millis"
      },
      "failureReason": { "type": "text" }
      // Add trace context fields if needed
    }
  }
  ```
- **Query Patterns:** Term queries on `userId`, `symbol`, `status`, `orderId`. Range queries on `executionTimestamp`. Full-text search on `failureReason`. Sorting by `executionTimestamp`.
- **Example Document (\_source):**
  ```json
  {
    "eventId": "uuid-...",
    "orderId": "order-uuid-...",
    "userId": "user-123",
    "symbol": "MOCKGOOG",
    "quantity": 10.0,
    "side": "BUY",
    "status": "EXECUTED",
    "executionPrice": 1950.25,
    "executionTimestamp": "2025-04-02T10:35:03.780Z",
    "failureReason": null
  }
  ```

**5.4. Clickhouse: `Trades` Table**

- **Table Name:** `trading_trades`
- **Engine:** `MergeTree()` partitioned by `toDate(executionTimestamp)` ordered by (`symbol`, `userId`, `executionTimestamp`). Or `ReplacingMergeTree` if updates are possible (unlikely for trades).
- **Schema:**
  ```sql
  CREATE TABLE trading_trades (
      eventId String,
      orderId String,
      userId String,
      symbol String,
      quantity Decimal64(4), -- Example: 4 decimal places
      side Enum8('BUY' = 1, 'SELL' = 2),
      status Enum8('EXECUTED' = 1, 'FAILED_INSUFFICIENT_FUNDS' = 2, 'FAILED_INSUFFICIENT_SHARES' = 3, 'FAILED_OTHER' = 4),
      executionPrice Nullable(Decimal64(4)), -- Nullable
      executionTimestamp DateTime64(3, 'UTC'), -- Millisecond precision
      failureReason Nullable(String)
  ) ENGINE = MergeTree()
  PARTITION BY toDate(executionTimestamp)
  ORDER BY (symbol, userId, executionTimestamp);
  ```
- **Query Patterns:** Aggregations (SUM, COUNT, AVG) grouped by `symbol`, `userId`, `side`, `status`, time windows. Filtering by `symbol`, `userId`, `executionTimestamp`.
- **Example Query:**
  ```sql
  SELECT symbol, SUM(quantity) as total_volume
  FROM trading_trades
  WHERE executionTimestamp >= now() - INTERVAL 1 HOUR AND status = 'EXECUTED'
  GROUP BY symbol;
  ```

**6. API Specifications**

- **Authentication:** Assumed handled by API Gateway (e.g., validating JWT and passing `userId` downstream, perhaps in a custom header). Services should expect authenticated requests.
- **Rate Limiting:** Defined at API Gateway level (e.g., per user/IP). Services may implement finer-grained limits if needed.
- **Base URL:** `https://api.mocktrading.example.com/v1` (Configured in API Gateway)

**6.1. Order API (`Order Service`)**

- **Endpoint:** `POST /orders`
- **Request Body (application/json):**
  ```json
  {
    "userId": "user-123", // Often injected by Gateway/Auth layer, not sent by client
    "symbol": "MOCKAAPL",
    "quantity": 10.5,
    "side": "BUY"
  }
  ```
- **Headers:** `X-Idempotency-Key: <unique-key-per-request>` (Optional but recommended). `Authorization: Bearer <jwt>` (Handled by Gateway).
- **Success Response (HTTP 202 Accepted):**
  ```json
  {
    "orderId": "order-uuid-...",
    "status": "ACCEPTED"
  }
  ```
- **Error Responses:**
  - `HTTP 400 Bad Request`: Invalid schema, missing fields, invalid `side`. Payload includes error details.
  - `HTTP 401 Unauthorized`: Invalid/missing auth token (Handled by Gateway).
  - `HTTP 402 Payment Required`: Insufficient funds for buy order.
  - `HTTP 403 Forbidden`: User not allowed to trade (Handled by Gateway/Auth).
  - `HTTP 409 Conflict`: Idempotency key reused with different request payload.
  - `HTTP 422 Unprocessable Entity`: Insufficient shares for sell order.
  - `HTTP 429 Too Many Requests`: Rate limit exceeded (Handled by Gateway).
  - `HTTP 500 Internal Server Error`: Kafka unavailable, unexpected errors.
  - `HTTP 503 Service Unavailable`: Cannot reach Redis/DynamoDB for validation.

**6.2. Portfolio API (`Portfolio Service`)**

- **Endpoint:** `GET /portfolio/{userId}`
- **Path Parameter:** `userId` (String). Often validated against authenticated user by Gateway/Service.
- **Headers:** `Authorization: Bearer <jwt>`.
- **Success Response (HTTP 200 OK):**
  ```json
  {
    "userId": "user-123",
    "cashBalance": 10000.5,
    "holdings": {
      "MOCKAAPL": 50.0,
      "MOCKGOOG": 25.5
    },
    "lastUpdatedTimestamp": "2025-04-02T09:00:00Z"
  }
  ```
- **Error Responses:**
  - `HTTP 401 Unauthorized`.
  - `HTTP 403 Forbidden` (If requesting portfolio for another user).
  - `HTTP 404 Not Found`: User portfolio does not exist.
  - `HTTP 503 Service Unavailable`: Cannot reach DynamoDB/Redis.

**6.3. Trade History API (`Query Service`)**

- **Endpoint:** `GET /trades/search`
- **Query Parameters:**
  - `userId` (String, required - usually validated against authenticated user).
  - `symbol` (String, optional).
  - `startDate`, `endDate` (String - ISO 8601, optional).
  - `limit` (Integer, optional, default 100).
  - `offset` / `pageToken` (String/Integer, optional - for pagination).
- **Headers:** `Authorization: Bearer <jwt>`.
- **Success Response (HTTP 200 OK):**
  ```json
  {
    "trades": [
      {
        "eventId": "uuid-...",
        "orderId": "...",
        "userId": "...",
        "symbol": "...",
        "quantity": 10.0,
        "side": "BUY",
        "status": "EXECUTED",
        "executionPrice": 1950.25,
        "executionTimestamp": "...",
        "failureReason": null
      }
      // ... more trades
    ],
    "pagination": {
      // Example pagination
      "nextPageToken": "...",
      "totalCount": 123
    }
  }
  ```
- **Error Responses:**
  - `HTTP 400 Bad Request`: Invalid query parameters (e.g., bad date format).
  - `HTTP 401 Unauthorized`.
  - `HTTP 403 Forbidden`.
  - `HTTP 500 Internal Server Error`: Error querying Elasticsearch.

**6.4. Analytics API (`Query Service`)**

- **Endpoint:** `GET /analytics/volume`
- **Query Parameters:**
  - `symbol` (String, optional - filter by symbol).
  - `timeWindow` (String, optional, e.g., "1h", "24h", "7d", default "24h").
- **Headers:** `Authorization: Bearer <jwt>` (Maybe not needed if analytics are public?).
- **Success Response (HTTP 200 OK):**
  ```json
  {
    "data": [
      { "symbol": "MOCKAAPL", "volume": 12345.5, "timeWindow": "24h" },
      { "symbol": "MOCKGOOG", "volume": 6789.0, "timeWindow": "24h" }
      // ... more symbols if symbol param omitted
    ]
  }
  ```
- **Error Responses:**
  - `HTTP 400 Bad Request`: Invalid `timeWindow` format.
  - `HTTP 500 Internal Server Error`: Error querying Clickhouse.

**6.5. Market Data API (WebSocket - `Market Data Service`)**

- **Endpoint:** `wss://api.mocktrading.example.com/v1/marketdata`
- **Connection:** Standard WebSocket handshake.
- **Client->Server Messages:**
  - Subscribe: `{"event": "phx_join", "topic": "prices:MOCKAAPL", "payload": {}, "ref": "1"}` (Example using Phoenix channels format - adapt if using plain WebSockets). Client subscribes to topics like `prices:{symbol}`.
  - Unsubscribe: `{"event": "phx_leave", "topic": "prices:MOCKAAPL", "payload": {}, "ref": "2"}`
- **Server->Client Messages:**
  - Price Update: `{"event": "price_update", "topic": "prices:MOCKAAPL", "payload": {"symbol": "MOCKAAPL", "price": 150.80}, "ref": null}`
  - Subscription Confirmation/Rejection: Standard WebSocket protocol messages or framework-specific replies (e.g., `phx_reply`).
- **Error Handling:** Connection errors handled by WebSocket protocol. Server may close connection on invalid messages or auth failure during handshake.

**7. Distributed Systems Patterns & Concerns**

**7.1. Distributed Transaction Patterns**

The system maintains consistency without distributed transactions through event sourcing and local atomic operations. Each operation that requires cross-component consistency is modeled as a sequence of local transactions with clear compensating actions.

**7.1.1. Order Execution Consistency**

- **Transaction Boundary:**

  - Core atomic unit: Portfolio state update in DynamoDB
  - Event publishing must be consistent with state changes
  - Price lookups are eventually consistent reads

- **Required Guarantees:**

  - Portfolio updates must be atomic
  - Balance/holdings validation must occur in the same atomic operation as the update
  - No lost or duplicated executions
  - Clear event trail of execution attempts and results

- **State Management Requirements:**
  - Conditional write for portfolio updates
  - Exactly-once processing for trade execution
  - Versioning or timestamps for state tracking
  - Clear rollback/compensation procedures

**7.1.2. Failure Mode Analysis**

_Pre-Execution Failures:_

- Price lookup failures must not affect portfolio consistency
- Portfolio read failures must not result in invalid state
- System must handle transient failures through retries
- Clear distinction between transient and permanent failures

_Execution Failures:_

- Failed portfolio updates must not leave inconsistent state
- System must handle concurrent modification attempts
- Clear tracking of partially completed operations
- Automatic compensation for incomplete transactions

_Post-Execution Failures:_

- Event publishing failures must not orphan state changes
- System must maintain order execution history
- Clear reconciliation path for interrupted operations

**7.1.3. Event Ordering Requirements**

_Required Ordering Guarantees:_

- Price updates must be processed in order per symbol
- Orders must be processed in sequence per user
- Execution events must maintain causal ordering
- Analytics events can be eventually consistent

_Partitioning Strategy Requirements:_

- Market data partitioned by symbol
- Trade operations partitioned by user
- Analytics can use various partitioning schemes

**7.2. Testing Strategy**

**7.2.1. Distributed Systems Testing Requirements**

_Consistency Testing:_

- Verify portfolio consistency under concurrent operations
- Test ordering guarantees across components
- Validate event sequences and causality
- Verify state consistency after failures

_Performance Testing:_

- Measure latency across distributed operations
- Verify throughput under normal conditions
- Test system behavior under load
- Measure recovery time after failures

_Failure Testing:_

- Simulate component failures
- Test network partition scenarios
- Verify data consistency after recovery
- Test partial failure handling

**7.2.2. Observability Requirements**

_Required Metrics:_

- Component health indicators
- Processing latencies
- Queue depths and processing lag
- Error rates and types
- Resource utilization

_Logging Requirements:_

- Consistent correlation IDs across components
- Clear event sequencing information
- State transition logging
- Error context preservation

_Tracing Requirements:_

- End-to-end operation tracking
- Component interaction recording
- Latency breakdown visibility
- Error propagation tracking

**7.2.3. Recovery Procedures**

_Recovery Requirements:_

- Clear service recovery order
- State reconstruction procedures
- Data reconciliation processes
- Client notification mechanisms

_Required Recovery Capabilities:_

- Point-in-time state recovery
- Event replay capabilities
- Partition tolerance
- Clear consistency verification
  **7.3. Schema Evolution and Versioning**

**7.3.1. Event Schema Evolution Requirements**

_Compatibility Rules:_

- Backward compatibility required for all changes
- Forward compatibility where possible
- Clear version identification in all events
- Support for multiple concurrent versions during transitions

_Prohibited Changes:_

- Removing required fields
- Changing field types
- Semantic changes to existing fields
- Breaking changes to event structure

_Required Evolution Capabilities:_

- Version detection in consumers
- Graceful handling of unknown fields
- Clear upgrade path for each schema change
- Monitoring of schema version distribution

**7.3.2. State Schema Evolution**

_Portfolio State Evolution:_

- Must maintain consistency during schema changes
- Clear migration strategy for existing data
- No downtime requirements for schema updates
- Version tracking for portfolio records

_Search/Analytics Evolution:_

- Support for reindexing existing data
- Clear schema versioning strategy
- Handling of mixed-version queries
- Migration verification procedures

**7.4. System Invariants and Consistency**

**7.4.1. Required Portfolio Invariants**

_Balance Constraints:_

- Non-negative cash balance
- Non-negative holdings
- Sum of executed trades matches balance changes
- Holdings consistent with trade history

_Execution Invariants:_

- No duplicate order executions
- No lost orders
- No phantom trades
- Clear audit trail of all changes

**7.4.2. Event Stream Invariants**

_Stream Consistency:_

- No gaps in event sequences
- Causal ordering maintained
- Clear event parentage
- Complete execution chains

_Cross-Stream Validation:_

- Market data consistency checks
- Trade execution validation
- Portfolio update verification
- Analytics consistency

**7.4.3. Monitoring and Verification**

_Required Consistency Checks:_

- Regular portfolio balance validation
- Event sequence verification
- Cross-system state comparison
- Audit trail verification

_Verification Timing:_

- Real-time constraint checking
- Periodic full verification
- Post-recovery validation
- Migration consistency checks

**7.5. System Boundaries and Failure Domains**

**7.5.1. Component Isolation Requirements**

_Service Isolation:_

- Independent failure modes
- Clear responsibility boundaries
- Separate scaling characteristics
- Isolated state management

_Required Bulkheads:_

- Market data processing isolation
- Order processing separation
- Analytics segregation
- Client connection management

**7.5.2. Cross-Boundary Interactions**

_Communication Patterns:_

- Clear retry strategies
- Circuit breaking requirements
- Timeout management
- Error propagation rules

_State Transfer Requirements:_

- Clear consistency boundaries
- State replication rules
- Cache consistency requirements
- Event propagation guarantees

**7.5.3. System Health Management**

_Health Check Requirements:_

- Component level checks
- Cross-component validation
- State consistency verification
- Performance monitoring

_Degradation Management:_

- Clear failure modes
- Graceful degradation paths
- Recovery priorities
- Client notification requirements

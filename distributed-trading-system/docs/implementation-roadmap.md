## Phased Implementation Plan: Mock Trading System

**Guiding Principles:**

- **Iterative Validation:** Each phase validates specific distributed patterns before building complexity.
- **Infrastructure First:** Core infrastructure (messaging, core persistence) is established early.
- **Flow-Based:** Implement end-to-end data flows incrementally (market data -> order -> execution -> query).
- **Test-Driven:** Emphasize testing distributed behavior at each stage.
- **Learning Focus:** Prioritize understanding patterns like event sourcing, CQRS, stream processing, and consistency models.

---

### Phase 0: Foundation & Core Infrastructure

- **Goal:** Set up the basic development environment and core messaging/caching infrastructure. Validate basic event publishing/consumption.
- **Pattern Focus:** Event Bus basics, Infrastructure-as-Code (IaC).
- **Environment Strategy:**
  - **Local:** Docker Compose managing Kafka (e.g., Redpanda/Strimzi Kafka container), Redis (consider starting with one instance, planning for split), basic DynamoDB Local, skeleton service containers. Define basic `docker-compose.yml`.
  - **Production Prerequisites:** None (Prod env setup starts later).
  - **IaC:** Basic Terraform/CloudFormation scripts initiated for VPC/networking fundamentals (if applicable), IAM roles. Start defining modules for Kafka, Redis.
  - **Testing:** Basic Kafka producer/consumer test scripts (e.g., Python/Go) to validate topic creation and message flow. Basic Redis connection tests.
  - **Monitoring:** Initial setup of local Prometheus/Grafana stack via Docker Compose (optional).
- **Dependencies:** None.
- **Deliverables:**
  - **Infrastructure:** Kafka cluster (local), Redis instance (local), DynamoDB `Portfolios` table schema defined (local). Basic IaC structure.
  - **Services:** None implemented. Skeleton projects created.
  - **APIs:** None.
  - **Data Flows:** Basic manual Kafka produce/consume validated.
  - **Tests:** Kafka connection/topic tests passing. Redis connection tests passing.
  - **Monitoring:** Local stack runnable (optional).
  - **CI/CD:** Basic pipeline structure (build, lint).
- **Validation Gate:** Can successfully produce to and consume from Kafka topics locally. Can connect to Redis and DynamoDB Local. Basic IaC applies cleanly.

---

### Phase 1: Market Data Flow & Real-time Push

- **Goal:** Implement the flow of market data from source to Kafka, processing, caching, and broadcasting via WebSockets. Validate event consumption and real-time push.
- **Pattern Focus:** Event-driven consumption, Caching (write-through), Pub/Sub pattern (via Redis), WebSocket communication.
- **Environment Strategy:**
  - **Local:** Add Market Data Ingestion Service and Market Data Service (Elixir) containers to Docker Compose. Configure connections to Kafka/Redis. Basic WebSocket client for testing.
  - **Production Prerequisites:** Foundational networking (VPC etc.). Decision on managed Kafka (MSK) vs. self-hosted. Decision on managed Redis (ElastiCache) vs. self-hosted.
  - **IaC:** Modules developed/tested for deploying Kafka (or configuring MSK), Redis (potentially split now or planned), DynamoDB table. Basic deployment mechanism (e.g., ECS/EKS Fargate profiles, EC2 ASG).
  - **Testing:** Integration tests simulating external feed -> Ingestion Service -> Kafka -> Elixir Service -> Redis Cache/PubSub -> WebSocket client. Validate price updates flow through. Test WebSocket connection handling.
  - **Monitoring:** Instrument Ingestion Service & Elixir Service with basic metrics (messages processed, connections, Redis latency) and structured logging. Add dashboards to local Grafana.
- **Dependencies:** Phase 0 completed.
- **Deliverables:**
  - **Infrastructure:** Kafka (`market_data` topic), Redis configured. Deployed locally via Docker Compose. IaC for prod Kafka/Redis/DynamoDB ready.
  - **Services:** Market Data Ingestion Service (v1), Market Data Service (Elixir - v1).
  - **APIs:** WebSocket endpoint for price subscriptions available.
  - **Data Flows:** Market data -> Kafka -> Elixir Service -> Redis Cache/PubSub -> WebSocket Client operational.
  - **Tests:** Market data flow integration tests passing.
  - **Monitoring:** Basic metrics/logs visible locally.
- **Validation Gate:** WebSocket clients receive price updates originating from the simulated feed. Redis cache contains latest prices. Services are containerized and run via Docker Compose. Prod IaC for core infra validates.

---

### Phase 2: Order Placement & Validation

- **Goal:** Implement the user-facing API for placing orders and the initial validation logic. Validate asynchronous request handling via Kafka.
- **Pattern Focus:** API Gateway integration, Request validation, Asynchronous processing initiation (Event publication), Dependency on external state (Cache/DB reads).
- **Environment Strategy:**
  - **Local:** Add API Gateway (e.g., local mock or basic config) and Order Service container to Docker Compose. Configure Order Service -> Kafka/Redis/DynamoDB connections.
  - **Production Prerequisites:** Deployed Kafka, Redis, DynamoDB from Phase 1. Initial API Gateway configuration.
  - **IaC:** API Gateway resource definitions added. IaC for Order Service deployment (e.g., ECS Service/Task Def).
  - **Testing:** API tests (e.g., Postman, code tests) for `POST /orders`. Validate successful acceptance (202 response, event on `trade_orders` topic) and rejections (4xx errors based on mock portfolio/price data). Test idempotency if implemented.
  - **Monitoring:** Instrument Order Service (request rate/latency, Kafka publish rate/errors, Redis/DynamoDB read latency). Update dashboards.
- **Dependencies:** Phase 1 completed. Kafka, Redis, DynamoDB available.
- **Deliverables:**
  - **Infrastructure:** API Gateway configured (locally/prod). Kafka `trade_orders` topic created.
  - **Services:** Order Service (v1).
  - **APIs:** `POST /orders` endpoint operational.
  - **Data Flows:** HTTP Request -> Order Service -> Validation -> Kafka `trade_orders` operational.
  - **Tests:** Order placement API tests passing (success/failure cases).
  - **Monitoring:** Order Service metrics/logs visible.
- **Validation Gate:** Valid orders result in messages on the `trade_orders` topic. Invalid orders are rejected with correct errors. Order Service runs reliably locally. Prod IaC validates.

---

### Phase 3: Basic Order Execution

- **Goal:** Introduce stream processing to consume orders and perform basic execution logic (without full atomicity yet). Validate Flink setup and basic stream consumption.
- **Pattern Focus:** Stream processing fundamentals (consumption, basic transformation), Reading external state within stream processor.
- **Environment Strategy:**
  - **Local:** Add Flink cluster (e.g., jobmanager/taskmanager containers) to Docker Compose. Develop basic Flink Execution Job. Configure Flink -> Kafka/Redis connections.
  - **Production Prerequisites:** Base compute environment for Flink (e.g., EKS, EMR, Kinesis Data Analytics/Managed Flink).
  - **IaC:** IaC for Flink cluster deployment or managed service configuration. Flink job deployment mechanism (e.g., build JAR, upload to S3, Flink CLI/API deploy).
  - **Testing:** Consume `trade_orders` in Flink job, log processing steps (price lookup, intended portfolio change), publish a simple `trade_executed` event (can just contain orderId + "PROCESSED" status initially). Validate events appear on `trade_executed`. Test basic job submission/management locally.
  - **Monitoring:** Integrate Flink metrics (records consumed, lag, checkpointing if enabled) into local monitoring. Add job-specific logs.
- **Dependencies:** Phase 2 completed. `trade_orders` topic populated.
- **Deliverables:**
  - **Infrastructure:** Flink cluster runnable (local). Kafka `trade_executed` topic created. Prod IaC for Flink environment.
  - **Services:** Flink Execution Job (v1 - basic processing).
  - **APIs:** None new.
  - **Data Flows:** Kafka `trade_orders` -> Flink Job -> Kafka `trade_executed` operational (basic processing).
  - **Tests:** Flink job consumes orders and produces basic execution events.
  - **Monitoring:** Basic Flink metrics visible locally.
- **Validation Gate:** Flink job successfully consumes from `trade_orders` and produces placeholder events to `trade_executed`. Job deployment works locally. Prod IaC validates.

---

### Phase 4: Atomic Portfolio Updates & Full Persistence

- **Goal:** Implement atomic portfolio updates using conditional writes and set up the persistence pipeline to Elasticsearch/Clickhouse. Validate consistency and reliable data sinking.
- **Pattern Focus:** Atomic state updates (Optimistic Concurrency Control via conditional writes), Exactly-Once Semantics (considerations/implementation), Reliable data sinking, CQRS write path completion.
- **Environment Strategy:**
  - **Local:** Add Elasticsearch and Clickhouse containers to Docker Compose. Enhance Flink Execution Job with DynamoDB conditional write logic. Develop Flink Persistence Job (single job, two sinks). Configure Flink -> DynamoDB/ES/CH connections.
  - **Production Prerequisites:** Deployed Flink environment. Decision on managed ES (OpenSearch)/Clickhouse vs. self-hosted.
  - **IaC:** IaC modules for ES/Clickhouse deployment/configuration. Update Flink job deployment scripts/IaC.
  - **Testing:**
    - **Execution Job:** Test conditional write success/failure scenarios (e.g., insufficient funds/shares). Inject concurrent conflicting updates for the same user to test atomicity. Validate `trade_executed` events reflect correct status (`EXECUTED`, `FAILED_...`).
    - **Persistence Job:** Validate `trade_executed` events are correctly written to ES and CH. Check data consistency. Test Flink job fault tolerance (restart job, check for duplicates/loss - requires EOS setup).
    - **Setup:** Test data generation scripts for portfolios and orders.
  - **Monitoring:** Monitor DynamoDB conditional check failures/throttling. Monitor ES/CH ingestion rates/errors. Monitor Flink checkpointing and end-to-end latency if EOS is enabled.
- **Dependencies:** Phase 3 completed. ES/CH infrastructure available.
- **Deliverables:**
  - **Infrastructure:** Elasticsearch, Clickhouse running (local). Prod IaC for ES/CH.
  - **Services:** Flink Execution Job (v2 - atomic updates), Flink Persistence Job (v1).
  - **APIs:** None new.
  - **Data Flows:** Full order execution flow with atomic updates operational. Persistence flow `trade_executed` -> ES/CH operational.
  - **Tests:** Atomicity tests, conditional write failure tests, persistence pipeline tests passing. EOS validation tests (if applicable).
  - **Monitoring:** DynamoDB write metrics, ES/CH ingestion metrics visible. Flink job health/checkpointing monitored.
- **Validation Gate:** Portfolio updates are atomic and correct. Trade history reliably appears in ES/CH. System recovers correctly from Flink job restarts (maintaining consistency).

---

### Phase 5: Querying & Analytics APIs

- **Goal:** Implement the read paths for portfolio, trade history, and analytics. Validate CQRS pattern.
- **Pattern Focus:** CQRS read paths, API design for queries, Data aggregation/querying from read stores.
- **Environment Strategy:**
  - **Local:** Add Portfolio Service and Query Service containers to Docker Compose. Configure connections to DynamoDB, ES, CH. Update API Gateway config.
  - **Production Prerequisites:** Deployed ES, CH, DynamoDB with data being populated.
  - **IaC:** API Gateway updates. IaC for Portfolio Service and Query Service deployments.
  - **Testing:** API tests for `GET /portfolio`, `GET /trades/search`, `GET /analytics/volume`. Validate data consistency (eventual consistency acceptable per NFR1.2) between write side (portfolio state) and read side (query results). Test filtering/pagination.
  - **Monitoring:** Instrument Portfolio and Query Services (request rate/latency, dependency query latency). Add API dashboards.
- **Dependencies:** Phase 4 completed. Read stores (DynamoDB, ES, CH) populated.
- **Deliverables:**
  - **Infrastructure:** API Gateway updated.
  - **Services:** Portfolio Service (v1), Query Service (v1).
  - **APIs:** `GET /portfolio`, `GET /trades/search`, `GET /analytics/volume` operational.
  - **Data Flows:** Read paths from APIs to respective data stores operational.
  - **Tests:** Query API tests passing. Basic data consistency checks passing.
  - **Monitoring:** Query service metrics/logs visible.
- **Validation Gate:** Users can view their portfolio, trade history, and basic analytics via the APIs. Data reflects processed trades (with acceptable eventual consistency).

---

### Phase 6: Production Hardening & Advanced Testing

- **Goal:** Ensure the system is observable, resilient, performant, and operationally ready.
- **Pattern Focus:** Observability, Resilience patterns (circuit breaking, retries), Performance tuning, Chaos engineering.
- **Environment Strategy:**
  - **Local:** Focus on enhancing test setups (e.g., failure injection tools like Toxiproxy, performance testing tools like k6). Refine local monitoring stack.
  - **Production Prerequisites:** All previous phases deployed to a staging/production environment.
  - **IaC:** Refine IaC for production readiness (security groups, IAM policies, logging/monitoring integration, scaling policies, health checks). Implement backup/restore scripts/IaC.
  - **Testing:**
    - Implement comprehensive distributed tracing.
    - Set up and run load/performance tests against key flows (order placement, price broadcast).
    - Implement failure injection tests (kill services, introduce latency, break dependencies) and validate system recovery and data consistency.
    - Test backup and recovery procedures.
  - **Monitoring:** Implement comprehensive alerting based on metrics and logs. Build consolidated dashboards covering end-to-end flows.
- **Dependencies:** Phase 5 completed and deployed.
- **Deliverables:**
  - **Infrastructure:** Production-ready IaC. Backup/recovery mechanisms implemented.
  - **Services:** Services potentially updated with circuit breakers, refined retry logic, improved instrumentation.
  - **APIs:** Stable, performant APIs.
  - **Data Flows:** Optimized and resilient flows.
  - **Tests:** Performance test suite, failure injection test suite passing. Recovery procedures validated.
  - **Monitoring:** Comprehensive monitoring and alerting operational.
- **Validation Gate:** System meets performance targets (NFR4). System recovers gracefully from simulated failures. Monitoring provides adequate visibility and alerts proactively. Backup/recovery works.

This phased approach allows for incremental building and testing, focusing on validating core distributed system concepts at each stage while progressing towards a fully functional system.

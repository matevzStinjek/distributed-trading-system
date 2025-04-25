Okay, here is a proposed roadmap/spec to incrementally build the market data ingestion service, focusing on getting a working pipeline first and then introducing abstractions, testing, and observability.

**Guiding Principles:**

* **Iterative Development:** Build and test functional slices.
* **Testability First:** Introduce mocks early enough to enable development/testing without relying on live market data.
* **Observability Driven:** Add monitoring as components are built to understand their behavior.
* **Configuration Management:** Externalize configuration from the start.
* **Deployment Target:** Keep Kubernetes (DO/Hetzner) in mind for configuration, logging, and probes.

---

**Phase 1: Basic E2E Pipeline (Coupled)**

* **Goal:** Get data flowing from Alpaca through a simple processor to *real* Redis & Kafka instances. Verify basic connectivity and data format.
* **Tasks:**
    * [x] **Setup Project:** Basic Go project structure, `go.mod`.
    * [x] **Initial Config:** Basic `Config` struct, load Redis/Kafka addresses, topics, Alpaca keys from environment variables (`os.Getenv`).
    * [x] **Integrate Redis Client:** Add `go-redis/v9`. Create simple functions/methods within `main` or a basic `RedisClient` struct to `SET price:SYMBOL` and `PUBLISH` to the trade channel. Connect in `main`.
    * [x] **Integrate Kafka Client:** Add `IBM/sarama`. Create simple functions/methods within `main` or a basic `KafkaClient` struct using `sarama.SyncProducer` to publish trades. Connect in `main`.
    * [x] **Modify Processor:** Update `TradeProcessor` (based on the starting code):
        * `processTrade` now directly calls the Redis SET, Redis PUBLISH, and Kafka PUBLISH functions/methods *synchronously*.
        * Add basic `log.Printf` statements for publish success/failure.
        * Instantiate `TradeProcessor` in `main` and pass client connections if needed.
        * Run the processor `Start` loop in a goroutine.
    * [x] **Connect Alpaca:** Use the existing Alpaca connection logic (`MarketDataClient` struct wrapping the stream client) to feed `stream.Trade` into `tp.recordTrade`. Ensure `recordTrade` converts to `marketdata.Trade` before sending to the channel.
    * [x] **Initial Testing:** Manual execution. Connect to local/dev Redis & Kafka instances. Use a tool (e.g., `redis-cli MONITOR`, `kafkacat`) to verify that trades received from Alpaca (when markets are open, or using paper trading) appear correctly in Redis (cache & pub/sub) and Kafka. This validates connectivity and basic data flow.
    * [x] **Observability:** Keep existing `pprof` endpoint. Add basic `log.Printf` calls for major events (startup, connection, publish attempts, shutdown).
    * [x] **Versioning:** Initialize Git repository. Tag this stage (e.g., `v0.1.0`).

---

**Phase 2: Aggregation & Async Kafka with Retries**

* **Goal:** Implement trade debouncing per symbol and make Kafka publishing non-blocking and resilient.
* **Tasks:**
    * [x] **Implement Aggregator:** Create the `TradeAggregator` struct and `Start` logic. Introduce `rawTradesChan` and `processedTradesChan`. Modify Alpaca handler (`recordTrade`) to send to `rawTradesChan`. Modify `TradeProcessor` to read from `processedTradesChan`. Add `AggregationInterval` to config.
    * [x] **Refactor Kafka:**
        * Create `backgroundKafkaChan`.
        * Modify `TradeProcessor.processTrade` to send trade data *non-blockingly* to `backgroundKafkaChan` instead of calling the Kafka publish method directly.
        * Create a background worker goroutine in `main` that reads from `backgroundKafkaChan`.
        * Implement robust retry logic (e.g., using `cenkalti/backoff`) within the background worker when calling the actual Kafka publish method. Log persistent failures after retries.
    * [x] **Refactor Redis:**
        * Modify `TradeProcessor.processTrade` to call Redis SET and Redis PUBLISH *synchronously* but add `context.WithTimeout` around the calls.
        * Add simple, limited retry logic (e.g., 1-2 retries on specific errors) for Redis calls within `processTrade`. Log persistent failures but *do not block* indefinitely.
    * [x] **Testing:**
        * Manual testing: Verify fewer messages hit Redis/Kafka than raw trades received (aggregation). Temporarily stop Kafka broker to verify retry logic in logs and check that Redis updates continue.
        * Unit Testing: Add unit tests specifically for the `TradeAggregator`'s debouncing logic (provide input trades, check output over time).
    * [ ] **Observability:**
        * **Introduce Prometheus:** Add `prometheus/client_golang`. Expose `/metrics` endpoint in `main`.
        * **Add Key Metrics:** Goroutine count, memory stats (using default collectors). Custom metrics: `raw_trades_received`, `aggregated_trades_sent`, `processor_trades_processed`, `redis_publish_errors`, `kafka_publish_errors`, `kafka_retries`, `aggregator_map_size`, channel buffer usage ratios for *all* channels.
        * **Local Monitoring Stack:** Set up `docker-compose.yml` with Prometheus & Grafana for local development visibility.
    * [ ] **Versioning:** Tag this stage (e.g., `v0.2.0`).

---

**Phase 3: Decoupling & Mocking for Testability**

* **Goal:** Refactor to use interfaces for external dependencies, enabling comprehensive testing without real services.
* **Tasks:**
    * [ ] **Define Interfaces:** Create `pkg/marketdata` (`Trade`, `MarketDataClient`) and `pkg/pubsub` (`RedisPublisherInterface`, `KafkaPublisherInterface`).
    * [ ] **Implement Adapters/Wrappers:**
        * Create `AlpacaAdapter` in `internal/infrastructure/provider/alpacaprovider` implementing `MarketDataClient`.
        * Ensure existing Redis/Kafka client wrappers in `internal/infrastructure/...` implement the `pubsub` interfaces.
    * [ ] **Refactor Core Logic:** Modify `Aggregator`, `Processor`, and `main` to use the generic `marketdata.Trade` type and depend on the *interfaces*, not concrete implementations.
    * [ ] **Create Mocks:** Implement mock versions for `MarketDataClient`, `RedisPublisherInterface`, `KafkaPublisherInterface` (e.g., in `pkg/pubsub/mocks/`, `internal/infrastructure/provider/mockprovider/`).
    * [ ] **Testing:**
        * **Write Key Integration Test:** Create a test (`internal/processor/pipeline_test.go` or similar) that:
            * Instantiates Aggregator & Processor.
            * Injects *mock* publishers.
            * Uses a *mock* market data client (or sends directly to `rawTradesChan`).
            * Sends controlled sequences of trades to test aggregation logic.
            * Configures mocks to return errors to test failure handling in the processor/background worker.
            * Asserts that the correct calls (with correct data) were made to the mock publishers.
        * Refine/add unit tests for Processor using mocks.
    * [ ] **Observability:** Ensure metrics still work correctly after refactoring.
    * [ ] **Versioning:** Tag this stage (e.g., `v0.3.0`).

---

**Phase 4: Production Hardening & Deployment Prep**

* **Goal:** Make the service robust, configurable, observable, and ready for deployment.
* **Tasks:**
    * [ ] **Configuration:** Implement robust configuration loading (e.g., using `spf13/viper`) supporting environment variables and potentially config files. Validate config on startup.
    * [ ] **Structured Logging:** Replace all `log.Printf` with a structured logger (`uber-go/zap` or `rs/zerolog`). Ensure logs are in JSON or another parseable format. Include context (like symbol) in logs where helpful.
    * [ ] **Error Handling Review:** Systematically review error handling. Are errors logged with sufficient context? Are retries appropriate? Does the app exit gracefully on fatal errors?
    * [ ] **Observability Enhancements:**
        * Refine Prometheus metrics/alerts based on testing insights. Create a basic Grafana dashboard (importing a standard Go dashboard + adding custom panels).
        * Consider adding basic Distributed Tracing (OpenTelemetry) if cross-service calls were involved or detailed latency breakdown is needed (may be optional initially).
    * [ ] **Dockerfile:** Create a production-ready, multi-stage Dockerfile. Ensure non-root user execution.
    * [ ] **Kubernetes Manifests:** Create basic Helm chart or Kustomize overlays for Deployment, Service, ConfigMap (for non-sensitive config), Secrets (for API keys).
    * [ ] **Health Checks:** Implement Kubernetes liveness and readiness probes (e.g., an HTTP endpoint `/healthz` that checks connections to dependencies or internal state).
    * [ ] **Testing:** Perform basic load testing (using mocks or a dedicated test environment) to understand resource usage (CPU/memory) under pressure and tune defaults. Test failure modes (e.g., disconnect Redis/Kafka during run).
    * [ ] **Versioning:** Tag release candidate (e.g., `v1.0.0-rc.1`).

---

**Phase 5: Deployment & Iteration**

* **Goal:** Deploy to production and establish ongoing monitoring and maintenance.
* **Tasks:**
    * [ ] **CI/CD:** Set up a basic CI/CD pipeline (e.g., GitHub Actions): Build -> Test (unit/integration) -> Build Docker Image -> Push to Registry -> Deploy to K8s (staging first, then production).
    * [ ] **Deployment:** Deploy to staging/production Kubernetes cluster (DO/Hetzner). Configure secrets and config maps correctly.
    * [ ] **Monitoring:** Monitor metrics and logs closely using Prometheus/Grafana and log aggregation in the production environment.
    * [ ] **Tuning:** Adjust resource requests/limits, channel buffer sizes, aggregation interval based on real-world performance and monitoring data.
    * [ ] **Iteration:** Address any issues found in production. Plan future enhancements based on requirements or performance observations.
    * [ ] **Versioning:** Tag official release (e.g., `v1.0.0`). Use semantic versioning for subsequent updates.

---

**Sequencing Notes:**

* **Development & Testing:** Closely interleaved. Write tests *as* components are developed or refactored (especially after decoupling in Phase 3). Use mocks heavily before markets open or for specific scenarios.
* **Observability:** Introduce basic logging/pprof early (Phase 1). Add core metrics (Phase 2) once key components exist. Enhance logging/metrics/tracing during production hardening (Phase 4). Observability *informs* tuning and development priorities.
* **Deployment:** Basic Dockerfile appears mid-way (Phase 2/3). K8s manifests and CI/CD come later during production hardening (Phase 4/5).
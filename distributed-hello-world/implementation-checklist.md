# Distributed Hello World Implementation Stages

## Stage 1: HTTP to Kafka Flow

- [x] Set up Go service with basic HTTP endpoint
- [x] Add Kafka producer to Go service
- [x] Send message to Kafka when HTTP endpoint is hit
- [x] Verify message appears in Kafka topic (using akhq)

Testing this stage:

1. HTTP POST to endpoint
2. Use akhq to see message

## Stage 2: Kafka to Redis Flow

- [x] Add Kafka consumer to Go service
- [ ] Add Redis client and implement publish
- [ ] Connect consumer to publisher
- [ ] Verify flow using redisinsight to subscribe

Testing this stage:

1. Use OffsetOldest to read all messages and publish to redis
2. Use redisinsight to verify message appears in channel

## Stage 3: Complete Flow with Elixir

- [ ] Create basic Elixir service
- [ ] Add Redis subscription to Elixir
- [ ] Verify end-to-end flow
- [ ] Add basic logging in both services

Testing this stage:

1. HTTP POST to Go service
2. Verify message appears in Elixir logs

## Basic Operational Needs

These should be added as needed during development:

- [ ] Health checks where required by docker-compose
- [ ] Error logging (not swallowing errors)
- [ ] Basic signal handling for clean shutdown

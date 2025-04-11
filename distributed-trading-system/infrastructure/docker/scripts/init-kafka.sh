#!/bin/bash

kafka-topics --bootstrap-server kafka-1:29092 --create --if-not-exists \
  --topic market_data --partitions 3 --replication-factor 3

kafka-topics --bootstrap-server kafka-1:29092 --create --if-not-exists \
  --topic trade_orders --partitions 3 --replication-factor 3

kafka-topics --bootstrap-server kafka-1:29092 --create --if-not-exists \
  --topic trade_executed --partitions 3 --replication-factor 3

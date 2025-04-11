#!/bin/sh

# Create Users table if it doesn't exist
if ! aws dynamodb describe-table --endpoint-url http://dynamodb:8000 --table-name Users > /dev/null 2>&1; then
  aws dynamodb create-table \
    --endpoint-url http://dynamodb:8000 \
    --table-name Users \
    --attribute-definitions AttributeName=userId,AttributeType=S \
    --key-schema AttributeName=userId,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5
fi

# Add sample data
aws dynamodb put-item \
  --endpoint-url http://dynamodb:8000 \
  --table-name Users \
  --item '{"userId": {"S": "user1"}, "name": {"S": "John Doe"}}'

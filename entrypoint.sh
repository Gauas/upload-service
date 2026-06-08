#!/bin/sh

SERVICE_TYPE=${1:-http}

echo "Starting service: $SERVICE_TYPE"

if [ "$SERVICE_TYPE" = "consumer" ]; then
    echo "Starting consumer service..."
    if [ -f "./consumer-service" ]; then
        exec ./consumer-service
    fi

    echo "Consumer binary not found. Running with go run..."
    exec go run ./consumer
fi

echo "Starting HTTP API service..."
if [ -f "./http-service" ]; then
    exec ./http-service
fi

echo "HTTP binary not found. Running with go run..."
exec go run ./cmd/main.go

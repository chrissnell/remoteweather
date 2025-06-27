#!/bin/bash

# API Test Runner for RemoteWeather Management API
# This script runs comprehensive CRUD API tests with automatic service startup

set -e

# Configuration
CAMPBELL_PORT=8123
MANAGEMENT_PORT=8081
TIMESCALE_PORT=5432
TEST_TOKEN="test-token-123"
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TEST_DIR="$(dirname "$0")"

# Cleanup function
cleanup() {
    echo
    echo "» Cleaning up services..."
    
    # Kill remoteweather first to avoid reconnection attempts
    if [ ! -z "$REMOTEWEATHER_PID" ]; then
        kill $REMOTEWEATHER_PID 2>/dev/null || true
        sleep 1  # Give it time to shutdown gracefully
        echo "  ▪ RemoteWeather stopped"
    fi
    
    # Kill Campbell emulator
    if [ ! -z "$CAMPBELL_PID" ]; then
        kill $CAMPBELL_PID 2>/dev/null || true
        echo "  ▪ Campbell emulator stopped"
    fi
    
    # Stop TimescaleDB container if we started it
    if [ "$STARTED_TIMESCALE" = "true" ]; then
        docker stop remoteweather-timescale 2>/dev/null || true
        docker rm remoteweather-timescale 2>/dev/null || true
        echo "  ▪ TimescaleDB container stopped"
    fi
    
    echo "  ■ Cleanup complete"
}

# Set trap for cleanup
trap cleanup EXIT

echo "=== RemoteWeather Management API Test Suite ==="
echo "Starting tests at $(date)"
echo "Project root: $PROJECT_ROOT"
echo

# Check if ports are available
check_port() {
    local port=$1
    local service=$2
    if lsof -i :$port >/dev/null 2>&1; then
        echo "▲ Port $port is already in use (needed for $service)"
        echo "   Please stop the service using port $port or use a different port"
        return 1
    fi
    return 0
}

echo "» Checking port availability..."
check_port $CAMPBELL_PORT "Campbell emulator" || exit 1
check_port $MANAGEMENT_PORT "Management API" || exit 1
echo "▪ Ports available"
echo

# Start TimescaleDB container if Docker is available and not already running
echo "» Checking for TimescaleDB..."
if command -v docker >/dev/null 2>&1; then
    if ! docker ps | grep -q remoteweather-timescale; then
        echo "  Starting TimescaleDB container..."
        docker run -d \
            --name remoteweather-timescale \
            -p $TIMESCALE_PORT:5432 \
            -e POSTGRES_PASSWORD=testpass \
            -e POSTGRES_USER=testuser \
            -e POSTGRES_DB=remoteweather \
            timescale/timescaledb:latest-pg15 >/dev/null 2>&1
        
        if [ $? -eq 0 ]; then
            STARTED_TIMESCALE=true
            echo "  ▪ TimescaleDB started on port $TIMESCALE_PORT"
            echo "     Connection: postgres://testuser:testpass@localhost:$TIMESCALE_PORT/remoteweather"
            echo "     Waiting for database to be ready..."
            sleep 5
        else
            echo "  ▲ Failed to start TimescaleDB container (tests will continue without it)"
        fi
    else
        echo "  ▪ TimescaleDB container already running"
    fi
else
    echo "  ▲ Docker not available, skipping TimescaleDB setup"
fi
echo

# Build Campbell emulator
echo "» Building Campbell Scientific emulator..."
cd "$PROJECT_ROOT/cmd/campbell-emulator"
go build -o campbell-emulator main.go
if [ $? -ne 0 ]; then
    echo "× Failed to build Campbell emulator"
    exit 1
fi
echo "▪ Campbell emulator built"

# Start Campbell emulator
echo "» Starting Campbell Scientific emulator on port $CAMPBELL_PORT..."
./campbell-emulator -port $CAMPBELL_PORT &
CAMPBELL_PID=$!
sleep 2

# Check if emulator started successfully
if ! kill -0 $CAMPBELL_PID 2>/dev/null; then
    echo "× Failed to start Campbell emulator"
    exit 1
fi
echo "▪ Campbell emulator running (PID: $CAMPBELL_PID)"

# Build remoteweather
echo "» Building remoteweather..."
cd "$PROJECT_ROOT"
go build -o remoteweather cmd/remoteweather/main.go
if [ $? -ne 0 ]; then
    echo "× Failed to build remoteweather"
    exit 1
fi
echo "▪ RemoteWeather built"

# Start remoteweather with SQLite backend
echo "» Starting remoteweather with management API..."
./remoteweather -config-backend sqlite -config test-config.db &
REMOTEWEATHER_PID=$!
sleep 3

# Check if remoteweather started successfully
if ! kill -0 $REMOTEWEATHER_PID 2>/dev/null; then
    echo "× Failed to start remoteweather"
    exit 1
fi

# Wait for management API to be ready
echo "» Waiting for management API to be ready..."
for i in {1..10}; do
    if curl -s -f -H "Authorization: Bearer $TEST_TOKEN" http://127.0.0.1:$MANAGEMENT_PORT/api/status >/dev/null 2>&1; then
        echo "▪ Management API is ready"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "× Management API failed to start within 30 seconds"
        exit 1
    fi
    sleep 3
done

# Show running services
echo
echo "» Services Status:"
echo "  ▪ Campbell Emulator: http://localhost:$CAMPBELL_PORT (PID: $CAMPBELL_PID)"
echo "  ▪ Management API: http://127.0.0.1:$MANAGEMENT_PORT (PID: $REMOTEWEATHER_PID)"
if [ "$STARTED_TIMESCALE" = "true" ]; then
    echo "  ▪ TimescaleDB: postgres://testuser:testpass@localhost:$TIMESCALE_PORT/remoteweather"
fi
echo

# Run the Go tests
echo "» Running comprehensive API tests..."
cd "$TEST_DIR"

# Run tests with verbose output
go test -v -timeout 120s ./...
TEST_EXIT_CODE=$?

echo
echo "=== Test Summary ==="
if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo "▪ All tests passed successfully!"
    echo "   ▪ Services performed as expected"
    echo "   ▪ Authentication and security working"
    echo "   ▪ CRUD operations validated"
else
    echo "× Some tests failed (exit code: $TEST_EXIT_CODE)"
fi

echo "Tests completed at $(date)"
exit $TEST_EXIT_CODE 
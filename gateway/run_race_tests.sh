#!/bin/bash
# Script to run WebSocket race condition stress tests
# Uses Go's built-in race detector to catch concurrency issues

set -e

echo "🔍 Running WebSocket Race Condition Stress Tests"
echo "=================================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test 1: Concurrent Close Race
echo -e "${YELLOW}Test 1: Concurrent Close() Race${NC}"
echo "This test triggers both readPump and writePump closing simultaneously"
echo ""
if go test -race -run TestWebSocket_ConcurrentCloseRace -v -count=5 .; then
    echo -e "${GREEN}✓ PASSED${NC}"
else
    echo -e "${RED}✗ FAILED - Race detected!${NC}"
fi
echo ""

# Test 2: High Throughput Stress
echo -e "${YELLOW}Test 2: High Throughput Stress Test${NC}"
echo "Sends 1000 rapid messages to stress the write pump"
echo ""
if go test -race -run TestWebSocket_HighThroughputStressTest -v -timeout=30s .; then
    echo -e "${GREEN}✓ PASSED${NC}"
else
    echo -e "${RED}✗ FAILED - Race detected!${NC}"
fi
echo ""

# Test 3: Disconnect During Broadcast
echo -e "${YELLOW}Test 3: Disconnect During Broadcast${NC}"
echo "Simulates client disconnect while broadcaster is sending events"
echo "This should expose the send-on-closed-channel race"
echo ""
if go test -race -run TestWebSocket_DisconnectDuringBroadcast -v -count=3 .; then
    echo -e "${GREEN}✓ PASSED${NC}"
else
    echo -e "${RED}✗ FAILED - Race detected!${NC}"
fi
echo ""

echo "=================================================="
echo "🏁 Race tests complete!"
echo ""
echo "To run with even more aggressive race detection:"
echo "  GORACE='halt_on_error=1 history_size=7' go test -race -run TestWebSocket -v ."
echo ""
echo "To run indefinitely until failure:"
echo "  while go test -race -run TestWebSocket_DisconnectDuringBroadcast -count=1; do :; done"
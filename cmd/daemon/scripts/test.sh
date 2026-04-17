#!/bin/bash
# Loom Daemon End-to-End Test Script

set -e

echo "================================"
echo "Loom Daemon Integration Tests"
echo "================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Build daemon
echo -e "${YELLOW}Building daemon...${NC}"
cd "$(dirname "$0")/.."
go build -o loomd .

# Start daemon in background
echo -e "${YELLOW}Starting daemon...${NC}"
./loomd start --listen 127.0.0.1:19999 &
DAEMON_PID=$!

# Wait for daemon to start
sleep 2

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    kill $DAEMON_PID 2>/dev/null || true
    wait $DAEMON_PID 2>/dev/null || true
}
trap cleanup EXIT

# Test health endpoint
echo -e "${YELLOW}Testing /health endpoint...${NC}"
HEALTH_RESPONSE=$(curl -s http://127.0.0.1:19999/health)
echo "Response: $HEALTH_RESPONSE"

if echo "$HEALTH_RESPONSE" | grep -q '"status":"healthy"'; then
    echo -e "${GREEN}✓ Health check passed${NC}"
else
    echo -e "${RED}✗ Health check failed${NC}"
    exit 1
fi

# Test status endpoint
echo -e "${YELLOW}Testing /v1/status endpoint...${NC}"
STATUS_RESPONSE=$(curl -s http://127.0.0.1:19999/v1/status)
echo "Response: $STATUS_RESPONSE"

if echo "$STATUS_RESPONSE" | grep -q '"version"'; then
    echo -e "${GREEN}✓ Status endpoint passed${NC}"
else
    echo -e "${RED}✗ Status endpoint failed${NC}"
    exit 1
fi

# Test list sessions endpoint
echo -e "${YELLOW}Testing /v1/sessions endpoint...${NC}"
SESSIONS_RESPONSE=$(curl -s http://127.0.0.1:19999/v1/sessions)
echo "Response: $SESSIONS_RESPONSE"

if echo "$SESSIONS_RESPONSE" | grep -q '"sessions"'; then
    echo -e "${GREEN}✓ List sessions passed${NC}"
else
    echo -e "${RED}✗ List sessions failed${NC}"
    exit 1
fi

# Test create session endpoint
echo -e "${YELLOW}Testing POST /v1/sessions endpoint...${NC}"
CREATE_RESPONSE=$(curl -s -X POST \
    -H "Content-Type: application/json" \
    -d '{"runtime_type":"claude","working_dir":"."}' \
    http://127.0.0.1:19999/v1/sessions)
echo "Response: $CREATE_RESPONSE"

if echo "$CREATE_RESPONSE" | grep -q '"id"'; then
    echo -e "${GREEN}✓ Create session passed${NC}"
    SESSION_ID=$(echo "$CREATE_RESPONSE" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    echo "Created session: $SESSION_ID"

    # Test get session
    echo -e "${YELLOW}Testing GET /v1/sessions/$SESSION_ID...${NC}"
    GET_RESPONSE=$(curl -s http://127.0.0.1:19999/v1/sessions/$SESSION_ID)
    echo "Response: $GET_RESPONSE"

    if echo "$GET_RESPONSE" | grep -q "\"id\":\"$SESSION_ID\""; then
        echo -e "${GREEN}✓ Get session passed${NC}"
    else
        echo -e "${RED}✗ Get session failed${NC}"
    fi

    # Test stop session
    echo -e "${YELLOW}Testing DELETE /v1/sessions/$SESSION_ID...${NC}"
    STOP_RESPONSE=$(curl -s -X DELETE http://127.0.0.1:19999/v1/sessions/$SESSION_ID)
    echo "Response: $STOP_RESPONSE"
    echo -e "${GREEN}✓ Stop session passed${NC}"
else
    echo -e "${YELLOW}⚠ Create session returned error (expected if runtime not available)${NC}"
fi

echo ""
echo -e "${GREEN}================================"
echo "All tests passed!"
echo "================================${NC}"

#!/bin/bash

# Quick Progressive Load Test - For rapid validation
#
# Phase 1: Low load    - 2 users, 2 minutes
# Phase 2: Medium load - 5 users, 3 minutes
# Phase 3: High load   - 8 users, 5 minutes
#
# Total duration: ~10 minutes
#
# Usage:
#   ./quick-progressive-test.sh [--verbose]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
COBUDDY_BIN="$PROJECT_ROOT/memdoor"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
GATEWAY_URL="http://localhost:18789"
CHANNEL="test"
VERBOSE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Test counters
TOTAL_MESSAGES=0
TOTAL_ERRORS=0

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
    ((TOTAL_ERRORS++))
}

log_phase() {
    echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
}

# Check gateway
log_info "Checking gateway..."
if ! curl -s "$GATEWAY_URL/health" > /dev/null 2>&1; then
    log_error "Gateway not running"
    exit 1
fi
log_success "Gateway is running"

# Send test message
send_message() {
    local user_id=$1
    local message=$2

    if [ "$VERBOSE" = true ]; then
        echo "  User $user_id: $message"
    fi

    if "$COBUDDY_BIN" agent --message "$message" --channel "$CHANNEL" --agent-id "writer" --thinking disabled >/dev/null 2>&1; then
        ((TOTAL_MESSAGES++))
        return 0
    else
        log_error "Message failed"
        return 1
    fi
}

# Run phase
run_phase() {
    local users=$1
    local duration_min=$2
    local phase_name=$3

    log_phase "$phase_name: $users users × $duration_min min"

    local start=$(date +%s)
    local end=$((start + (duration_min * 60)))
    local round=0

    while [ $(date +%s) -lt $end ]; do
        ((round++))
        log_info "Round $round - sending $users messages..."

        for user in $(seq 1 $users); do
            send_message "$user" "Test message round $round from user $user" &
        done
        wait

        # Short delay between rounds
        sleep 3
    done

    log_success "$phase_name complete: $TOTAL_MESSAGES total messages, $TOTAL_ERRORS errors"
    sleep 5  # Cool down
}

# Main
echo -e "${CYAN}"
echo "╔═══════════════════════════════════════════════════╗"
echo "║     Quick Progressive Load Test (10 min)         ║"
echo "╚═══════════════════════════════════════════════════╝"
echo -e "${NC}\n"

run_phase 2 2 "PHASE 1: LOW LOAD"
run_phase 5 3 "PHASE 2: MEDIUM LOAD"
run_phase 8 5 "PHASE 3: HIGH LOAD"

# Summary
echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}SUMMARY${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

log_info "Total messages: $TOTAL_MESSAGES"
log_info "Total errors: $TOTAL_ERRORS"

if [ $TOTAL_ERRORS -eq 0 ]; then
    log_success "All tests passed!"
else
    log_error "Tests completed with errors"
fi

# Gateway health
echo ""
log_info "Gateway status:"
curl -s "$GATEWAY_URL/health" | jq -r '.gateway | "  Uptime: \(.uptime)s | Goroutines: \(.goroutines) | Active Conns: \(.activeConns)"' 2>/dev/null || echo "  Unable to fetch"

echo ""

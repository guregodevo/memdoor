#!/bin/bash

# Progressive Load Test - Start Low to High
# Gradually increases load to find system limits
#
# Phase 1: Low load    - 2 concurrent users, 5 minutes
# Phase 2: Medium load - 5 concurrent users, 10 minutes
# Phase 3: High load   - 10 concurrent users, 15 minutes
#
# Usage:
#   ./progressive-load-test.sh [--skip-low] [--skip-medium] [--verbose]

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
SKIP_LOW=false
SKIP_MEDIUM=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-low)
            SKIP_LOW=true
            shift
            ;;
        --skip-medium)
            SKIP_MEDIUM=true
            shift
            ;;
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --help|-h)
            echo "Progressive Load Test - Low to High"
            echo ""
            echo "Usage: $0 [--skip-low] [--skip-medium] [--verbose]"
            echo ""
            echo "Phases:"
            echo "  Phase 1: Low    - 2 users, 5 min"
            echo "  Phase 2: Medium - 5 users, 10 min"
            echo "  Phase 3: High   - 10 users, 15 min"
            echo ""
            echo "Options:"
            echo "  --skip-low       Skip low load phase"
            echo "  --skip-medium    Skip medium load phase"
            echo "  --verbose        Show detailed output"
            exit 0
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
PHASE_RESULTS=()

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
    echo -e "\n${CYAN}===================================================${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${CYAN}===================================================${NC}\n"
}

log_verbose() {
    if [ "$VERBOSE" = true ]; then
        echo -e "${YELLOW}[VERBOSE]${NC} $1"
    fi
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check if memdoor binary exists
    if [ ! -f "$COBUDDY_BIN" ]; then
        log_error "memdoor binary not found at $COBUDDY_BIN"
        log_info "Run: make build"
        exit 1
    fi
    log_success "CoBuddy binary found"

    # Check if gateway is running
    if ! curl -s "$GATEWAY_URL/health" > /dev/null 2>&1; then
        log_error "Gateway not running at $GATEWAY_URL"
        log_info "Start gateway: ./memdoor gateway --verbose"
        exit 1
    fi
    log_success "Gateway is running"

    # Check if test channel exists
    "$COBUDDY_BIN" channels list 2>/dev/null | grep -q "test" || {
        log_info "Creating test channel..."
        "$COBUDDY_BIN" channels create --name test --description "Load testing channel" 2>/dev/null || true
    }
    log_success "Test channel exists"

    # Check if test agents exist
    "$COBUDDY_BIN" agents list 2>/dev/null | grep -q "writer" || {
        log_error "Test agent 'writer' not found"
        log_info "Run: ./memdoor setup"
        exit 1
    }
    log_success "Test agents available"
}

# Send a test message
send_message() {
    local user_id=$1
    local message=$2
    local agent=${3:-writer}

    log_verbose "User $user_id: $message"

    if "$COBUDDY_BIN" agent --message "$message" --channel "$CHANNEL" --agent-id "$agent" --thinking disabled 2>&1 | grep -q "error"; then
        log_error "Failed to send message: $message"
        return 1
    else
        ((TOTAL_MESSAGES++))
        return 0
    fi
}

# Simulate concurrent users
simulate_users() {
    local num_users=$1
    local duration_minutes=$2
    local phase_name=$3

    log_phase "Phase: $phase_name"
    log_info "Users: $num_users | Duration: $duration_minutes min | Channel: $CHANNEL"

    local start_time=$(date +%s)
    local end_time=$((start_time + (duration_minutes * 60)))
    local phase_messages=0
    local phase_errors=$TOTAL_ERRORS

    log_info "Starting at $(date '+%H:%M:%S'), will run until $(date -r $end_time '+%H:%M:%S')"

    # Message patterns for variety
    local messages=(
        "What's the current status?"
        "Can you help me with this task?"
        "Please review the latest changes"
        "What are the next steps?"
        "Can you summarize the discussion?"
    )

    while [ $(date +%s) -lt $end_time ]; do
        # Simulate concurrent users sending messages
        for user in $(seq 1 $num_users); do
            local msg_index=$((RANDOM % ${#messages[@]}))
            local message="${messages[$msg_index]}"

            # Send message in background to simulate concurrency
            send_message "$user" "$message" &

            # Small delay between users (0.5-2 seconds)
            sleep $(awk "BEGIN {print $RANDOM / 32768 * 1.5 + 0.5}")
        done

        # Wait for all background jobs to complete
        wait

        # Progress update every minute
        local elapsed=$(($(date +%s) - start_time))
        if [ $((elapsed % 60)) -eq 0 ]; then
            local remaining=$((duration_minutes - (elapsed / 60)))
            log_info "Progress: $elapsed/${duration_minutes}min elapsed, $remaining min remaining, $TOTAL_MESSAGES messages sent"
        fi

        # Delay between rounds (5-15 seconds)
        sleep $(awk "BEGIN {print $RANDOM / 32768 * 10 + 5}")
    done

    # Calculate phase stats
    phase_messages=$((TOTAL_MESSAGES - phase_messages))
    local phase_errors_count=$((TOTAL_ERRORS - phase_errors))
    local avg_throughput=$(awk "BEGIN {print $TOTAL_MESSAGES / ($duration_minutes * 60)}")

    # Store results
    PHASE_RESULTS+=("$phase_name: $TOTAL_MESSAGES messages, $phase_errors_count errors, ${avg_throughput} msg/sec")

    log_success "$phase_name completed: $TOTAL_MESSAGES total messages, $phase_errors_count errors"

    # Cool down between phases (30 seconds)
    if [ $(date +%s) -lt $end_time ]; then
        log_info "Cooling down for 30 seconds..."
        sleep 30
    fi
}

# Check for errors in logs
check_logs() {
    log_info "Checking logs for errors..."

    local recent_errors=$("$COBUDDY_BIN" logs errors --limit 20 2>/dev/null | grep -c "ERROR" || echo "0")

    if [ "$recent_errors" -gt 0 ]; then
        log_error "Found $recent_errors recent errors in logs"
        if [ "$VERBOSE" = true ]; then
            "$COBUDDY_BIN" logs errors --limit 5
        fi
    else
        log_success "No recent errors in logs"
    fi
}

# Print summary
print_summary() {
    echo -e "\n${CYAN}===================================================${NC}"
    echo -e "${CYAN}LOAD TEST SUMMARY${NC}"
    echo -e "${CYAN}===================================================${NC}\n"

    log_info "Total messages sent: $TOTAL_MESSAGES"
    log_info "Total errors: $TOTAL_ERRORS"

    echo ""
    log_info "Phase Results:"
    for result in "${PHASE_RESULTS[@]}"; do
        echo "  - $result"
    done

    echo ""
    if [ $TOTAL_ERRORS -eq 0 ]; then
        log_success "All tests passed! No errors detected."
    else
        log_error "Tests completed with $TOTAL_ERRORS errors"
    fi

    # Show system health
    echo ""
    log_info "Gateway health check:"
    curl -s "$GATEWAY_URL/health" | jq -r '.gateway | "  Uptime: \(.uptime)s | Memory: \(.memoryUsage) bytes | Goroutines: \(.goroutines)"' 2>/dev/null || echo "  Unable to fetch health data"
}

# Main execution
main() {
    echo -e "${CYAN}"
    echo "╔═══════════════════════════════════════════════════╗"
    echo "║     Progressive Load Test: Low → High            ║"
    echo "╚═══════════════════════════════════════════════════╝"
    echo -e "${NC}"

    check_prerequisites

    # Phase 1: Low load (2 users, 5 min)
    if [ "$SKIP_LOW" = false ]; then
        simulate_users 2 5 "LOW LOAD"
    else
        log_info "Skipping low load phase"
    fi

    # Phase 2: Medium load (5 users, 10 min)
    if [ "$SKIP_MEDIUM" = false ]; then
        simulate_users 5 10 "MEDIUM LOAD"
    else
        log_info "Skipping medium load phase"
    fi

    # Phase 3: High load (10 users, 15 min)
    simulate_users 10 15 "HIGH LOAD"

    # Final checks
    check_logs

    # Print summary
    print_summary
}

# Trap Ctrl+C for graceful shutdown
trap 'echo -e "\n${YELLOW}Test interrupted by user${NC}"; print_summary; exit 130' INT

# Run the test
main

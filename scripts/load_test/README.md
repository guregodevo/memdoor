# Load Testing Scripts

Progressive load testing scripts for Memdoor gateway.

## Quick Test (10 minutes)

Fast validation test with 3 phases:

```bash
./scripts/load_test/quick-progressive-test.sh
```

**Phases:**
- Phase 1: 2 users × 2 minutes (low load)
- Phase 2: 5 users × 3 minutes (medium load)
- Phase 3: 8 users × 5 minutes (high load)

**Total duration:** ~10 minutes

## Full Test (30 minutes)

Comprehensive load test with 3 phases:

```bash
./scripts/load_test/progressive-load-test.sh
```

**Phases:**
- Phase 1: 2 users × 5 minutes (low load)
- Phase 2: 5 users × 10 minutes (medium load)
- Phase 3: 10 users × 15 minutes (high load)

**Total duration:** ~30 minutes

## Options

Both scripts support:

- `--verbose` - Show detailed output during test
- `--skip-low` - Skip low load phase (full test only)
- `--skip-medium` - Skip medium load phase (full test only)

## Prerequisites

Before running:

1. Build the project: `make build`
2. Start the gateway: `make start` or `./memdoor gateway`
3. Ensure test channel and agents exist: `./memdoor setup`

## What It Tests

- Concurrent agent executions in shared channels
- Session state integrity under load
- Message ordering and threading
- API error handling (orphaned tool_use blocks)
- Gateway stability and resource usage

## Interpreting Results

**Success:**
- Total errors: 0
- No "orphaned tool_use" errors in logs
- Gateway remains responsive

**Failure:**
- Any API errors reported
- Check logs: `./memdoor logs query --regex "error|Error|orphaned" --limit 20`

## History

These scripts were created to validate the fix for session poisoning bugs that caused orphaned tool_use blocks under concurrent load (fixed in commit a842e29).

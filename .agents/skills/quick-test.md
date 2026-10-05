---
name: quick-test
description: Quick testing workflow before committing changes
user-invocable: true
disable-model-invocation: false
metadata:
  openclaw:
    emoji: ⚡
    skillKey: quick-test
    always: true
    os:
      - darwin
      - linux
---

# Quick Test Skill

Fast testing workflow to run before every commit. Use this skill automatically before finalizing changes.

## Quick Test Checklist

```bash
# 1. Build (kills processes, cleans, builds)
make build

# 2. Start gateway (if not running)
make clean stop start

# 3. Test critical path: Send message
./memdoor agent --message "test" --channel test2 --agent-id writer

# 4. View messages
./memdoor messages --channel test2 --limit 5

# 5. Check logs for errors
./memdoor logs query --limit 20

# 6. Optional: Run unit tests if touching core code
make test
```

## When to Use

**ALWAYS run before:**
- Committing changes
- Creating PR
- Pushing to remote

**ESPECIALLY after:**
- Refactoring
- Adding new features
- Fixing bugs
- Changing core logic

## Quick Commands

### Minimum Test (30 seconds)

```bash
make build && \
make clean stop start && \
./memdoor agent --message "ping" --channel test2 --agent-id writer && \
./memdoor logs errors --since 1m
```

### Standard Test (1 minute)

```bash
# Build and start
make build
make clean stop start

# Test message send
./memdoor agent --message "test" --channel test2 --agent-id writer

# Verify
./memdoor messages --channel test2 --limit 5
./memdoor logs query --limit 20
```

### Full Test (2-3 minutes)

```bash
# Build and start
make build
make clean stop start

# Unit tests
make test

# Integration test
./memdoor agent --message "test" --channel test2 --agent-id writer
./memdoor messages --channel test2 --limit 5 --include-threads

# Check logs
./memdoor logs query --limit 20
./memdoor logs errors --since 5m
```

## What Each Step Tests

### 1. `make build`
- ✅ Code compiles without errors
- ✅ No syntax errors
- ✅ All imports valid
- ✅ Kills old processes (clean slate)

### 2. Send message
- ✅ Gateway running
- ✅ Agent execution works
- ✅ Database writes succeed
- ✅ Authentication works

### 3. View messages
- ✅ Database reads work
- ✅ Message retrieval works
- ✅ Threading works (if --include-threads)

### 4. Check logs
- ✅ No errors logged
- ✅ Operations completed
- ✅ No warnings

### 5. Unit tests (optional)
- ✅ Core logic unchanged
- ✅ No regressions
- ✅ New code tested

## Common Issues & Fixes

### Build Fails

```bash
# Check for syntax errors
go vet ./...

# Check imports
go mod tidy

# Format code
make fmt
```

### Message Send Fails

```bash
# Check gateway running
curl http://localhost:18789/health

# Check authentication
./memdoor auth whoami

# View recent errors
./memdoor logs errors --since 5m
```

### Database Issues

```bash
# Check database exists
ls -la ~/.memdoor/data/

# Re-initialize if needed (WARNING: deletes data)
rm -rf ~/.memdoor/data/
./memdoor setup
```

## Integration with Git

### Pre-Commit Hook

Create `.git/hooks/pre-commit`:

```bash
#!/bin/bash

echo "🧪 Running quick tests..."

# Build
if ! make build; then
    echo "❌ Build failed"
    exit 1
fi

# Quick smoke test
if ! ./memdoor agent --message "pre-commit test" --channel test2 --agent-id writer; then
    echo "❌ Message send failed"
    exit 1
fi

# Check for errors
if ./memdoor logs errors --since 1m | grep -q "ERROR"; then
    echo "❌ Errors found in logs"
    exit 1
fi

echo "✅ Tests passed"
exit 0
```

Make executable:
```bash
chmod +x .git/hooks/pre-commit
```

## Test Results Interpretation

### ✅ Success Indicators

```bash
# Build output
✅ Memdoor built: ./memdoor

# Message send output
✓ Message posted to #test2 mentioning @writer

# Logs output (no errors)
time=... level=INFO msg="Message processed"
time=... level=DEBUG msg="Agent execution complete"
```

### ❌ Failure Indicators

```bash
# Build errors
# compilation error
undefined: SomeFunction

# Message send errors
Error: authentication required
Error: gateway unreachable

# Log errors
level=ERROR msg="Database write failed"
level=ERROR msg="Tool execution failed"
```

## Speed Optimizations

### Skip Unit Tests for Minor Changes

For cosmetic changes (docs, comments):
```bash
make build && \
./memdoor agent --message "quick test" --channel test2 --agent-id writer
```

### Parallel Testing

Test multiple things simultaneously:
```bash
# Terminal 1
make build && make test

# Terminal 2
./memdoor agent --message "test" --channel test2 --agent-id writer
```

### Cache Builds

```bash
# Build once
make build

# Test multiple times without rebuilding
./memdoor agent --message "test1" --channel test2 --agent-id writer
./memdoor agent --message "test2" --channel test2 --agent-id writer
```

## Related Skills

- `.agents/skills/refactoring.md` - Testing after refactoring

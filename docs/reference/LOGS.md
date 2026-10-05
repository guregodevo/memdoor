# Memdoor Logs System

**Version**: 1.0
**Date**: 2026-04-18

**Agent-First Logging for Self-Debugging AI Systems**

> **The CLI is six subcommands**: `logs errors`, `logs query`, `logs session`,
> `logs stats`, `logs trace`, `logs prune`. Sections below that name
> `logs tail`, `logs write`, `logs rotate` or `logs clean`, or the flags
> `--level`, `--component`, `--errors-only`, `--follow`, `--keep-archives`
> and `--dry-run`, describe a version that no longer exists — the flags
> `logs query` really takes are listed under "logs query" below.

Memdoor implements an advanced logging system designed specifically for AI agents to debug themselves. Unlike traditional logging systems that output text to files, Memdoor's event-based logging enables agents to query causality chains, reconstruct sessions, and detect patterns through structured queries.

## Quick Reference for AI Agents

**Which Command Should I Use?**

### Need to debug an issue?

**Recent errors?**
```bash
./memdoor logs errors --since 1h
```

**Specific pattern?**
```bash
./memdoor logs query --regex "pattern"
```

**Session timeline?**
```bash
./memdoor logs session <session-id>
```

**Root cause?**
```bash
./memdoor logs trace <event-id>
```

### Need real-time monitoring?

**All events**
```bash
./memdoor logs query --since 5m
```

**Only errors**
```bash
./memdoor logs errors --since 1h
```

**Specific component**
```bash
./memdoor logs query --regex 'websocket' --limit 50
```

### Need to ingest external logs?

There is no `logs write` any more: the store holds the gateway's own event
stream (`gateway/logs`), written by the components that emit it.

### Need to manage disk space?

**Delete old events and reclaim the space**
```bash
./memdoor logs prune --days 7      # or --before 720h, or --all
```

---

## Common Debugging Patterns

```bash
# Pattern 1: Find what caused a specific error
./memdoor logs trace <event-id>

# Pattern 2: Search for error patterns across time
./memdoor logs query --regex "timeout|connection.*refused" --since 7d

# Pattern 3: Debug a specific session
./memdoor logs session <session-id>

# Pattern 4: Monitor errors in real-time
./memdoor logs query --level ERROR --follow

# Pattern 5: Analyze component behavior
./memdoor logs query --component WebSocket --since 1h --verbose
```

## Table of Contents

- [Quick Reference for AI Agents](#quick-reference-for-ai-agents)
- [Overview](#overview)
- [Quick Start](#quick-start)
- [CLI Commands](#cli-commands)
- [Event System](#event-system)
- [Causality Tracking](#causality-tracking)
- [Query Interface](#query-interface)
- [Programming Interface](#programming-interface)
- [Best Practices](#best-practices)
- [Troubleshooting Guide](#troubleshooting-guide)

## Overview

### Why Agent-First Logging?

Traditional logging (slog, log4j, etc.) is designed for humans reading text files. AI agents need:

- **Structured queries**: SQL-like filtering and aggregation
- **Causality tracking**: Understanding what caused what
- **Session reconstruction**: Replaying complete conversation timelines
- **Pattern detection**: Finding similar errors or events

Memdoor's logging system provides all of these capabilities through an event-based architecture with SQLite storage.

### Key Features

- **Event-Based**: Rich event types with metadata and causality
- **SQLite Storage**: Fast, queryable local database
- **Causality Chains**: Parent-child event relationships
- **Session Timeline**: Reconstruct complete conversation flows
- **Performance**: Buffered writes, indexed queries
- **Zero Config**: Works out of the box, no setup required

## Quick Start

### View Recent Logs

```bash
./memdoor logs tail
```

### Query Logs

```bash
# Last hour's errors
./memdoor logs query --since 1h --level ERROR

# All events from a session
./memdoor logs query --session session_main

# Find errors in last 24 hours
./memdoor logs errors
```

### Trace Causality

```bash
# Get event ID from logs
./memdoor logs tail

# Trace the causal chain
./memdoor logs trace <event-id>
```

### View Statistics

```bash
./memdoor logs stats
```

### Real-Time Log Tailing

```bash
# Follow all logs in real-time (like tail -f)
./memdoor logs tail --follow

# Follow only errors
./memdoor logs query --level ERROR --follow
```

### Ingest Logs from Other Commands

```bash
# Pipe command output into logs
echo "Server started" | ./memdoor logs write --component MyApp

# Ingest from file
cat access.log | ./memdoor logs write --component WebServer

# Real-time ingestion from another process
tail -f app.log | ./memdoor logs write --session app1
```

### Manage Log Archives

```bash
# Manually rotate current database to archive
./memdoor logs rotate

# Delete archives older than 7 days (preview first)
./memdoor logs clean --days 7 --dry-run

# Keep only 5 most recent archives
./memdoor logs clean --keep-archives 5
```

## CLI Commands

### logs tail

Show recent log entries with optional filtering and real-time following.

```bash
./memdoor logs tail [flags]
```

**Flags:**
- `--level` - Filter by log level (DEBUG, INFO, WARN, ERROR)
- `--component` - Filter by component name
- `--limit` - Maximum number of entries to show (default: 50)
- `-f, --follow` - Follow log output in real-time (like tail -f)

**Examples:**

```bash
# Last 50 entries
./memdoor logs tail

# Last 20 errors
./memdoor logs tail --level ERROR --limit 20

# WebSocket component events
./memdoor logs tail --component WebSocket

# Follow logs in real-time (Ctrl+C to exit)
./memdoor logs tail --follow

# Follow only errors as they occur
./memdoor logs tail --level ERROR --follow
```

### logs query

Advanced filtering and search with SQL-like capabilities and real-time following.

```bash
./memdoor logs query [flags]
```

**Flags:**
- `--since <duration>` - Time range (5m, 1h, 24h, 7d, 30d)
- `--session <id>` - Filter by session ID
- `--run <id>` - Filter by run ID
- `--regex <pattern>` - Regex pattern to search in messages (Go RE2 syntax)
- `--workspace-filter <slug>` - Events whose `data.workspace` is this slug
- `--order asc|desc` - Sort order
- `--data` - Show each event's id and data attributes (the ids the filters take)
- `--limit <n>` - Maximum results (default: 50)

**Regex Dialect:**

The `--regex` flag uses **Go's RE2 syntax** (https://pkg.go.dev/regexp/syntax), which is similar to PCRE but with some limitations for safety and performance:

- **Supported**: Most common regex features including character classes, quantifiers, anchors, groups, alternation
- **Case-insensitive**: Use `(?i)` flag (e.g., `--regex "(?i)error"`)
- **Not supported**: Backreferences, lookahead/lookbehind assertions
- **Performance**: Guaranteed linear time complexity (O(n)) - safe for untrusted input

**Examples:**

```bash
# Last hour's errors
./memdoor logs query --since 1h --level ERROR

# All events from specific session
./memdoor logs query --session session_main

# Last 7 days of WebSocket and Agent events
./memdoor logs query --since 7d --component WebSocket --component Agent

# Find all failures
./memdoor logs query --errors-only --limit 50

# Follow errors as they occur (Ctrl+C to exit)
./memdoor logs query --level ERROR --follow

# Follow specific session in real-time
./memdoor logs query --session session_main --follow

# Regex search for error patterns
./memdoor logs query --regex "error|fail|timeout"

# Case-insensitive regex search
./memdoor logs query --regex "(?i)connection"

# Find events with numbers (Event 1, Event 2, etc.)
./memdoor logs query --regex "Event [0-9]+"

# Regex with alternation (specific event IDs)
./memdoor logs query --regex "Event (5|7|9):"

# Combine regex with other filters
./memdoor logs query --regex "error" --component Agent --since 1h
```

**Piping Support:**

The query command outputs logs to stdout, allowing you to pipe to other CLI tools:

```bash
# Filter specific events with grep
./memdoor logs query --session session_main 2>/dev/null | grep "Error"

# Count matching events
./memdoor logs query --since 1h 2>/dev/null | wc -l

# Extract and process with awk
./memdoor logs query --component Agent 2>/dev/null | awk '{print $3}'
```

*Note: Use `2>/dev/null` to suppress status messages when piping.*

### logs trace

Reconstruct the complete causal chain for an event.

```bash
./memdoor logs trace <event-id>
```

**Output:**
- Root event (original trigger)
- Complete cause-effect chain
- Maximum depth
- Chronological ordering

**Example:**

```bash
$ ./memdoor logs trace abc123...

 Root Event: abc123-456-789
   Agent - User asked: What is the weather?

 Chain Statistics:
   Total Events: 3
   Max Depth: 2

 Causal Chain (chronological):
14:10:24 [INFO] Agent - User asked: What is the weather?
  → 14:10:24 [INFO] Agent - Calling weather API
  → 14:10:24 [INFO] Agent - Weather API returned: 72°F
```

### logs session

Reconstruct the complete timeline for a session.

```bash
./memdoor logs session <session-id>
```

**Output:**
- Session metadata (start time, event count)
- Complete chronological timeline
- All event types (messages, tools, errors)

**Example:**

```bash
$ ./memdoor logs session session_main

 Session: session_main
   Events: 42
   Started: 2026-02-25T10:30:00+01:00
   Latest: 2026-02-25T14:15:24+01:00

 Timeline:
10:30:15 INFO  [Agent] Session started
10:30:20 INFO  [Agent] User message received
...
```

### logs stats

Show comprehensive storage statistics.

```bash
./memdoor logs stats
```

**Output:**
- Total event count
- Events by level (with percentages)
- Time range (oldest → newest)
- Database size

**Example:**

```bash
$ ./memdoor logs stats

 Log Statistics

Total Events:  1,247

Events by Level:
  INFO         890 (71.4%)
  DEBUG        234 (18.8%)
  WARN         98 (7.9%)
  ERROR        25 (2.0%)

Time Range:
  Oldest: 2026-02-20T09:00:00+01:00
  Newest: 2026-02-25T14:15:24+01:00

Storage:
  Database Size: 2.4 MB
```

### logs errors

Quick error analysis with grouping by component.

```bash
./memdoor logs errors [--since <duration>]
```

**Flags:**
- `--since <duration>` - Time range (default: 24h)

**Example:**

```bash
$ ./memdoor logs errors --since 7d

 Found 12 errors in the last 7d

 WebSocket (8 errors):
   [14:10:24] Connection timeout
      Error: dial tcp: i/o timeout
   ...

 Agent (4 errors):
   [13:45:10] Tool execution failed
      Error: command not found: weather_api
   ...
```

### logs write

Ingest logs from stdin to the structured logging system. Useful for capturing output from other commands and storing them as structured events.

```bash
./memdoor logs write [flags]
```

**Flags:**
- `--component <name>` - Component name for the logs (default: "stdin")
- `--level <level>` - Log level (DEBUG, INFO, WARN, ERROR) (default: "INFO")
- `--session <id>` - Session ID for the logs (optional)
- `--format <type>` - Input format: "text" or "json" (default: "text")

**Input Formats:**

1. **Text format** (default): Each line becomes a separate log event
2. **JSON format**: Each line should be a JSON object with a "message" field

**Examples:**

```bash
# Simple text ingestion
echo "Server started" | ./memdoor logs write

# Ingest log file with custom component
cat access.log | ./memdoor logs write --component WebServer

# Real-time log following from another process
tail -f app.log | ./memdoor logs write --level INFO --session app1

# JSON format ingestion
echo '{"message": "User logged in", "user_id": 123}' | \
  ./memdoor logs write --format json --component Auth

# Pipe from command output
./my-script.sh | ./memdoor logs write --component MyScript --session script_run_1

# Multi-line ingestion
cat << EOF | ./memdoor logs write --component Test
Event 1: Starting process
Event 2: Processing data
Event 3: Completed successfully
EOF
```

**Use Cases:**

- **Legacy log ingestion**: Capture logs from applications without native structured logging
- **Script output**: Store script output as queryable events
- **Real-time monitoring**: Tail log files and store events for later analysis
- **Integration**: Pipe output from any command into the structured logging system
- **Agent debugging**: Capture tool output for agent self-debugging

**Performance:**

- Buffered writes (100 events or 1 second intervals)
- Progress displayed to stderr every 100 events (with --verbose)
- Graceful shutdown on Ctrl+C (flushes remaining events)

### logs rotate

Manually rotate the current log database to an archive.

```bash
./memdoor logs rotate [--verbose]
```

**What it does:**

- Moves the current `events.db` to `archives/events_YYYY-MM-DD.db`
- If an archive with the same date exists, adds a timestamp: `events_YYYY-MM-DD_HH-MM-SS.db`
- Also moves WAL and SHM files if they exist
- New database will be created automatically on next log event

**Flags:**
- `--verbose` - Show detailed rotation information

**Examples:**

```bash
# Manually rotate logs
./memdoor logs rotate

# Rotate with detailed output
./memdoor logs rotate --verbose
```

**When to use:**

- Before archiving or backing up logs
- When the database grows too large
- Before major system changes
- As part of regular maintenance

### logs clean

Delete old archived log files to free disk space.

```bash
./memdoor logs clean [flags]
```

**Flags:**
- `--days <N>` - Keep logs from last N days (default: 30)
- `--keep-archives <N>` - Keep N most recent archives (0 = unlimited)
- `--dry-run` - Preview what would be deleted without actually deleting

**Retention strategies:**

1. **Time-based**: `--days 30` keeps logs from last 30 days
2. **Count-based**: `--keep-archives 10` keeps only 10 most recent archives
3. **Combined**: Both flags can be used together (archives must meet both criteria to be kept)

**Examples:**

```bash
# Delete archives older than 30 days (default)
./memdoor logs clean

# Keep only last 7 days
./memdoor logs clean --days 7

# Keep only 5 most recent archives
./memdoor logs clean --keep-archives 5

# Preview what would be deleted
./memdoor logs clean --days 7 --dry-run

# Aggressive cleanup: keep only 3 most recent
./memdoor logs clean --keep-archives 3
```

**Safety features:**

- Shows detailed preview of what will be deleted (size, age)
- Requires user confirmation (y/N prompt)
- `--dry-run` flag for safe preview
- Deletes associated WAL and SHM files automatically

**Example output:**

```bash
$ ./memdoor logs clean --keep-archives 1 --dry-run

🗑️  Found 1 archives to delete (0.07 MB total):

  - events_2026-02-25.db (0.07 MB, 0 days old)

 This was a dry run. Use without --dry-run to actually delete.
```

## Event System

### Event Types

Memdoor logs 24 different event types:

**Lifecycle Events:**
- `agent_started`, `agent_stopped`
- `session_started`, `session_ended`
- `run_started`, `run_completed`

**Message Events:**
- `message_received` - User messages
- `message_sent` - Assistant responses

**Tool Events:**
- `tool_called` - Tool execution started
- `tool_completed` - Tool execution finished
- `tool_failed` - Tool execution error

**Processing Events:**
- `thinking_started`, `thinking_finished`
- `compaction_started`, `compaction_completed`

**State Events:**
- `state_change` - General state transitions
- `error` - Error events
- `metric` - Performance metrics

### Event Structure

Each event contains:

```go
{
  "id": "unique-event-id",
  "timestamp": "2026-02-25T14:10:24Z",
  "type": "tool_called",
  "component": "Agent",
  "level": "INFO",           // DEBUG, INFO, WARN, ERROR

  // Context
  "session": "session_main",
  "run_id": "run_123",
  "span_id": "span_456",

  // Causality
  "parent_id": "parent-event-id",  // What triggered this
  "root_id": "root-event-id",      // Original root cause

  // Content
  "message": "Calling weather API",
  "data": { ... },           // Structured metadata
  "error": { ... },          // Error details if failed
  "duration": 250000000,     // Nanoseconds
  "success": true
}
```

### Log Levels

- **DEBUG**: Verbose diagnostic information (only logged when `--verbose`)
- **INFO**: Normal operational events
- **WARN**: Warning conditions that should be addressed
- **ERROR**: Error events that need attention

## Causality Tracking

### Parent-Child Relationships

Every event can have a parent event, creating a cause-effect chain:

```
User Message (root)
  ├─ Tool Call: weather_api (depth 1)
  │   ├─ HTTP Request (depth 2)
  │   └─ Response Parsed (depth 2)
  └─ Assistant Response (depth 1)
```

### Root Cause Analysis

The `logs trace` command reconstructs the entire causal chain from any event ID:

```bash
$ ./memdoor logs trace <event-id>
```

This shows:
- The root event that started the chain
- All child events in chronological order
- Maximum depth of the chain
- Total events involved

### Session Context

All events within a session are automatically linked:

```bash
$ ./memdoor logs session session_main
```

This reconstructs the complete timeline showing:
- User messages
- Tool executions
- Errors
- State changes

## Query Interface

### Fluent Query Builder (Programming)

```go
import "memdoor/gateway/logs"

qb := logs.NewQueryBuilder().
    Since(1 * time.Hour).
    Levels(logs.LevelError, logs.LevelWarn).
    Components("WebSocket", "Agent").
    ErrorsOnly().
    Limit(50)

query := qb.Build()
result, err := storage.QueryEvents(ctx, query)
```

### Time Filtering

```go
// Last N duration
qb.Since(1 * time.Hour)
qb.Since(7 * 24 * time.Hour) // 7 days

// Specific time range
qb.After(startTime)
qb.Before(endTime)
```

### Multiple Filters

```go
// Filter by levels (OR)
qb.Levels(logs.LevelError, logs.LevelWarn)

// Filter by components (OR)
qb.Components("WebSocket", "Agent", "Compaction")

// Filter by event types
qb.EventTypes(logs.EventToolCalled, logs.EventToolCompleted)
```

### Context Filters

```go
// Session-specific events
qb.Session("session_main")

// Run-specific events
qb.RunID("run_123")

// Span-specific events
qb.SpanID("span_456")
```

### Causality Queries

```go
// Find all children of an event
qb.ParentID("parent-event-id")

// Find all events in a causal chain
qb.RootID("root-event-id")
```

### Text Search

```go
// Search message content
qb.MessageContains("timeout")

// Search error messages
qb.ErrorContains("connection refused")
```

### Success/Failure Filter

```go
// Only successful events
qb.SuccessOnly()

// Only failed events
qb.ErrorsOnly()
```

### Pagination

```go
// Limit and offset
qb.Limit(100)
qb.Offset(200)

// Sort order
qb.Ascending()  // Oldest first (default: descending)
```

## Programming Interface

### Creating a Logger

```go
import "memdoor/gateway/logs"

// With SQLite storage
logger, err := logs.NewEventLoggerWithSQLite(
    "ComponentName",
    "~/.memdoor/logs",  // logs directory
    true,                  // verbose mode
)
defer logger.Close()
```

### Basic Logging

```go
// Standard log levels
logger.Debug("Verbose diagnostic info",
    logs.String("key", "value"))

logger.Info("Normal operation",
    logs.Int("count", 42))

logger.Warn("Warning condition",
    logs.String("reason", "timeout"))

logger.Error("Error occurred", err,
    logs.String("context", "database"))
```

### Structured Fields

```go
import "memdoor/gateway/logs"

// Type-safe field constructors
logs.String("key", "value")
logs.Int("count", 42)
logs.Int64("bytes", int64(1024))
logs.Float64("ratio", 0.95)
logs.Bool("success", true)
logs.Duration("elapsed", time.Second)
logs.Time("started_at", time.Now())
logs.Error(err)
logs.Any("data", complexStruct)
```

### Custom Events

```go
// Create custom event
event := logs.NewEvent(
    logs.EventToolCalled,
    "Agent",
    "Calling weather API",
).
    WithSession("session_main").
    WithLevel(logs.LevelInfo).
    WithData("tool", "weather_api").
    WithData("args", map[string]any{
        "city": "Seattle",
    })

logger.EmitEvent(event)
```

### Causality Tracking

```go
// Root event
root := logs.NewEvent(
    logs.EventMessageReceived,
    "Agent",
    "User asked: What is the weather?",
).WithSession("session_main")

logger.EmitEvent(root)

// Child event (caused by root)
child := logs.NewEvent(
    logs.EventToolCalled,
    "Agent",
    "Calling weather API",
).
    WithSession("session_main").
    WithParent(root.ID).
    WithRoot(root.ID)

logger.EmitEvent(child)
```

### Span Tracking

Spans track operations with automatic start/finish events:

```go
// Start a span
span := logger.StartSpan("processMessage", logs.EventMessageReceived)
span.SetTag("message_type", "user_query")

// Log within span
span.Info("Processing user message")
span.Debug("Validating input")

// Finish span (automatically logs completion)
span.Finish()

// Or finish with error
span.FinishWithError(err)
```

### Child Loggers

Create child loggers that inherit context:

```go
// Parent logger
logger := logs.NewEventLoggerWithSQLite("Gateway", logsDir, true)

// Child logger with session context
sessionLogger := logger.WithSession("session_main")

// Events from sessionLogger automatically include session
sessionLogger.Info("Session event")  // session="session_main"

// Chain context
runLogger := sessionLogger.WithRun("run_123")
runLogger.Info("Run event")  // session="session_main", run_id="run_123"
```

### Querying Logs

```go
import "memdoor/gateway/logs"

// Open storage
storage, err := logs.NewSQLiteStorage("~/.memdoor/logs/events.db")
defer storage.Close()

// Build query
query := logs.NewQueryBuilder().
    Since(1 * time.Hour).
    Levels(logs.LevelError).
    Components("WebSocket").
    Build()

// Execute query
result, err := storage.QueryEvents(ctx, query)

// Process results
for _, event := range result.Events {
    fmt.Printf("%s [%s] %s - %s\n",
        event.Timestamp.Format(time.RFC3339),
        event.Level,
        event.Component,
        event.Message,
    )
}
```

### Trace Causality

```go
// Reconstruct causal chain
chain, err := storage.TraceChain(ctx, eventID)

fmt.Printf("Root: %s\n", chain.RootEvent.Message)
fmt.Printf("Total Events: %d\n", chain.TotalEvents)
fmt.Printf("Max Depth: %d\n", chain.Depth)

for _, event := range chain.Chain {
    fmt.Printf("  → %s\n", event.Message)
}
```

### Reconstruct Session

```go
// Get all events for a session
events, err := storage.ReconstructSession(ctx, "session_main")

// Events are in chronological order
for _, event := range events {
    fmt.Printf("[%s] %s\n", event.Type, event.Message)
}
```

### Storage Statistics

```go
stats, err := storage.GetStats(ctx)

fmt.Printf("Total Events: %d\n", stats.TotalEvents)
fmt.Printf("Database Size: %d bytes\n", stats.DatabaseSize)

for level, count := range stats.EventsByLevel {
    fmt.Printf("%s: %d\n", level, count)
}
```

## Best Practices

### 1. Use Structured Fields

**Good:**
```go
logger.Info("User message received",
    logs.String("session", sessionID),
    logs.Int("length", len(message)),
    logs.String("source", "websocket"),
)
```

**Bad:**
```go
logger.Info(fmt.Sprintf("User message received: session=%s, length=%d", sessionID, len(message)))
```

### 2. Track Causality

Always link related events:

```go
// Root event
root := logs.NewEvent(...).WithSession(sessionID)
logger.EmitEvent(root)

// Child event
child := logs.NewEvent(...).
    WithSession(sessionID).
    WithParent(root.ID).
    WithRoot(root.ID)
logger.EmitEvent(child)
```

### 3. Use Spans for Operations

```go
span := logger.StartSpan("handleRequest", logs.EventMessageReceived)
defer span.Finish()

span.Info("Validating input")
// ... work ...
span.Info("Request completed")
```

### 4. Choose Appropriate Levels

- **DEBUG**: Verbose details only needed during development
- **INFO**: Normal operational events
- **WARN**: Potential issues that don't prevent operation
- **ERROR**: Failures that need attention

### 5. Include Context

Always include session and run IDs:

```go
sessionLogger := logger.WithSession(sessionID)
runLogger := sessionLogger.WithRun(runID)

runLogger.Info("Processing message")
```

### 6. Handle Errors Gracefully

```go
if err := operation(); err != nil {
    logger.Error("Operation failed", err,
        logs.String("operation", "process_message"),
        logs.String("session", sessionID),
    )
    return err
}
```

### 7. Use Child Loggers

Instead of passing sessionID everywhere:

```go
// Bad
func handleMessage(logger *logs.EventLogger, sessionID string) {
    logger.Info("Handling message", logs.String("session", sessionID))
}

// Good
func handleMessage(logger *logs.EventLogger) {
    logger.Info("Handling message")  // session auto-included
}

// Usage
sessionLogger := logger.WithSession(sessionID)
handleMessage(sessionLogger)
```

### 8. Query Efficiently

Use indexes effectively:

```go
// Good - uses timestamp index
qb.Since(1 * time.Hour).Levels(logs.LevelError)

// Good - uses session index
qb.Session("session_main")

// Less efficient - full text search
qb.MessageContains("timeout")  // Use sparingly
```

## Storage Location

- **Database**: `~/.memdoor/logs/events.db`
- **SQLite WAL files**: `events.db-wal`, `events.db-shm`
- **Size**: Approximately 500 bytes per event

## Performance Characteristics

- **Write throughput**: ~10,000 events/second (buffered)
- **Query latency**: <1ms for indexed queries
- **Storage overhead**: ~500 bytes/event
- **Buffering**: 100 events or 1 second (whichever comes first)
- **Indexes**: 10+ indexes for fast queries

## Troubleshooting Guide

**Agent Diagnostic Checklist:**

```
Problem: Not finding expected events
├─ Database exists? → ls -lh ~/.memdoor/logs/events.db
├─ Events present? → ./memdoor logs stats
├─ Time filter correct? → Check --since parameter
└─ Regex syntax? → Test with simpler pattern first

Problem: Commands failing
├─ Database not found → Start gateway to create it
├─ Permission denied → Check ~/.memdoor/logs/ permissions
└─ Command not found → Build binary: go build -o memdoor ./cmd/cli/main.go

Problem: No output from queries
├─ Check filters → Remove filters one at a time
├─ Check time range → Expand --since parameter
└─ Verify events exist → ./memdoor logs stats

Problem: Disk space issues
├─ Check database size → ./memdoor logs stats
├─ Rotate logs → ./memdoor logs rotate
└─ Clean old archives → ./memdoor logs clean --days 7
```

### Events not appearing

**Symptoms**: Query returns no results, `./memdoor logs tail` is empty

**Diagnostic steps**:

```bash
# 1. Check if database exists
ls -lh ~/.memdoor/logs/events.db

# 2. Check event count
./memdoor logs stats

# 3. Check if gateway is logging
./memdoor logs tail --limit 1

# 4. If empty, start gateway to create events
./memdoor gateway --verbose &
```

**Solutions**:
- Database doesn't exist → Start gateway to create it automatically
- Database exists but empty → Gateway not logging yet, wait for activity
- Old events only → Check time filters (--since parameter)

### Slow queries

**Symptoms**: Queries take >1 second, `--verbose` shows high query time

**Diagnostic steps**:

```bash
# 1. Check query time
./memdoor logs query --since 7d --verbose

# 2. Check database size
./memdoor logs stats

# 3. Check number of events
sqlite3 ~/.memdoor/logs/events.db "SELECT COUNT(*) FROM events;"
```

**Solutions**:
- Narrow time range: `--since 1h` instead of `--since 7d`
- Add more filters: `--component Agent --level ERROR`
- Use indexed fields: session, component, level (fast) vs regex (slower)
- Rotate large databases: `./memdoor logs rotate`

### Database locked errors

**Symptoms**: `database is locked` error when querying

**Root cause**: Multiple writers or WAL checkpoint conflict

**Solutions**:

```bash
# 1. Check for multiple gateway instances
ps aux | grep memdoor-gateway

# 2. Stop extra instances
pkill -f memdoor-gateway

# 3. Restart single gateway
./memdoor gateway --verbose &
```

**Prevention**:
- Only one logger instance per process
- Multiple readers are safe
- WAL mode minimizes lock contention

### Regex queries returning no results

**Symptoms**: `--regex` finds nothing, but manual grep works

**Root cause**: RE2 syntax differs from PCRE/grep

**Solutions**:

```bash
# Test with simpler patterns first
./memdoor logs query --regex "error"

# RE2 doesn't support backreferences - use alternatives
# Bad: --regex "(\w+) \1"
# Good: --regex "\w+ \w+"

# Use (?i) for case-insensitive
./memdoor logs query --regex "(?i)error"
```

**RE2 limitations**:
- No backreferences: `\1`, `\2` don't work
- No lookahead/lookbehind: `(?=...)`, `(?<=...)` not supported
- Use alternation instead: `error|fail|timeout`

### Archive cleanup not deleting files

**Symptoms**: `./memdoor logs clean` shows files but doesn't delete

**Root cause**: Dry-run mode or confirmation cancelled

**Solutions**:

```bash
# 1. Check if using --dry-run
./memdoor logs clean --days 7 --dry-run

# 2. Run without --dry-run and confirm with 'y'
./memdoor logs clean --days 7
# Type 'y' when prompted

# 3. Use automation-friendly approach
echo "y" | ./memdoor logs clean --days 7
```

### Real-time follow not showing new events

**Symptoms**: `--follow` flag shows old events but not new ones

**Diagnostic steps**:

```bash
# 1. Verify new events are being created
./memdoor logs stats

# 2. Check last event timestamp
./memdoor logs tail --limit 1

# 3. Test with explicit time filter
./memdoor logs query --since 1m --follow
```

**Solutions**:
- Events cached → Wait for flush (100 events or 1 second)
- Gateway not running → Start gateway
- Filters too restrictive → Remove filters to test

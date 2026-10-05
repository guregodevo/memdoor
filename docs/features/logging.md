# Structured Logging

Memdoor provides powerful structured logging with SQLite-based storage and full-text search capabilities.

## Overview

All system activity is logged to a SQLite database with:
- Structured fields (level, component, message, metadata)
- Full-text search (SQLite FTS5)
- Efficient querying and filtering
- Retention policies

## Log Storage

Logs are stored in:
```
~/.memdoor/logs/memdoor.db
```

Schema:
```sql
CREATE TABLE logs (
    id INTEGER PRIMARY KEY,
    timestamp INTEGER NOT NULL,
    level TEXT NOT NULL,
    component TEXT NOT NULL,
    message TEXT NOT NULL,
    metadata JSON,
    user_id TEXT,
    workspace_id TEXT,
    channel_id TEXT,
    agent_id TEXT
);

-- Full-text search index
CREATE VIRTUAL TABLE logs_fts USING fts5(message, metadata);
```

## Log Levels

- **DEBUG**: Detailed debugging information
- **INFO**: General informational messages
- **WARN**: Warning messages (potential issues)
- **ERROR**: Error messages (failures)
- **FATAL**: Critical errors (system crashes)

## CLI Commands

### Query Logs

View recent logs:
```bash
./memdoor logs query --limit 50
```

With time filtering:
```bash
./memdoor logs query --since 5m --limit 20
./memdoor logs query --since 1h --limit 100
./memdoor logs query --since 2024-03-18 --limit 200
```

### Search Logs

Pattern matching:
```bash
./memdoor logs query --regex "ERROR|WARN" --limit 20
./memdoor logs query --regex "agent.*coder" --limit 10
./memdoor logs query --regex "A2A|mention_depth" --limit 15
```

### Filter by Level

ERROR and WARN are the `errors` view; there is no `--level` flag (the
filter is the message regex above).

```bash
./memdoor logs errors --limit 30
./memdoor logs errors --since 1h
```

### Filters (what `logs query` really takes)

`--regex` matches the message field; `--session`, `--run` and
`--workspace-filter` narrow by the ids `--data` prints, and `--since`,
`--order` and `--limit` shape the window. There is no `--level` or
`--component` filter — a level is `logs errors` for ERROR/WARN and the
message text for the rest:

```bash
./memdoor logs query --regex 'tool call' --limit 20
./memdoor logs query --session <session> --limit 50
./memdoor logs query --run <run-id> --data
./memdoor logs query --workspace-filter general --limit 30
```

### Following a Log

Nothing follows a log live any more (`logs tail` and `--follow` were
removed with the other thin wrappers): re-run `logs query --since 5m`,
which is cheap because it reads the same SQLite store.

### Export Logs

Export to JSON:
```bash
./memdoor logs query --limit 1000 --data > logs.json
```

Export to CSV:
```bash
# one-shot query; the CLI prints the rows, jq/csvtool shapes them
./memdoor logs query --limit 1000 --data | jq -r '[.timestamp,.component,.message]|@csv' > logs.csv
```

## Common Debugging Patterns

### Debug Agent Execution

```bash
# View agent activity
./memdoor logs query --regex "agent_id.*coder" --limit 20 --since 5m

# Check agent mentions
./memdoor logs query --regex "A2A|mention" --limit 15
```

### Debug Authentication Issues

```bash
./memdoor logs query --regex "auth|token" --limit 10 --since 10m
```

### Debug RAG System

```bash
./memdoor logs query --regex "RAG|embedding|vector" --limit 20
```

### Debug WebSocket Issues

```bash
./memdoor logs query --regex 'websocket' --limit 15
```

### Debug Performance

```bash
# Find slow operations
./memdoor logs query --regex "duration.*[5-9][0-9][0-9]ms" --limit 20

# Database query performance
./memdoor logs query --regex "query.*slow" --limit 10
```

### Debug Message Threading

```bash
./memdoor logs query --regex "parent_message|thread" --limit 20
```

## Log Rotation

Logs are automatically rotated based on:
- **Size**: When database exceeds 100MB
- **Age**: Logs older than 30 days (configurable)

Manual rotation:
```bash
./memdoor logs rotate
```

Configuration in `~/.memdoor/config.yaml`:
```yaml
logging:
  rotation:
    max_size_mb: 100
    max_age_days: 30
    keep_count: 5  # Keep 5 rotated files
```

## Log Retention

Configure retention policy:
```yaml
logging:
  retention:
    # Delete logs older than
    max_age_days: 90

    # Keep errors longer
    error_retention_days: 180

    # Compact old logs
    compact_after_days: 30
```

Apply retention policy:
```bash
./memdoor logs prune --days 30
./memdoor logs prune --before 720h
./memdoor logs prune --all
```

## Programmatic Logging

### In Go Code

```go
import "memdoor/gateway/logs"

// Get logger
log := logs.GetLogger()

// Log messages
log.Info("User logged in", slog.String("user_id", userID))
log.Warn("Rate limit exceeded", slog.Int("requests", count))
log.Error("Database query failed", slog.Any("error", err))

// With context
log.Debug("Processing message",
    slog.String("message_id", msgID),
    slog.String("channel_id", channelID),
    slog.String("agent_id", agentID),
)
```

### Best Practices

1. **Use structured fields**:
```go
// Good
log.Info("Message sent",
    slog.String("message_id", msgID),
    slog.String("channel", channel),
)

// Bad
log.Info(fmt.Sprintf("Message %s sent to %s", msgID, channel))
```

2. **Use appropriate levels**:
```go
// DEBUG: Detailed trace
log.Debug("Entering function", slog.String("func", "processMessage"))

// INFO: Normal operation
log.Info("Message processed", slog.String("message_id", msgID))

// WARN: Potential issue
log.Warn("Slow query", slog.Duration("duration", elapsed))

// ERROR: Operation failed
log.Error("Failed to save", slog.Any("error", err))
```

3. **Include context**:
```go
log.Info("Agent executed",
    slog.String("agent_id", agentID),
    slog.String("user_id", userID),
    slog.String("workspace_id", workspaceID),
    slog.Duration("duration", elapsed),
)
```

## Log Analysis

### Common Queries

**Find errors in last hour:**
```bash
./memdoor logs errors --since 1h
```

**Find slow operations:**
```bash
./memdoor logs query --regex "duration.*[0-9]{4,}ms" --limit 20
```

**Find authentication failures:**
```bash
./memdoor logs query --regex "auth.*fail|invalid.*token" --limit 10
```

**Find agent loops:**
```bash
./memdoor logs query --regex "mention_depth.*[3-9]|infinite.*loop" --limit 10
```

**Find database errors:**
```bash
./memdoor logs query --regex "database.*error|sqlite.*error" --limit 15
```

### Log Aggregation

`logs stats` takes no flags — it prints the counts it has, and the grouping
is `jq` on a `--data` query:

```bash
./memdoor logs stats
./memdoor logs query --data --limit 500 | jq -r '.data.component' | sort | uniq -c | sort -rn
```

## Performance Monitoring

### Indexing Performance

Logs are indexed for fast queries:
- **Timestamp index**: Fast time-range queries
- **Level index**: Fast filtering by level
- **FTS index**: Fast text search
- **Component index**: Fast component filtering

### Query Performance

Typical query times:
- **Recent logs** (last 1000): < 10ms
- **Filtered queries**: 10-50ms
- **Full-text search**: 20-100ms
- **Large exports**: 100ms-1s

Optimize queries:
```bash
# Good: Time-limited query
./memdoor logs query --since 1h --limit 100

# Bad: Unbounded query
./memdoor logs query --limit 1000000
```

## Integration

### CI/CD

```yaml
# .github/workflows/test.yml
- name: Run tests and capture logs
  run: |
    ./memdoor gateway start
    npm test
    ./memdoor logs errors --since 5m > test-errors.log
```

### Monitoring

```bash
#!/bin/bash
# monitor.sh - Alert on errors
while true; do
  errors=$(./memdoor logs errors --since 1m | grep -c '^')
  if [ "$errors" -gt 10 ]; then
    echo "High error rate: $errors errors in last minute"
  fi
  sleep 60
done
```

### Log Forwarding

Forward logs to external systems:
```bash
# To Elasticsearch
./memdoor logs query --since 1m --data | \
  curl -X POST "http://localhost:9200/memdoor-logs/_bulk" \
    -H "Content-Type: application/json" \
    -d @-
```

## Troubleshooting

### "Database locked" error

```bash
# Stop all processes
./memdoor gateway stop

# Remove lock files
rm ~/.memdoor/logs/*.db-wal
rm ~/.memdoor/logs/*.db-shm

# Restart
./memdoor gateway start
```

### Large database size

```bash
# Check size
du -h ~/.memdoor/logs/memdoor.db

# Cleanup old logs
./memdoor logs prune --days 30
```

### Slow queries

```bash
# Add indexes if needed
./memdoor logs reindex

# Limit query scope
./memdoor logs query --since 1h --limit 100  # Instead of unlimited
```

## See Also

- [CLI Reference](../reference/CLI.md)
- [Configuration](../architecture/CONFIG_SYSTEM_DESIGN.md)
- [Logs Reference](../reference/LOGS.md)

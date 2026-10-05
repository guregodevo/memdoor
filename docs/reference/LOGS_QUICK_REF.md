# Memdoor Logs - Quick Reference for AI Agents

**Version**: 1.0
**Date**: 2026-04-18

**Purpose**: Agent-first logging system with SQL-like queries, causality tracking, and real-time monitoring.

**Location**: `~/.memdoor/logs/events.db`

## Command Selection Matrix

| Need | Command | Example |
|------|---------|---------|
| **Find recent errors** | `logs errors --since <time>` | `logs errors --since 1h` |
| **Search pattern** | `logs query --regex "<pattern>"` | `logs query --regex "timeout"` |
| **Root cause** | `logs trace <event-id>` | `logs trace abc-123-def` |
| **Session replay** | `logs session <session-id>` | `logs session session_main` |
| **Real-time monitor** | `logs tail --follow` | `logs tail --follow` |
| **Component debug** | `logs query --component <name>` | `logs query --component Agent` |
| **System health** | `logs stats` | `logs stats` |
| **Ingest external** | `<cmd> \| logs write --component <name>` | `tail -f app.log \| logs write` |
| **Archive logs** | `logs rotate` | `logs rotate` |
| **Free disk space** | `logs clean --days <N>` | `logs clean --days 7` |

## Common Patterns

```bash
# Pattern 1: Debug recent failures
./memdoor logs errors --since 1h

# Pattern 2: Find specific error type
./memdoor logs query --regex "connection.*refused\|timeout" --since 24h

# Pattern 3: Trace root cause of error
EVENT_ID=$(./memdoor logs query --level ERROR --limit 1 | grep -oE "[a-f0-9-]{36}")
./memdoor logs trace $EVENT_ID

# Pattern 4: Monitor specific component
./memdoor logs query --component WebSocket --follow

# Pattern 5: Session debugging
./memdoor logs session session_main

# Pattern 6: Disk cleanup
./memdoor logs clean --dry-run  # Preview
./memdoor logs clean --days 30  # Execute
```

## Query Filters

| Filter | Flag | Example |
|--------|------|---------|
| Time range | `--since <duration>` | `--since 1h`, `--since 7d` |
| Log level | `--level <LEVEL>` | `--level ERROR` |
| Component | `--component <name>` | `--component Agent` |
| Session | `--session <id>` | `--session session_main` |
| Regex | `--regex "<pattern>"` | `--regex "error\|fail"` |
| Follow | `--follow` | `--follow` (real-time) |
| Limit | `--limit <N>` | `--limit 50` |
| Verbose | `--verbose` | `--verbose` (detailed output) |

**Regex Dialect**: Go RE2 (no backreferences, no lookahead/lookbehind)

## Diagnostic Decision Tree

```
No events found?
├─ DB exists? → ls ~/.memdoor/logs/events.db
├─ Events? → ./memdoor logs stats
├─ Filters? → Remove --since, --level, etc.
└─ Regex? → Test with simpler pattern

Query slow?
├─ Narrow --since → Use 1h instead of 7d
├─ Add filters → --component, --level
└─ Rotate DB → ./memdoor logs rotate

Database locked?
├─ Multiple gateways? → ps aux | grep memdoor
└─ Kill extras → pkill -f memdoor-gateway

Archive cleanup fails?
├─ Dry-run? → Remove --dry-run flag
└─ Confirm → Type 'y' or echo "y" |
```

## Output Formats

**JSON Output**: Pipe to `jq` for processing:
```bash
./memdoor logs query --since 1h 2>/dev/null | jq '.'
```

**Piping**: Status messages go to stderr, data to stdout:
```bash
./memdoor logs query --component Agent 2>/dev/null | grep ERROR
```

## Performance

- **Write**: ~10,000 events/sec (buffered)
- **Query**: <1ms (indexed fields)
- **Storage**: ~500 bytes/event
- **Buffer**: 100 events or 1 second

## File Locations

- **Current DB**: `~/.memdoor/logs/events.db`
- **Archives**: `~/.memdoor/logs/archives/events_YYYY-MM-DD.db`
- **WAL files**: `events.db-wal`, `events.db-shm`

## Safety Features

- **Dry-run**: `--dry-run` previews without executing
- **Confirmation**: User prompt for destructive operations
- **Backups**: WAL files archived during rotation
- **Retention**: Default 30-day archive retention

## Programming Interface

```go
import "memdoor/gateway/logs"

// Query logs
qb := logs.NewQueryBuilder().
    Since(1 * time.Hour).
    Levels(logs.LevelError).
    Build()
result, err := storage.QueryEvents(ctx, qb)

// Trace causality
chain, err := storage.TraceChain(ctx, eventID)

// Session replay
events, err := storage.ReconstructSession(ctx, sessionID)
```

## Quick Troubleshooting

| Problem | Solution |
|---------|----------|
| No events | Start gateway: `./memdoor gateway &` |
| Query slow | Add filters or narrow time range |
| DB locked | Kill duplicate gateways |
| Regex fails | Use simpler pattern, check RE2 syntax |
| Follow stale | Wait for buffer flush (1 sec) |
| Cleanup skips | Remove `--dry-run`, confirm with 'y' |

---

**Full Docs**: `docs/reference/LOGS.md`

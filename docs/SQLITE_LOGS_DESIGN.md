# SQLite-Based Queryable Log System Design

**Status**: Design Phase
**Author**: Claude
**Date**: 2026-02-25
**Version**: 1.0

## Overview

Replace the current file-based logging with a SQLite database that enables:
- **Agent-Queryable Logs**: Agents can write SQL to debug themselves
- **Efficient Queries**: Indexed searches by time, level, component, session
- **Time Partitioning**: Daily partitions for scalability
- **Retention Policies**: Auto-delete old logs per severity level
- **Structured Data**: JSON metadata for complex log context

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                       Application                            │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   Gateway   │  │     CLI      │  │     TUI      │       │
│  └──────┬──────┘  └──────┬───────┘  └──────┬───────┘       │
│         │                │                  │                │
│         └────────────────┼──────────────────┘                │
│                          │                                   │
│                          ▼                                   │
│                 ┌─────────────────┐                          │
│                 │  logger.Logger  │                          │
│                 │   (existing)    │                          │
│                 └────────┬────────┘                          │
│                          │                                   │
│                          ▼                                   │
│         ┌────────────────────────────────────┐               │
│         │      slog.Handler (custom)         │               │
│         │  ┌──────────────────────────────┐  │               │
│         │  │   SQLiteHandler              │  │               │
│         │  │  - Buffers writes            │  │               │
│         │  │  - Parses slog.Record        │  │               │
│         │  │  - Extracts metadata         │  │               │
│         │  └──────────────────────────────┘  │               │
│         └────────────────┬───────────────────┘               │
│                          │                                   │
│                          ▼                                   │
│         ┌────────────────────────────────────┐               │
│         │       gateway/logs/writer.go       │               │
│         │  - Batch inserts (100 entries)     │               │
│         │  - Async flushing (1 second)       │               │
│         │  - Transaction management          │               │
│         └────────────────┬───────────────────┘               │
│                          │                                   │
└──────────────────────────┼───────────────────────────────────┘
                           │
                           ▼
         ┌─────────────────────────────────────┐
         │      ~/.memdoor/logs/              │
         │  ┌────────────────────────────────┐  │
         │  │  current.db (active writes)    │  │
         │  ├────────────────────────────────┤  │
         │  │  2026-02-25.db (yesterday)     │  │
         │  ├────────────────────────────────┤  │
         │  │  2026-02-24.db (older)         │  │
         │  ├────────────────────────────────┤  │
         │  │  2026-02-20.db.gz (archived)   │  │
         │  └────────────────────────────────┘  │
         │                                       │
         │  ┌────────────────────────────────┐  │
         │  │  index.db (partition metadata) │  │
         │  └────────────────────────────────┘  │
         └───────────────────────────────────────┘
```

## Database Schema

### Main Logs Table

```sql
-- logs table (in each partition DB file)
CREATE TABLE logs (
    -- Primary key
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Time (dual format for performance + readability)
    timestamp INTEGER NOT NULL,              -- Unix timestamp (seconds since epoch)
    time TEXT NOT NULL,                      -- ISO 8601: "2026-02-25T10:30:45.123Z"

    -- Core fields
    level TEXT NOT NULL,                     -- DEBUG, INFO, WARN, ERROR
    component TEXT NOT NULL,                 -- WebSocket, Agent, Queue, etc.
    message TEXT NOT NULL,                   -- Human-readable message

    -- Context (nullable)
    session TEXT,                            -- Session ID
    run_id TEXT,                             -- Run ID for agent executions
    error TEXT,                              -- Error message/stack trace

    -- Structured metadata (JSON)
    metadata TEXT,                           -- JSON object with additional fields

    -- Constraints
    CHECK (level IN ('DEBUG', 'INFO', 'WARN', 'ERROR')),
    CHECK (timestamp > 0)
);

-- Indexes for fast queries
CREATE INDEX idx_timestamp ON logs(timestamp);
CREATE INDEX idx_level ON logs(level);
CREATE INDEX idx_component ON logs(component);
CREATE INDEX idx_session ON logs(session) WHERE session IS NOT NULL;
CREATE INDEX idx_run_id ON logs(run_id) WHERE run_id IS NOT NULL;

-- Composite indexes for common query patterns
CREATE INDEX idx_time_level ON logs(timestamp DESC, level);
CREATE INDEX idx_time_component ON logs(timestamp DESC, component);
CREATE INDEX idx_session_time ON logs(session, timestamp DESC) WHERE session IS NOT NULL;

-- Full-text search (optional, adds overhead)
CREATE VIRTUAL TABLE logs_fts USING fts5(
    message,
    error,
    content='logs',
    content_rowid='id'
);
```

### Index Database (index.db)

```sql
-- Tracks all partition files
CREATE TABLE partitions (
    -- Partition identifier
    partition_date TEXT PRIMARY KEY,         -- "2026-02-25"

    -- File info
    db_file TEXT NOT NULL,                   -- "2026-02-25.db" or "2026-02-25.db.gz"
    is_compressed INTEGER DEFAULT 0,         -- 0 = no, 1 = yes

    -- Stats
    record_count INTEGER DEFAULT 0,          -- Number of log entries
    file_size INTEGER DEFAULT 0,             -- Bytes

    -- Timestamps
    created_at INTEGER NOT NULL,             -- When partition was created
    last_written INTEGER NOT NULL,           -- Last write timestamp
    compressed_at INTEGER,                   -- When compressed (NULL if not)

    -- State
    is_active INTEGER DEFAULT 0              -- 1 if current partition
);

-- Retention policies per log level
CREATE TABLE retention_policies (
    level TEXT PRIMARY KEY,                  -- DEBUG, INFO, WARN, ERROR
    retention_days INTEGER NOT NULL,         -- How long to keep

    CHECK (retention_days > 0),
    CHECK (level IN ('DEBUG', 'INFO', 'WARN', 'ERROR'))
);

-- Default retention policies
INSERT INTO retention_policies VALUES
    ('DEBUG', 7),        -- Debug: 7 days
    ('INFO', 30),        -- Info: 30 days
    ('WARN', 90),        -- Warning: 90 days
    ('ERROR', 365);      -- Error: 1 year

-- Query statistics (for optimization)
CREATE TABLE query_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    query_pattern TEXT NOT NULL,            -- Normalized query
    execution_time_ms INTEGER NOT NULL,     -- How long it took
    rows_scanned INTEGER,                   -- Rows examined
    rows_returned INTEGER                   -- Rows returned
);

-- Maintenance log
CREATE TABLE maintenance_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    operation TEXT NOT NULL,                -- "partition_rotate", "retention_cleanup", "vacuum"
    details TEXT,                           -- JSON with operation details
    success INTEGER NOT NULL,               -- 0 = failed, 1 = success
    duration_ms INTEGER
);
```

## Module Structure

```
gateway/logs/
├── schema.go           # Database schema definitions
├── writer.go           # Log writer with buffering
├── reader.go           # Query interface
├── partition.go        # Partition management
├── retention.go        # Retention policy enforcement
├── handler.go          # Custom slog.Handler implementation
└── maintenance.go      # Background maintenance tasks

gateway/logs/types.go
├── LogEntry            # Structured log entry
├── LogQuery            # Query builder
└── QueryResult         # Query result wrapper

cmd/cli/commands/logs/
├── query.go            # SQL query command
├── stats.go            # Statistics and aggregations
├── tail.go             # Real-time tail functionality
└── export.go           # Export logs (CSV, JSON)

tools/
└── query_logs_tool.go  # Agent tool for log queries
```

## Core Types

```go
// gateway/logs/types.go

package logs

import "time"

// LogEntry represents a structured log record
type LogEntry struct {
    ID        int64                  `json:"id"`
    Timestamp int64                  `json:"timestamp"`
    Time      string                 `json:"time"`
    Level     string                 `json:"level"`
    Component string                 `json:"component"`
    Message   string                 `json:"message"`
    Session   string                 `json:"session,omitempty"`
    RunID     string                 `json:"run_id,omitempty"`
    Error     string                 `json:"error,omitempty"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// LogQuery represents a log query
type LogQuery struct {
    // Time range
    Since  time.Duration
    After  time.Time
    Before time.Time

    // Filters
    Levels     []string
    Components []string
    Session    string
    RunID      string

    // Text search
    Match   string   // SQL LIKE pattern
    MatchFTS string  // Full-text search (if FTS enabled)

    // Limits
    Limit      int
    Offset     int
    Descending bool
}

// QueryResult wraps query results with metadata
type QueryResult struct {
    Entries       []*LogEntry       `json:"entries"`
    TotalMatched  int               `json:"total_matched"`
    QueryTime     time.Duration     `json:"query_time_ms"`
    PartitionsUsed []string         `json:"partitions_used"`
}

// PartitionInfo represents partition metadata
type PartitionInfo struct {
    Date         string
    DBFile       string
    IsCompressed bool
    RecordCount  int64
    FileSize     int64
    IsActive     bool
}

// RetentionPolicy defines log retention rules
type RetentionPolicy struct {
    Level         string
    RetentionDays int
}
```

## Implementation Plan

### Phase 1: Core Infrastructure (Day 1)

#### 1.1 Database Schema (`gateway/logs/schema.go`)
```go
package logs

const (
    // Schema version for migrations
    SchemaVersion = 1

    // SQL statements
    createLogsTableSQL = `CREATE TABLE IF NOT EXISTS logs (...)`
    createIndexesSQL   = `CREATE INDEX IF NOT EXISTS idx_timestamp ...`
    // ... all schema DDL
)

func CreateSchema(db *sql.DB) error {
    // Create tables and indexes
}

func MigrateSchema(db *sql.DB, fromVersion, toVersion int) error {
    // Handle schema migrations
}
```

#### 1.2 Log Writer (`gateway/logs/writer.go`)
```go
package logs

type Writer struct {
    db          *sql.DB           // Current partition DB
    indexDB     *sql.DB           // Index DB
    buffer      []*LogEntry       // Write buffer
    bufferSize  int               // Max buffer size (100)
    flushTicker *time.Ticker      // Flush interval (1s)
    mu          sync.Mutex        // Protects buffer
    wg          sync.WaitGroup    // For graceful shutdown
    stopChan    chan struct{}
}

func NewWriter(logsDir string) (*Writer, error) {
    // Initialize writer
    // Open current partition
    // Start flush ticker
}

func (w *Writer) Write(entry *LogEntry) error {
    // Buffer entry
    // Flush if full
}

func (w *Writer) flush() error {
    // Batch insert buffered entries
    // Update partition stats
}

func (w *Writer) Close() error {
    // Flush remaining buffer
    // Close databases
}
```

#### 1.3 Custom slog.Handler (`gateway/logs/handler.go`)
```go
package logs

type SQLiteHandler struct {
    writer  *Writer
    level   slog.Level
    attrs   []slog.Attr  // Accumulated attributes
    groups  []string     // Group stack
}

func NewSQLiteHandler(writer *Writer, level slog.Level) *SQLiteHandler {
    return &SQLiteHandler{
        writer: writer,
        level:  level,
    }
}

func (h *SQLiteHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return level >= h.level
}

func (h *SQLiteHandler) Handle(ctx context.Context, record slog.Record) error {
    // Extract fields from slog.Record
    entry := &LogEntry{
        Timestamp: record.Time.Unix(),
        Time:      record.Time.Format(time.RFC3339Nano),
        Level:     record.Level.String(),
        Message:   record.Message,
        Metadata:  make(map[string]interface{}),
    }

    // Extract attributes
    record.Attrs(func(attr slog.Attr) bool {
        switch attr.Key {
        case "component":
            entry.Component = attr.Value.String()
        case "session":
            entry.Session = attr.Value.String()
        case "run_id":
            entry.RunID = attr.Value.String()
        case "error":
            entry.Error = attr.Value.String()
        default:
            entry.Metadata[attr.Key] = attr.Value.Any()
        }
        return true
    })

    // Write to database
    return h.writer.Write(entry)
}

func (h *SQLiteHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    // Return new handler with additional attributes
}

func (h *SQLiteHandler) WithGroup(name string) slog.Handler {
    // Return new handler with group
}
```

### Phase 2: Partitioning & Retention (Day 2)

#### 2.1 Partition Management (`gateway/logs/partition.go`)
```go
package logs

type PartitionManager struct {
    logsDir  string
    indexDB  *sql.DB
    currentDB *sql.DB
    mu       sync.RWMutex
}

func NewPartitionManager(logsDir string) (*PartitionManager, error) {
    // Initialize partition manager
    // Load index.db
    // Open current partition
}

func (pm *PartitionManager) GetCurrentPartition() (*sql.DB, error) {
    // Return current active partition
}

func (pm *PartitionManager) RotatePartition() error {
    // Called at midnight
    // 1. Close current.db
    // 2. Rename to YYYY-MM-DD.db
    // 3. Create new current.db
    // 4. Update index.db
}

func (pm *PartitionManager) CompressOldPartitions() error {
    // Compress partitions older than 7 days
    // 1. Find uncompressed old partitions
    // 2. Gzip them
    // 3. Update index.db
}

func (pm *PartitionManager) QueryAcrossPartitions(query *LogQuery) (*QueryResult, error) {
    // 1. Determine which partitions to query based on time range
    // 2. Execute query on each partition
    // 3. Merge results
    // 4. Apply limit/offset
}
```

#### 2.2 Retention Policy (`gateway/logs/retention.go`)
```go
package logs

type RetentionEnforcer struct {
    indexDB *sql.DB
    logsDir string
}

func NewRetentionEnforcer(indexDB *sql.DB, logsDir string) *RetentionEnforcer {
    return &RetentionEnforcer{indexDB: indexDB, logsDir: logsDir}
}

func (re *RetentionEnforcer) EnforceRetention() error {
    // Get retention policies
    policies, err := re.getRetentionPolicies()
    if err != nil {
        return err
    }

    // For each level
    for level, retentionDays := range policies {
        cutoff := time.Now().AddDate(0, 0, -retentionDays)

        // Delete old records from each partition
        partitions, err := re.getPartitionsOlderThan(cutoff)
        if err != nil {
            return err
        }

        for _, partition := range partitions {
            if err := re.deleteOldLogs(partition, level, cutoff); err != nil {
                return err
            }
        }
    }

    // Vacuum databases to reclaim space
    return re.vacuumPartitions()
}

func (re *RetentionEnforcer) RunDaily() {
    // Schedule retention enforcement at 2 AM daily
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()

    for range ticker.C {
        if err := re.EnforceRetention(); err != nil {
            log.Error("Retention enforcement failed", "error", err)
        }
    }
}
```

### Phase 3: Query Interface (Day 3)

#### 3.1 Reader (`gateway/logs/reader.go`)
```go
package logs

type Reader struct {
    partitionMgr *PartitionManager
}

func NewReader(logsDir string) (*Reader, error) {
    pm, err := NewPartitionManager(logsDir)
    if err != nil {
        return nil, err
    }

    return &Reader{partitionMgr: pm}, nil
}

func (r *Reader) Query(query *LogQuery) (*QueryResult, error) {
    startTime := time.Now()

    // Determine partitions to query
    partitions := r.selectPartitions(query)

    // Build SQL
    sql, args := query.ToSQL()

    // Execute across partitions
    entries := []*LogEntry{}
    for _, partition := range partitions {
        partitionEntries, err := r.queryPartition(partition, sql, args)
        if err != nil {
            return nil, err
        }
        entries = append(entries, partitionEntries...)
    }

    // Sort if querying multiple partitions
    if len(partitions) > 1 {
        sort.Slice(entries, func(i, j int) bool {
            if query.Descending {
                return entries[i].Timestamp > entries[j].Timestamp
            }
            return entries[i].Timestamp < entries[j].Timestamp
        })
    }

    // Apply global limit/offset
    totalMatched := len(entries)
    if query.Offset > 0 {
        if query.Offset >= len(entries) {
            entries = []*LogEntry{}
        } else {
            entries = entries[query.Offset:]
        }
    }
    if query.Limit > 0 && len(entries) > query.Limit {
        entries = entries[:query.Limit]
    }

    return &QueryResult{
        Entries:        entries,
        TotalMatched:   totalMatched,
        QueryTime:      time.Since(startTime),
        PartitionsUsed: partitionNames(partitions),
    }, nil
}

func (r *Reader) Tail(follow bool, lastN int) (<-chan *LogEntry, error) {
    // Stream log entries
    // If follow=true, continue streaming new entries
}
```

#### 3.2 SQL Query Builder (`gateway/logs/query.go`)
```go
package logs

func (q *LogQuery) ToSQL() (string, []interface{}) {
    var conditions []string
    var args []interface{}

    // Time range
    if !q.After.IsZero() {
        conditions = append(conditions, "timestamp >= ?")
        args = append(args, q.After.Unix())
    }
    if !q.Before.IsZero() {
        conditions = append(conditions, "timestamp <= ?")
        args = append(args, q.Before.Unix())
    }
    if q.Since > 0 {
        conditions = append(conditions, "timestamp >= ?")
        args = append(args, time.Now().Add(-q.Since).Unix())
    }

    // Levels
    if len(q.Levels) > 0 {
        placeholders := make([]string, len(q.Levels))
        for i := range placeholders {
            placeholders[i] = "?"
        }
        conditions = append(conditions, "level IN ("+strings.Join(placeholders, ",")+")")
        for _, level := range q.Levels {
            args = append(args, level)
        }
    }

    // Components
    if len(q.Components) > 0 {
        placeholders := make([]string, len(q.Components))
        for i := range placeholders {
            placeholders[i] = "?"
        }
        conditions = append(conditions, "component IN ("+strings.Join(placeholders, ",")+")")
        for _, comp := range q.Components {
            args = append(args, comp)
        }
    }

    // Session
    if q.Session != "" {
        conditions = append(conditions, "session = ?")
        args = append(args, q.Session)
    }

    // RunID
    if q.RunID != "" {
        conditions = append(conditions, "run_id = ?")
        args = append(args, q.RunID)
    }

    // Text search
    if q.Match != "" {
        conditions = append(conditions, "(message LIKE ? OR error LIKE ?)")
        pattern := "%" + q.Match + "%"
        args = append(args, pattern, pattern)
    }

    // Build query
    query := "SELECT * FROM logs"
    if len(conditions) > 0 {
        query += " WHERE " + strings.Join(conditions, " AND ")
    }

    // Order
    if q.Descending {
        query += " ORDER BY timestamp DESC"
    } else {
        query += " ORDER BY timestamp ASC"
    }

    // Limit (per partition - global limit applied by reader)
    if q.Limit > 0 {
        query += " LIMIT ?"
        args = append(args, q.Limit)
    }

    return query, args
}
```

### Phase 4: CLI Integration (Day 4)

#### 4.1 Enhanced Logs Command
```bash
# Query logs with SQL
memdoor logs query "SELECT * FROM logs WHERE level = 'ERROR' LIMIT 10"

# Convenience commands
memdoor logs tail                         # Last 50 entries, follow mode
memdoor logs --since 5m --level error     # Last 5 min errors
memdoor logs --component WebSocket        # WebSocket logs only
memdoor logs --session session_main       # Session-specific logs

# Stats and aggregations
memdoor logs stats                        # Log statistics
memdoor logs stats --since 1h             # Last hour stats
memdoor logs errors                       # All errors, grouped by component

# Export
memdoor logs export --format csv --output logs.csv
memdoor logs export --format json --since 24h > logs.json
```

#### 4.2 Agent Tool
```json
{
    "name": "query_logs",
    "description": "Query gateway logs using SQL. Available tables: logs. Available columns: timestamp, time, level, component, message, session, run_id, error, metadata",
    "input_schema": {
        "type": "object",
        "properties": {
            "sql": {
                "type": "string",
                "description": "SQL SELECT query. Example: SELECT * FROM logs WHERE level = 'ERROR' AND timestamp >= 1709000000 LIMIT 10"
            }
        },
        "required": ["sql"]
    }
}
```

## Migration Strategy

### Step 1: Dual-Write Period (Week 1)
- Keep existing file-based logging
- Add SQLite logging in parallel
- Both systems active

### Step 2: Validation Period (Week 2)
- Compare file logs vs SQLite logs
- Verify query performance
- Fix any issues

### Step 3: Cutover (Week 3)
- Switch CLI to read from SQLite
- Keep file logs as backup for 7 days
- Remove file-based logging

### Migration Tool
```bash
# Import existing logs into SQLite
memdoor logs migrate --from ~/.memdoor/gateway.log --to ~/.memdoor/logs/

# Verify migration
memdoor logs verify
```

## Performance Targets

- **Write Latency**: < 1ms (buffered, async)
- **Query Latency**:
  - Simple queries (last 100 entries): < 10ms
  - Complex queries (1 hour range with filters): < 100ms
  - Cross-partition queries (24 hours): < 500ms
- **Storage Efficiency**:
  - ~500 bytes per log entry (uncompressed)
  - ~150 bytes per log entry (compressed)
  - ~1GB for 2M log entries (7 days of logs)
- **Write Throughput**: > 10,000 entries/sec

## Monitoring

### Health Checks
```bash
memdoor logs health
# Output:
# ✓ Current partition: 2026-02-25.db (142 MB, 285K entries)
# ✓ Total partitions: 32
# ✓ Oldest partition: 2026-01-25.db
# ✓ Compression: 25 partitions compressed
# ✓ Write buffer: 43/100 entries
# ✓ Last flush: 0.8s ago
# ✓ Retention enforcement: 12 hours ago
```

### Metrics to Track
- Partition count
- Total log entries
- Write buffer size
- Query performance (P50, P95, P99)
- Disk usage
- Compression ratio

## Testing Strategy

### Unit Tests
- Schema creation
- Log entry parsing
- Query builder
- Partition rotation
- Retention enforcement

### Integration Tests
- End-to-end write → query
- Cross-partition queries
- Compression and decompression
- Concurrent writes
- Migration from file logs

### Performance Tests
- Write throughput benchmark
- Query latency benchmark
- Large dataset queries (millions of entries)
- Concurrent query performance

### Agent Tests
- Agent writes valid SQL queries
- Agent interprets results correctly
- Agent can debug real issues

## Open Questions

1. **Full-text search**: Enable FTS5 for message/error text? Adds overhead but enables better search.
2. **Replication**: Support for log replication to S3/GCS for long-term storage?
3. **Metrics integration**: Export logs to Prometheus/Grafana?
4. **Alerting**: Real-time alerts on ERROR logs?
5. **Sampling**: Sample DEBUG logs (e.g., 10%) to reduce volume?

## Next Steps

1. Review and approve design
2. Create implementation plan with milestones
3. Set up development branch
4. Implement Phase 1 (Core Infrastructure)
5. Write tests for Phase 1
6. Continue with subsequent phases

---

**Estimated Implementation Time**: 4 days (32 hours)
- Phase 1: 1 day (schema, writer, handler)
- Phase 2: 1 day (partitioning, retention)
- Phase 3: 1 day (reader, queries)
- Phase 4: 1 day (CLI integration, agent tool)

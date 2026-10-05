package logs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memdoor/pkg/sqlitedriver"
)

// logsDriverName installs REGEXP on every connection the pool opens
// (pkg/sqlitedriver, for builds with and without cgo).
const logsDriverName = sqlitedriver.LogsDriverName

// SQLiteStorage implements Storage using SQLite
type SQLiteStorage struct {
	db   *sql.DB
	path string
}

// NewSQLiteStorage creates a new SQLite storage backend
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create logs directory: %w", err)
	}

	// Open database with regex support. The REGEXP function MUST be installed
	// on EVERY pooled connection via the driver's ConnectHook — registering it
	// on a single db.Conn (the previous code) left every OTHER connection the
	// pool opens without it, so regex queries randomly failed with "no such
	// function: REGEXP" or returned empty exactly when the gateway was busy
	// (heartbeat + coder + queries growing the pool). A week of phantom
	// "(0 events)" traced back to this.
	dsn := fmt.Sprintf("file:%s?_regexp=1", dbPath)
	db, err := sql.Open(logsDriverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Configure for performance
	if _, err := db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
		PRAGMA cache_size = -64000;
		PRAGMA temp_store = MEMORY;
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure database: %w", err)
	}

	storage := &SQLiteStorage{
		db:   db,
		path: dbPath,
	}

	// Create schema
	if err := storage.createSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return storage, nil
}

// createSchema creates the database schema
func (s *SQLiteStorage) createSchema() error {
	schema := `
	-- Events table
	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		timestamp INTEGER NOT NULL,
		time TEXT NOT NULL,
		type TEXT NOT NULL,
		component TEXT NOT NULL,
		level TEXT NOT NULL CHECK(level IN ('DEBUG', 'INFO', 'WARN', 'ERROR')),

		-- Context
		session TEXT,
		run_id TEXT,
		span_id TEXT,

		-- Causality
		parent_id TEXT,
		root_id TEXT,

		-- Content
		message TEXT NOT NULL,
		data TEXT, -- JSON
		error_type TEXT,
		error_message TEXT,
		error_stack TEXT,
		error_retryable INTEGER,

		-- Performance
		duration_ns INTEGER,

		-- Outcome
		success INTEGER NOT NULL DEFAULT 1,

		-- Timestamps for queries
		CHECK(timestamp > 0),
		CHECK(success IN (0, 1))
	);

	-- Indexes for fast queries
	CREATE INDEX IF NOT EXISTS idx_timestamp ON events(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_level ON events(level);
	CREATE INDEX IF NOT EXISTS idx_component ON events(component);
	CREATE INDEX IF NOT EXISTS idx_type ON events(type);
	CREATE INDEX IF NOT EXISTS idx_session ON events(session) WHERE session IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_run_id ON events(run_id) WHERE run_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_span_id ON events(span_id) WHERE span_id IS NOT NULL;

	-- Causality indexes
	CREATE INDEX IF NOT EXISTS idx_parent_id ON events(parent_id) WHERE parent_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_root_id ON events(root_id) WHERE root_id IS NOT NULL;

	-- Composite indexes for common query patterns
	CREATE INDEX IF NOT EXISTS idx_time_level ON events(timestamp DESC, level);
	CREATE INDEX IF NOT EXISTS idx_time_component ON events(timestamp DESC, component);
	CREATE INDEX IF NOT EXISTS idx_session_time ON events(session, timestamp DESC) WHERE session IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_success ON events(success, timestamp DESC);
	`

	_, err := s.db.Exec(schema)
	return err
}

// WriteEvents writes a batch of events to storage
func (s *SQLiteStorage) WriteEvents(ctx context.Context, events []*Event) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO events (
			id, timestamp, time, type, component, level,
			session, run_id, span_id, parent_id, root_id,
			message, data, error_type, error_message, error_stack, error_retryable,
			duration_ns, success
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, event := range events {
		// Serialize data as JSON
		var dataJSON []byte
		if len(event.Data) > 0 {
			dataJSON, err = json.Marshal(event.Data)
			if err != nil {
				return fmt.Errorf("marshal data: %w", err)
			}
		}

		// Extract error fields
		var errorType, errorMessage, errorStack sql.NullString
		var errorRetryable sql.NullInt64
		if event.Error != nil {
			errorType = sql.NullString{String: event.Error.Type, Valid: true}
			errorMessage = sql.NullString{String: event.Error.Message, Valid: true}
			if event.Error.Stack != "" {
				errorStack = sql.NullString{String: event.Error.Stack, Valid: true}
			}
			errorRetryable = sql.NullInt64{Int64: boolToInt64(event.Error.Retryable), Valid: true}
		}

		// Convert duration to nanoseconds
		var durationNS sql.NullInt64
		if event.Duration > 0 {
			durationNS = sql.NullInt64{Int64: event.Duration.Nanoseconds(), Valid: true}
		}

		// Convert nullable strings
		session := toNullString(event.Session)
		runID := toNullString(event.RunID)
		spanID := toNullString(event.SpanID)
		parentID := toNullString(event.ParentID)
		rootID := toNullString(event.RootID)

		_, err = stmt.ExecContext(ctx,
			event.ID,
			event.Timestamp.Unix(),
			event.Timestamp.Format(time.RFC3339Nano),
			event.Type,
			event.Component,
			event.Level,
			session,
			runID,
			spanID,
			parentID,
			rootID,
			event.Message,
			dataJSON,
			errorType,
			errorMessage,
			errorStack,
			errorRetryable,
			durationNS,
			boolToInt64(event.Success),
		)
		if err != nil {
			return fmt.Errorf("insert event %s: %w", event.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// QueryEvents retrieves events based on query criteria
func (s *SQLiteStorage) QueryEvents(ctx context.Context, query *Query) (*QueryResult, error) {
	startTime := time.Now()

	// Build SQL query
	sql, args := s.buildQuery(query)

	// Execute query
	rows, err := s.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	// Parse results
	events := make([]*Event, 0)
	for rows.Next() {
		event, err := s.scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return &QueryResult{
		Events:       events,
		TotalMatched: len(events),
		QueryTime:    time.Since(startTime),
	}, nil
}

// buildQuery constructs SQL and arguments from Query
func (s *SQLiteStorage) buildQuery(query *Query) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	// Time range
	if !query.After.IsZero() {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, query.After.Unix())
	}
	if !query.Before.IsZero() {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, query.Before.Unix())
	}
	if query.Since > 0 {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, time.Now().Add(-query.Since).Unix())
	}

	// Levels
	if len(query.Levels) > 0 {
		placeholders := make([]string, len(query.Levels))
		for i, level := range query.Levels {
			placeholders[i] = "?"
			args = append(args, level)
		}
		conditions = append(conditions, fmt.Sprintf("level IN (%s)", strings.Join(placeholders, ",")))
	}

	// Components
	if len(query.Components) > 0 {
		placeholders := make([]string, len(query.Components))
		for i, comp := range query.Components {
			placeholders[i] = "?"
			args = append(args, comp)
		}
		conditions = append(conditions, fmt.Sprintf("component IN (%s)", strings.Join(placeholders, ",")))
	}

	// Event types
	if len(query.EventTypes) > 0 {
		placeholders := make([]string, len(query.EventTypes))
		for i, typ := range query.EventTypes {
			placeholders[i] = "?"
			args = append(args, typ)
		}
		conditions = append(conditions, fmt.Sprintf("type IN (%s)", strings.Join(placeholders, ",")))
	}

	// Session
	if query.Session != "" {
		conditions = append(conditions, "session = ?")
		args = append(args, query.Session)
	}

	// RunID
	if query.RunID != "" {
		conditions = append(conditions, "run_id = ?")
		args = append(args, query.RunID)
	}

	// SpanID
	if query.SpanID != "" {
		conditions = append(conditions, "span_id = ?")
		args = append(args, query.SpanID)
	}

	// Causality
	if query.ParentID != "" {
		conditions = append(conditions, "parent_id = ?")
		args = append(args, query.ParentID)
	}
	if query.RootID != "" {
		conditions = append(conditions, "root_id = ?")
		args = append(args, query.RootID)
	}

	// Text search
	if query.MessageContains != "" {
		conditions = append(conditions, "message LIKE ?")
		args = append(args, "%"+query.MessageContains+"%")
	}
	if query.MessageRegex != "" {
		conditions = append(conditions, "message REGEXP ?")
		args = append(args, query.MessageRegex)
	}
	if query.ErrorContains != "" {
		conditions = append(conditions, "error_message LIKE ?")
		args = append(args, "%"+query.ErrorContains+"%")
	}

	// Workspace filter — projects on the data JSON column where most
	// event writers attach `workspace` via slog.String("workspace", ws).
	// SQLite's json_extract returns NULL for events without that key,
	// which compares != to a non-empty string and is correctly excluded.
	if query.Workspace != "" {
		conditions = append(conditions, "json_extract(data, '$.workspace') = ?")
		args = append(args, query.Workspace)
	}

	// Success filter
	if query.SuccessOnly != nil {
		if *query.SuccessOnly {
			conditions = append(conditions, "success = 1")
		} else {
			conditions = append(conditions, "success = 0")
		}
	}

	// Build query
	sql := "SELECT * FROM events"
	if len(conditions) > 0 {
		sql += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Order (use ID as secondary sort for deterministic ordering)
	if query.Descending {
		sql += " ORDER BY timestamp DESC, id DESC"
	} else {
		sql += " ORDER BY timestamp ASC, id ASC"
	}

	// Limit and offset
	if query.Limit > 0 {
		sql += " LIMIT ?"
		args = append(args, query.Limit)
	}
	if query.Offset > 0 {
		sql += " OFFSET ?"
		args = append(args, query.Offset)
	}

	return sql, args
}

// scanEvent scans a database row into an Event
func (s *SQLiteStorage) scanEvent(rows *sql.Rows) (*Event, error) {
	var (
		id, timeStr, typ, component, level, message string
		timestamp, success                          int64
		session, runID, spanID, parentID, rootID    sql.NullString
		dataJSON                                    []byte
		errorType, errorMessage, errorStack         sql.NullString
		errorRetryable                              sql.NullInt64
		durationNS                                  sql.NullInt64
	)

	err := rows.Scan(
		&id, &timestamp, &timeStr, &typ, &component, &level,
		&session, &runID, &spanID, &parentID, &rootID,
		&message, &dataJSON, &errorType, &errorMessage, &errorStack, &errorRetryable,
		&durationNS, &success,
	)
	if err != nil {
		return nil, err
	}

	event := &Event{
		ID:        id,
		Timestamp: time.Unix(timestamp, 0),
		Type:      EventType(typ),
		Component: component,
		Level:     Level(level),
		Message:   message,
		Success:   success == 1,
	}

	// Parse nullable fields
	if session.Valid {
		event.Session = session.String
	}
	if runID.Valid {
		event.RunID = runID.String
	}
	if spanID.Valid {
		event.SpanID = spanID.String
	}
	if parentID.Valid {
		event.ParentID = parentID.String
	}
	if rootID.Valid {
		event.RootID = rootID.String
	}

	// Parse JSON data
	if len(dataJSON) > 0 {
		if err := json.Unmarshal(dataJSON, &event.Data); err != nil {
			return nil, fmt.Errorf("unmarshal data: %w", err)
		}
	}

	// Parse error
	if errorType.Valid {
		event.Error = &ErrorInfo{
			Type:    errorType.String,
			Message: errorMessage.String,
		}
		if errorStack.Valid {
			event.Error.Stack = errorStack.String
		}
		if errorRetryable.Valid {
			event.Error.Retryable = errorRetryable.Int64 == 1
		}
	}

	// Parse duration
	if durationNS.Valid {
		event.Duration = time.Duration(durationNS.Int64)
	}

	return event, nil
}

// TraceChain reconstructs the causal chain for an event
func (s *SQLiteStorage) TraceChain(ctx context.Context, eventID string) (*CausalChain, error) {
	// First, get the event and find its root
	event, err := s.getEventByID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}

	// Determine root ID
	rootID := event.RootID
	if rootID == "" {
		rootID = event.ID // This event is the root
	}

	// Get root event
	rootEvent, err := s.getEventByID(ctx, rootID)
	if err != nil {
		return nil, fmt.Errorf("get root event: %w", err)
	}

	// Get all events in the chain (events with root_id matching)
	query := &Query{
		RootID:     rootID,
		Descending: false, // Chronological order
	}
	result, err := s.QueryEvents(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query chain: %w", err)
	}

	// Build complete chain (include root event if not already in results)
	chain := []*Event{rootEvent}
	for _, e := range result.Events {
		if e.ID != rootID {
			chain = append(chain, e)
		}
	}

	// Calculate depth
	depth := 0
	for _, e := range chain {
		d := s.calculateEventDepth(e, chain)
		if d > depth {
			depth = d
		}
	}

	return &CausalChain{
		RootEvent:   rootEvent,
		Chain:       chain,
		TotalEvents: len(chain),
		Depth:       depth,
	}, nil
}

// calculateEventDepth calculates the depth of an event in the chain
func (s *SQLiteStorage) calculateEventDepth(event *Event, chain []*Event) int {
	if event.ParentID == "" {
		return 0
	}

	// Find parent
	for _, e := range chain {
		if e.ID == event.ParentID {
			return 1 + s.calculateEventDepth(e, chain)
		}
	}

	return 0
}

// getEventByID retrieves a single event by ID
func (s *SQLiteStorage) getEventByID(ctx context.Context, eventID string) (*Event, error) {
	row := s.db.QueryRowContext(ctx, "SELECT * FROM events WHERE id = ?", eventID)
	return s.scanEventFromRow(row)
}

// scanEventFromRow scans a single row into an Event
func (s *SQLiteStorage) scanEventFromRow(row *sql.Row) (*Event, error) {
	var (
		id, timeStr, typ, component, level, message string
		timestamp, success                          int64
		session, runID, spanID, parentID, rootID    sql.NullString
		dataJSON                                    []byte
		errorType, errorMessage, errorStack         sql.NullString
		errorRetryable                              sql.NullInt64
		durationNS                                  sql.NullInt64
	)

	err := row.Scan(
		&id, &timestamp, &timeStr, &typ, &component, &level,
		&session, &runID, &spanID, &parentID, &rootID,
		&message, &dataJSON, &errorType, &errorMessage, &errorStack, &errorRetryable,
		&durationNS, &success,
	)
	if err != nil {
		return nil, err
	}

	event := &Event{
		ID:        id,
		Timestamp: time.Unix(timestamp, 0),
		Type:      EventType(typ),
		Component: component,
		Level:     Level(level),
		Message:   message,
		Success:   success == 1,
	}

	if session.Valid {
		event.Session = session.String
	}
	if runID.Valid {
		event.RunID = runID.String
	}
	if spanID.Valid {
		event.SpanID = spanID.String
	}
	if parentID.Valid {
		event.ParentID = parentID.String
	}
	if rootID.Valid {
		event.RootID = rootID.String
	}

	if len(dataJSON) > 0 {
		if err := json.Unmarshal(dataJSON, &event.Data); err != nil {
			return nil, fmt.Errorf("unmarshal data: %w", err)
		}
	}

	if errorType.Valid {
		event.Error = &ErrorInfo{
			Type:    errorType.String,
			Message: errorMessage.String,
		}
		if errorStack.Valid {
			event.Error.Stack = errorStack.String
		}
		if errorRetryable.Valid {
			event.Error.Retryable = errorRetryable.Int64 == 1
		}
	}

	if durationNS.Valid {
		event.Duration = time.Duration(durationNS.Int64)
	}

	return event, nil
}

// ReconstructSession rebuilds the timeline for a session
func (s *SQLiteStorage) ReconstructSession(ctx context.Context, sessionID string) ([]*Event, error) {
	query := &Query{
		Session:    sessionID,
		Descending: false, // Chronological order
	}

	result, err := s.QueryEvents(ctx, query)
	if err != nil {
		return nil, err
	}

	return result.Events, nil
}

// FindSimilar finds events with similar patterns
func (s *SQLiteStorage) FindSimilar(ctx context.Context, eventID string, limit int) ([]*Event, error) {
	// Get the source event
	event, err := s.getEventByID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}

	// Find similar events (same type, component, level)
	query := &Query{
		EventTypes: []EventType{event.Type},
		Components: []string{event.Component},
		Levels:     []Level{event.Level},
		Limit:      limit,
		Descending: true,
	}

	result, err := s.QueryEvents(ctx, query)
	if err != nil {
		return nil, err
	}

	// Filter out the original event
	similar := make([]*Event, 0, len(result.Events))
	for _, e := range result.Events {
		if e.ID != eventID {
			similar = append(similar, e)
		}
	}

	return similar, nil
}

// GetStats returns storage statistics
func (s *SQLiteStorage) GetStats(ctx context.Context) (*StorageStats, error) {
	stats := &StorageStats{
		EventsByLevel: make(map[Level]int64),
	}

	// Total events
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&stats.TotalEvents); err != nil {
		return nil, fmt.Errorf("count events: %w", err)
	}

	// Events by level
	rows, err := s.db.QueryContext(ctx, "SELECT level, COUNT(*) FROM events GROUP BY level")
	if err != nil {
		return nil, fmt.Errorf("group by level: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var level string
		var count int64
		if err := rows.Scan(&level, &count); err != nil {
			return nil, err
		}
		stats.EventsByLevel[Level(level)] = count
	}

	// Oldest and newest events
	var oldestTS, newestTS sql.NullInt64
	if err := s.db.QueryRowContext(ctx, "SELECT MIN(timestamp), MAX(timestamp) FROM events").Scan(&oldestTS, &newestTS); err != nil {
		return nil, fmt.Errorf("get time range: %w", err)
	}
	if oldestTS.Valid {
		stats.OldestEvent = time.Unix(oldestTS.Int64, 0)
	}
	if newestTS.Valid {
		stats.NewestEvent = time.Unix(newestTS.Int64, 0)
	}

	// Database size
	fileInfo, err := os.Stat(s.path)
	if err == nil {
		stats.DatabaseSize = fileInfo.Size()
	}

	return stats, nil
}

// Close closes the storage backend
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// PruneBefore deletes events older than cutoff (timestamps are stored as
// unix seconds) and returns the number removed. After a non-empty delete
// it runs VACUUM to return the freed pages to the OS — best-effort, since
// VACUUM takes a write lock and can't run in a transaction; a transient
// lock must not fail the prune (the rows are already gone and space
// reclaims on the next VACUUM/retention tick).
func (s *SQLiteStorage) PruneBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM events WHERE timestamp < ?", cutoff.Unix())
	if err != nil {
		return 0, fmt.Errorf("prune events: %w", err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		_, _ = s.db.ExecContext(ctx, "VACUUM")
	}
	return n, nil
}

// Helper functions

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

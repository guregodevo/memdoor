package logs

import (
	"context"
	"time"
)

// Storage defines the interface for log event persistence
type Storage interface {
	// WriteEvents writes a batch of events to storage
	WriteEvents(ctx context.Context, events []*Event) error

	// QueryEvents retrieves events based on query criteria
	QueryEvents(ctx context.Context, query *Query) (*QueryResult, error)

	// TraceChain reconstructs the causal chain for an event
	TraceChain(ctx context.Context, eventID string) (*CausalChain, error)

	// ReconstructSession rebuilds the timeline for a session
	ReconstructSession(ctx context.Context, sessionID string) ([]*Event, error)

	// FindSimilar finds events with similar patterns
	FindSimilar(ctx context.Context, eventID string, limit int) ([]*Event, error)

	// GetStats returns storage statistics
	GetStats(ctx context.Context) (*StorageStats, error)

	// PruneBefore deletes events older than cutoff and reclaims freed
	// disk space (VACUUM). Returns the number of events deleted. Backs
	// both `memdoor logs prune` and the automatic retention loop.
	PruneBefore(ctx context.Context, cutoff time.Time) (int64, error)

	// Close closes the storage backend
	Close() error
}

// Query represents search criteria for events
type Query struct {
	// Time range
	After  time.Time
	Before time.Time
	Since  time.Duration

	// Filters
	Levels     []Level
	Components []string
	EventTypes []EventType
	Session    string
	RunID      string
	SpanID     string

	// Causality
	ParentID string // Find children of this event
	RootID   string // Find all events in this causal chain

	// Text search
	MessageContains string
	MessageRegex    string // Regex search on message field
	ErrorContains   string

	// Workspace filter — matches events whose Data["workspace"] equals
	// the given slug. Many event writers attach the workspace via
	// slog.String("workspace", ws) which serializes into the data
	// JSON column; this filter projects on that field via
	// json_extract(data, '$.workspace'). No schema migration needed —
	// the column already exists. Empty string disables the filter.
	Workspace string

	// Success filter
	SuccessOnly *bool // nil = all, true = success only, false = failures only

	// Pagination
	Limit      int
	Offset     int
	Descending bool
}

// QueryResult wraps query results with metadata
type QueryResult struct {
	Events       []*Event      `json:"events"`
	TotalMatched int           `json:"total_matched"`
	QueryTime    time.Duration `json:"query_time"`
}

// CausalChain represents the cause-effect chain of events
type CausalChain struct {
	RootEvent   *Event   `json:"root_event"`   // The original trigger
	Chain       []*Event `json:"chain"`        // Events in causal order
	TotalEvents int      `json:"total_events"` // Total events in chain
	Depth       int      `json:"depth"`        // Maximum depth
}

// StorageStats provides storage metrics
type StorageStats struct {
	TotalEvents   int64           `json:"total_events"`
	EventsByLevel map[Level]int64 `json:"events_by_level"`
	OldestEvent   time.Time       `json:"oldest_event"`
	NewestEvent   time.Time       `json:"newest_event"`
	DatabaseSize  int64           `json:"database_size_bytes"`
	IndexSize     int64           `json:"index_size_bytes"`
}

// QueryBuilder provides a fluent interface for building queries
type QueryBuilder struct {
	query Query
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		query: Query{
			Limit:      100,
			Descending: true,
		},
	}
}

// Since filters events from the last duration
func (qb *QueryBuilder) Since(d time.Duration) *QueryBuilder {
	qb.query.Since = d
	return qb
}

// Levels filters by log levels
func (qb *QueryBuilder) Levels(levels ...Level) *QueryBuilder {
	qb.query.Levels = levels
	return qb
}

// Components filters by components
func (qb *QueryBuilder) Components(components ...string) *QueryBuilder {
	qb.query.Components = components
	return qb
}

// Session filters by session ID
func (qb *QueryBuilder) Session(sessionID string) *QueryBuilder {
	qb.query.Session = sessionID
	return qb
}

// RunID filters by run ID
func (qb *QueryBuilder) RunID(runID string) *QueryBuilder {
	qb.query.RunID = runID
	return qb
}

// MessageRegex filters by message using regex pattern
func (qb *QueryBuilder) MessageRegex(pattern string) *QueryBuilder {
	qb.query.MessageRegex = pattern
	return qb
}

// Workspace filters events whose Data["workspace"] matches the given slug.
// Used by per-workspace dashboards and by the agents' logs_query tool to show
// only events relevant to the current workspace.
func (qb *QueryBuilder) Workspace(slug string) *QueryBuilder {
	qb.query.Workspace = slug
	return qb
}

// ErrorsOnly filters to show only failed events
func (qb *QueryBuilder) ErrorsOnly() *QueryBuilder {
	f := false
	qb.query.SuccessOnly = &f
	return qb
}

// Limit sets the maximum number of results
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.query.Limit = limit
	return qb
}

// Build returns the constructed query
func (qb *QueryBuilder) Build() *Query {
	return &qb.query
}

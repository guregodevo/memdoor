package health

// HealthSummary represents comprehensive gateway health information
type HealthSummary struct {
	// Convenience flag - always true if RPC succeeded
	OK bool `json:"ok"`
	// Timestamp when snapshot was taken
	Timestamp int64 `json:"ts"`
	// Duration to collect this snapshot
	DurationMs int64 `json:"durationMs"`
	// Cached indicates if this response was served from cache
	Cached bool `json:"cached,omitempty"`

	// Gateway information
	Gateway GatewayHealth `json:"gateway"`

	// Default agent ID
	DefaultAgentID string `json:"defaultAgentId"`

	// Agent health summaries
	Agents []AgentHealthSummary `json:"agents"`

	// Session information
	Sessions SessionsHealth `json:"sessions"`

	// LLM inference availability. Lets a client tell whether this gateway can
	// actually answer, instead of hanging on a request that will never
	// produce a reply.
	LLM *LLMHealth `json:"llm,omitempty"`
}

// LLMHealth reports whether a model can answer on this gateway.
type LLMHealth struct {
	Available bool   `json:"available"`
	Hint      string `json:"hint,omitempty"`
}

// GatewayHealth represents gateway server health
type GatewayHealth struct {
	Version     string `json:"version"`
	Port        int    `json:"port"`
	Uptime      int64  `json:"uptime"`      // seconds
	StartTime   int64  `json:"startTime"`   // unix timestamp
	ActiveConns int    `json:"activeConns"` // active WebSocket connections
	TotalConns  int64  `json:"totalConns"`  // total connections since start
	MemoryUsage uint64 `json:"memoryUsage"` // bytes
	Goroutines  int    `json:"goroutines"`
	ToolsCount  int    `json:"toolsCount"`  // number of registered tools
	AgentsCount int    `json:"agentsCount"` // number of configured agents
}

// AgentHealthSummary represents health for a single agent
type AgentHealthSummary struct {
	AgentID   string         `json:"agentId"`
	Name      string         `json:"name,omitempty"`
	IsDefault bool           `json:"isDefault"`
	Sessions  SessionsHealth `json:"sessions"`
}

// heartbeat interval in milliseconds
// last heartbeat timestamp
// next expected heartbeat

// SessionsHealth represents session store health
type SessionsHealth struct {
	Path   string          `json:"path"`
	Count  int             `json:"count"`
	Recent []RecentSession `json:"recent"`
}

// RecentSession represents a recently updated session
type RecentSession struct {
	Key       string `json:"key"`
	UpdatedAt *int64 `json:"updatedAt"`
	Age       *int64 `json:"age"` // milliseconds since last update
}

// SnapshotOptions configures health snapshot generation
type SnapshotOptions struct {
	// Probe enables deep inspection (channel probes, etc.)
	Probe bool
	// IncludeCache allows returning cached snapshots
	IncludeCache bool
	// CacheMaxAge is max age in milliseconds for cache validity
	CacheMaxAge int64
}

// DefaultSnapshotOptions returns default options
func DefaultSnapshotOptions() SnapshotOptions {
	return SnapshotOptions{
		Probe:        false,
		IncludeCache: true,
		CacheMaxAge:  15000, // 15 seconds
	}
}

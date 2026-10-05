package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"memdoor/gateway/logs"
)

// AgentLogTool allows agents to query structured logs via the global logger storage
type AgentLogTool struct {
	storage logs.Storage
	agentID string
}

// AgentLogParams defines the parameters for the agent_log tool
type AgentLogParams struct {
	Operation *string `json:"operation,omitempty" jsonschema_description:"Operation to perform: query (default), trace, session, stats"`

	// Query filters (for operation=query)
	Session   *string   `json:"session,omitempty" jsonschema_description:"Filter logs by session ID"`
	Component *string   `json:"component,omitempty" jsonschema_description:"Filter by component (e.g., Agent, WebSocket, Queue, A2A)"`
	Level     *string   `json:"level,omitempty" jsonschema_description:"Filter by log level: DEBUG, INFO, WARN, ERROR"`
	Levels    *[]string `json:"levels,omitempty" jsonschema_description:"Filter by multiple levels (e.g., ['ERROR', 'WARN'])"`
	Since     *string   `json:"since,omitempty" jsonschema_description:"Filter logs since duration ago (e.g., '5m', '1h', '24h', '7d')"`
	Regex     *string   `json:"regex,omitempty" jsonschema_description:"Regex pattern to search in messages (Go RE2 syntax)"`
	Limit     *int      `json:"limit,omitempty" jsonschema_description:"Maximum number of results (default: 50, max: 500)"`

	// For operation=trace
	EventID *string `json:"event_id,omitempty" jsonschema_description:"Event ID to trace causality chain (for operation=trace)"`

	// For operation=session
	SessionID *string `json:"session_id,omitempty" jsonschema_description:"Session ID to reconstruct timeline (for operation=session)"`
}

// AgentLogResult is the result returned by the agent_log tool
type AgentLogResult struct {
	Operation    string             `json:"operation"`
	Events       []*logs.Event      `json:"events,omitempty"`
	Count        int                `json:"count"`
	CausalChain  *logs.CausalChain  `json:"causal_chain,omitempty"`
	Stats        *logs.StorageStats `json:"stats,omitempty"`
	QueryTime    string             `json:"query_time,omitempty"`
	ErrorMessage string             `json:"error,omitempty"`
}

// NewAgentLogTool creates an agent_log tool using the global logger's storage
func NewAgentLogTool(agentID string) (*AgentLogTool, error) {
	storage := logs.GetGlobalStorage()
	if storage == nil {
		return nil, fmt.Errorf("log storage not available (gateway may not be running)")
	}

	return &AgentLogTool{
		storage: storage,
		agentID: agentID,
	}, nil
}

// Name returns the tool name
func (t *AgentLogTool) Name() string {
	return "agent_log"
}

// Description returns the tool description
func (t *AgentLogTool) Description() string {
	return `Query structured logs using SQLite-backed event storage with causality tracking.

This tool provides agents with powerful self-debugging capabilities:

**Operations:**
- query (default): Search events with filters (session, component, level, time, regex)
- trace: Reconstruct causal chain from root cause to effects
- session: Replay complete session timeline
- stats: Get log statistics (total events, levels, time range)

**Common use cases:**
- Debug errors: {"operation": "query", "level": "ERROR", "since": "1h"}
- Session replay: {"operation": "session", "session_id": "session_main"}
- Root cause: {"operation": "trace", "event_id": "<event-id>"}
- Component logs: {"operation": "query", "component": "Agent", "since": "5m"}
- Pattern search: {"operation": "query", "regex": "timeout|error", "limit": 20}

**Performance:**
- Indexed queries: <1ms for session, component, level filters
- Regex search: ~10-50ms depending on database size
- Limit: Default 50 events, max 500 to prevent excessive output

**Regex syntax:** Go RE2 (no backreferences, no lookahead/lookbehind)
- Case-insensitive: Use (?i) flag, e.g., "(?i)error"
- Alternation: "timeout|connection.*refused"
- Character classes: "[Ee]rror|WARN"`
}

// Parameters returns the JSON schema for the tool parameters
func (t *AgentLogTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": "Operation: query (default), trace, session, stats",
				"enum":        []string{"query", "trace", "session", "stats"},
			},
			"session": map[string]interface{}{
				"type":        "string",
				"description": "Filter logs by session ID (for query operation)",
			},
			"component": map[string]interface{}{
				"type":        "string",
				"description": "Filter by component: Agent, WebSocket, Queue, A2A, etc.",
			},
			"level": map[string]interface{}{
				"type":        "string",
				"description": "Filter by single log level",
				"enum":        []string{"DEBUG", "INFO", "WARN", "ERROR"},
			},
			"levels": map[string]interface{}{
				"type":        "array",
				"description": "Filter by multiple levels (e.g., ['ERROR', 'WARN'])",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []string{"DEBUG", "INFO", "WARN", "ERROR"},
				},
			},
			"since": map[string]interface{}{
				"type":        "string",
				"description": "Filter logs since duration: '5m', '1h', '24h', '7d'",
			},
			"regex": map[string]interface{}{
				"type":        "string",
				"description": "Regex pattern for message search (Go RE2 syntax)",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum results (default: 50, max: 500)",
				"default":     50,
			},
			"event_id": map[string]interface{}{
				"type":        "string",
				"description": "Event ID to trace (for operation=trace)",
			},
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Session ID to reconstruct (for operation=session)",
			},
		},
	}
}

// Execute runs the agent_log tool
func (t *AgentLogTool) Execute(paramsJSON string) (string, error) {
	var params AgentLogParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	// Default operation is query
	operation := "query"
	if params.Operation != nil {
		operation = *params.Operation
	}

	ctx := context.Background()
	startTime := time.Now()

	var result AgentLogResult
	result.Operation = operation

	switch operation {
	case "query":
		events, err := t.executeQuery(ctx, params)
		if err != nil {
			result.ErrorMessage = err.Error()
		} else {
			result.Events = events
			result.Count = len(events)
		}

	case "trace":
		if params.EventID == nil || *params.EventID == "" {
			return "", fmt.Errorf("event_id required for trace operation")
		}
		chain, err := t.storage.TraceChain(ctx, *params.EventID)
		if err != nil {
			result.ErrorMessage = err.Error()
		} else {
			result.CausalChain = chain
			result.Count = chain.TotalEvents
		}

	case "session":
		if params.SessionID == nil || *params.SessionID == "" {
			return "", fmt.Errorf("session_id required for session operation")
		}
		events, err := t.storage.ReconstructSession(ctx, *params.SessionID)
		if err != nil {
			result.ErrorMessage = err.Error()
		} else {
			result.Events = events
			result.Count = len(events)
		}

	case "stats":
		stats, err := t.storage.GetStats(ctx)
		if err != nil {
			result.ErrorMessage = err.Error()
		} else {
			result.Stats = stats
		}

	default:
		return "", fmt.Errorf("unknown operation: %s (use: query, trace, session, stats)", operation)
	}

	result.QueryTime = time.Since(startTime).String()

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// executeQuery performs a query operation
func (t *AgentLogTool) executeQuery(ctx context.Context, params AgentLogParams) ([]*logs.Event, error) {
	qb := logs.NewQueryBuilder()

	// Set limit (default 50, max 500)
	limit := 50
	if params.Limit != nil {
		limit = *params.Limit
		if limit > 500 {
			limit = 500
		}
	}
	qb = qb.Limit(limit)

	// Time filter
	if params.Since != nil {
		duration, err := time.ParseDuration(*params.Since)
		if err != nil {
			return nil, fmt.Errorf("invalid since duration '%s': %w", *params.Since, err)
		}
		qb = qb.Since(duration)
	}

	// Level filters
	if params.Level != nil {
		qb = qb.Levels(logs.Level(*params.Level))
	} else if params.Levels != nil && len(*params.Levels) > 0 {
		levels := make([]logs.Level, len(*params.Levels))
		for i, l := range *params.Levels {
			levels[i] = logs.Level(l)
		}
		qb = qb.Levels(levels...)
	}

	// Component filter
	if params.Component != nil {
		qb = qb.Components(*params.Component)
	}

	// Session filter
	if params.Session != nil {
		qb = qb.Session(*params.Session)
	}

	// Regex filter
	if params.Regex != nil {
		qb = qb.MessageRegex(*params.Regex)
	}

	// Execute query
	query := qb.Build()
	queryResult, err := t.storage.QueryEvents(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return queryResult.Events, nil
}

// Close is a no-op since we use the global shared storage
func (t *AgentLogTool) Close() error {
	return nil
}

// AgentLogInputSchema is the JSON schema for agent_log tool
var AgentLogInputSchema = GenerateSchema[AgentLogParams]()

// AgentLogDefinition is the tool definition for agent_log
var AgentLogDefinition = ToolDefinition{
	Name:        "agent_log",
	Description: "Query structured logs with SQLite backend. Supports query, trace (causality), session (replay), and stats operations.",
	InputSchema: AgentLogInputSchema,
	Function:    nil, // Execution handled in gateway
}

package rpc

import "time"

// AgentParams represents parameters for the agent RPC method
// Pattern: OpenClaw src/gateway/protocol/schema/agent.ts - AgentParamsSchema
type AgentParams struct {
	Message        string `json:"message"`
	SessionKey     string `json:"session_key"`
	IdempotencyKey string `json:"idempotency_key"`
	Lane           string `json:"lane,omitempty"`
	Timeout        int    `json:"timeout,omitempty"`
	Label          string `json:"label,omitempty"`
	SpawnedBy      string `json:"spawned_by,omitempty"`
	AnnounceBack   bool   `json:"announce_back,omitempty"`
}

// AgentResult represents the result of an agent RPC call
// Pattern: OpenClaw agent handler response
type AgentResult struct {
	RunID      string `json:"run_id"`
	Status     string `json:"status"` // "accepted", "ok", "error"
	AcceptedAt int64  `json:"accepted_at,omitempty"`
	Summary    string `json:"summary,omitempty"`
}

// AgentWaitParams represents parameters for the agent.wait RPC method
// Pattern: OpenClaw src/gateway/protocol/schema/agent.ts - AgentWaitParamsSchema
type AgentWaitParams struct {
	RunID     string `json:"run_id"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
}

// AgentWaitResult represents the result of an agent.wait call
// Pattern: OpenClaw agent.wait handler response
type AgentWaitResult struct {
	RunID     string     `json:"run_id"`
	Status    string     `json:"status"` // "ok", "error", "timeout"
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Error     string     `json:"error,omitempty"`
}

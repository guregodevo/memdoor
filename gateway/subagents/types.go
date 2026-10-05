package subagents

import (
	"time"
)

// SubagentRunRecord tracks the lifecycle of a spawned subagent
// Pattern: OpenClaw src/agents/subagent-registry.ts
type SubagentRunRecord struct {
	RunID               string           `json:"runId"`
	ChildSessionKey     string           `json:"childSessionKey"`     // agent:main:subagent:uuid
	RequesterSessionKey string           `json:"requesterSessionKey"` // agent:main:main
	RequesterDisplayKey string           `json:"requesterDisplayKey"`
	Task                string           `json:"task"`
	Cleanup             string           `json:"cleanup"` // "delete" or "keep"
	Label               string           `json:"label,omitempty"`
	ParentMessageID     int64            `json:"parentMessageId,omitempty"` // Original message ID that triggered this subagent (for threading)
	CreatedAt           time.Time        `json:"createdAt"`
	StartedAt           *time.Time       `json:"startedAt,omitempty"`
	EndedAt             *time.Time       `json:"endedAt,omitempty"`
	Outcome             *SubagentOutcome `json:"outcome,omitempty"`
	ArchiveAtMs         int64            `json:"archiveAtMs,omitempty"` // Timestamp for auto-cleanup
	CleanupCompletedAt  *time.Time       `json:"cleanupCompletedAt,omitempty"`
	CleanupHandled      bool             `json:"cleanupHandled"`
}

// SubagentOutcome represents the final outcome of a subagent run
type SubagentOutcome struct {
	Status string `json:"status"` // OutcomeOK, OutcomeError, OutcomeTimeout, "unknown"
	Error  string `json:"error,omitempty"`
}

const (
	OutcomeOK      = "ok"
	OutcomeError   = "error"
	OutcomeTimeout = "timeout"
)

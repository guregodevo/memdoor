package domain

import "time"

// SubagentRun tracks the lifecycle of a spawned subagent
type SubagentRun struct {
	RunID               string     `json:"run_id"`
	ChildSessionKey     string     `json:"child_session_key"`
	RequesterSessionKey string     `json:"requester_session_key"`
	RequesterDisplayKey string     `json:"requester_display_key"`
	Task                string     `json:"task"`
	Cleanup             string     `json:"cleanup"`
	Label               string     `json:"label,omitempty"`
	ParentMessageID     int64      `json:"parent_message_id,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	StartedAt           *time.Time `json:"started_at,omitempty"`
	EndedAt             *time.Time `json:"ended_at,omitempty"`
	OutcomeStatus       string     `json:"outcome_status,omitempty"`
	OutcomeError        string     `json:"outcome_error,omitempty"`
	ArchiveAtMs         int64      `json:"archive_at_ms,omitempty"`
	CleanupCompletedAt  *time.Time `json:"cleanup_completed_at,omitempty"`
	CleanupHandled      bool       `json:"cleanup_handled"`
}

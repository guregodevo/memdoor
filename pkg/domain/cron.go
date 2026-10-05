package domain

import "time"

// CronJob represents a scheduled job for an agent
type CronJob struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	AgentID     string    `json:"agent_id,omitempty"`
	Schedule    string    `json:"schedule"`
	Message     string    `json:"message"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// A JOB AN AGENT ASKED FOR, rather than one a person configured, carries
	// where to work and where to report, and it is bounded. A poll that waits
	// for CI runs on the project it was started from and answers into the
	// conversation that asked; it stops itself after Runs runs or at Expires,
	// because the fix for a loop is a counter, not a better prompt.
	Workdir    string    `json:"workdir,omitempty"`     // the project the check runs on
	SessionKey string    `json:"session_key,omitempty"` // where its answer lands
	MaxRuns    int       `json:"max_runs,omitempty"`    // 0 = until removed
	RunCount   int       `json:"run_count,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"` // zero = no expiry
}

// Done answers whether this job has nothing left to do: it has run its count,
// or its time is past.
func (j CronJob) Done(now time.Time) bool {
	if j.MaxRuns > 0 && j.RunCount >= j.MaxRuns {
		return true
	}
	return !j.ExpiresAt.IsZero() && now.After(j.ExpiresAt)
}

package domain

import "time"

// CronRunRecord represents a single cron job execution
type CronRunRecord struct {
	ID         string    `json:"id"`
	JobID      string    `json:"job_id"`
	AgentID    string    `json:"agent_id"`
	SessionKey string    `json:"session_key"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	DurationMs int64     `json:"duration_ms"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	Message    string    `json:"message,omitempty"`
}

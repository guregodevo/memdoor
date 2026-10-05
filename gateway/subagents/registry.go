package subagents

import (
	"context"
	"log/slog"
	"time"

	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/pkg/domain"
	"memdoor/pkg/repository"
)

// SubagentRegistry tracks the lifecycle of all spawned subagents
// Backed by SQLite repository (Raft-replicated in cluster mode)
type SubagentRegistry struct {
	repo    repository.SubagentRunRepository
	verbose bool
	log     *logs.EventLogger
}

// NewSubagentRegistry creates a new subagent registry backed by a repository
func NewSubagentRegistry(repo repository.SubagentRunRepository, verbose bool) *SubagentRegistry {
	return &SubagentRegistry{
		repo:    repo,
		verbose: verbose,
		log:     logs.New("Subagents"),
	}
}

// Register adds a new subagent run to the registry
func (r *SubagentRegistry) Register(record *SubagentRunRecord) error {
	if record.RunID == "" {
		return nil
	}

	run := recordToDomain(record)
	if err := r.repo.Create(context.Background(), run); err != nil {
		return err
	}

	r.log.Debug("Registered run",
		slog.String("run_id", record.RunID),
		slog.String("task", record.Task))

	return nil
}

// ReactivateChildSession clears any prior run records bound to a child session key
// so the session can be re-run cleanly for a follow-up turn (OpenClaw reactivation,
// src/agents/subagent-registry-run-manager.ts replaceSubagentRunAfterSteer). Without
// this, a finalized run (EndedAt set, CleanupHandled) shadows the session: lifecycle
// lookups by child session key resolve to the ended record and its completion/announce
// is deduped away, so a naive re-run stalls. Deleting the prior runs lets the fresh
// run own the session's lifecycle.
func (r *SubagentRegistry) ReactivateChildSession(childSessionKey string) {
	for _, run := range r.ListAll() {
		if run.ChildSessionKey != childSessionKey {
			continue
		}
		if err := r.repo.Delete(context.Background(), run.RunID); err != nil {
			r.log.Warn("reactivate: failed to clear prior run",
				slog.String("run_id", run.RunID),
				slog.String("child_session", childSessionKey),
				slog.String("error", err.Error()))
		}
	}
}

// Get retrieves a subagent run by ID
func (r *SubagentRegistry) Get(runID string) (*SubagentRunRecord, bool) {
	run, err := r.repo.GetByID(context.Background(), runID)
	if err != nil {
		return nil, false
	}
	return domainToRecord(run), true
}

// ListAll returns all subagent runs
func (r *SubagentRegistry) ListAll() []*SubagentRunRecord {
	runs, err := r.repo.List(context.Background())
	if err != nil {
		r.log.Warn("Failed to list subagent runs", slog.String("error", err.Error()))
		return nil
	}
	records := make([]*SubagentRunRecord, len(runs))
	for i, run := range runs {
		records[i] = domainToRecord(run)
	}
	return records
}

// OnLifecycleEvent handles agent lifecycle events (start, end, error)
func (r *SubagentRegistry) OnLifecycleEvent(event *infra.AgentEvent) {
	if event == nil {
		return
	}

	if !isSubagentSession(event.SessionID) {
		return
	}

	// Find run by child session key
	run, err := r.repo.GetByChildSession(context.Background(), event.SessionID)
	if err != nil {
		return // Not a registered subagent run
	}

	// Extract event type from Data
	eventType := ""
	if event.Data != nil {
		if et, ok := event.Data["event"].(string); ok {
			eventType = et
		}
	}

	switch eventType {
	case "start", "agent_start":
		now := time.Now()
		run.StartedAt = &now
		r.log.Debug("Run started", slog.String("run_id", run.RunID))

	case "end", "agent_end", "complete":
		now := time.Now()
		run.EndedAt = &now
		if run.OutcomeStatus == "" {
			run.OutcomeStatus = "ok"
		}
		r.log.Debug("Run ended",
			slog.String("run_id", run.RunID),
			slog.String("status", run.OutcomeStatus))

	case "error", "agent_error":
		now := time.Now()
		run.EndedAt = &now
		errorMsg := ""
		if event.Data != nil {
			if msg, ok := event.Data["message"].(string); ok {
				errorMsg = msg
			}
		}
		run.OutcomeStatus = "error"
		run.OutcomeError = errorMsg
		r.log.Debug("Run failed",
			slog.String("run_id", run.RunID),
			slog.String("error", errorMsg))

	default:
		return
	}

	if err := r.repo.Update(context.Background(), run); err != nil {
		r.log.Warn("Failed to update run after lifecycle event", slog.String("error", err.Error()))
	}
}

// Sweep archives old completed runs
func (r *SubagentRegistry) Sweep() {
	nowMs := time.Now().UnixMilli()
	deleted, err := r.repo.DeleteArchived(context.Background(), nowMs)
	if err != nil {
		r.log.Warn("Failed to sweep archived runs", slog.String("error", err.Error()))
		return
	}
	if deleted > 0 {
		r.log.Debug("Swept archived runs", slog.Int64("count", deleted))
	}
}

// Persist is a no-op — data is already in SQLite
func (r *SubagentRegistry) Persist() error {
	return nil
}

// UpdateOutcome updates the outcome of a run
func (r *SubagentRegistry) UpdateOutcome(runID string, outcome *SubagentOutcome) error {
	run, err := r.repo.GetByID(context.Background(), runID)
	if err != nil {
		return err
	}

	if outcome != nil {
		run.OutcomeStatus = outcome.Status
		run.OutcomeError = outcome.Error
	}

	return r.repo.Update(context.Background(), run)
}

// MarkEnded sets a run's EndedAt (with a default OK outcome) synchronously if it
// isn't already set. handleSubagentCompletion runs AFTER the subagent's
// ProcessMessage returns — the run is definitively finished — so this removes the
// race with the async lifecycle event that also sets EndedAt (and which
// ShouldAnnounce depends on). Without it a fast completion is dropped: the announce
// is skipped and the requesting agent never gets the result back.
func (r *SubagentRegistry) MarkEnded(childSessionKey string) {
	run, err := r.repo.GetByChildSession(context.Background(), childSessionKey)
	if err != nil || run == nil {
		return
	}
	if run.EndedAt != nil {
		return
	}
	now := time.Now()
	run.EndedAt = &now
	if run.OutcomeStatus == "" {
		run.OutcomeStatus = "ok"
	}
	if err := r.repo.Update(context.Background(), run); err != nil {
		r.log.Warn("MarkEnded update failed", slog.String("error", err.Error()))
	}
}

// MarkFailed ends a run with a failure outcome ("error" or "timeout") and
// its text, so the announcement to the requester says what happened
// instead of "completed successfully" over an empty result.
func (r *SubagentRegistry) MarkFailed(childSessionKey, status, text string) {
	run, err := r.repo.GetByChildSession(context.Background(), childSessionKey)
	if err != nil || run == nil {
		return
	}
	if run.EndedAt == nil {
		now := time.Now()
		run.EndedAt = &now
	}
	run.OutcomeStatus = status
	run.OutcomeError = text
	if err := r.repo.Update(context.Background(), run); err != nil {
		r.log.Warn("MarkFailed update failed", slog.String("error", err.Error()))
	}
}

// MarkCleanupHandled marks a run as having its cleanup handled
func (r *SubagentRegistry) MarkCleanupHandled(runID string) error {
	run, err := r.repo.GetByID(context.Background(), runID)
	if err != nil {
		return err
	}

	now := time.Now()
	run.CleanupHandled = true
	run.CleanupCompletedAt = &now

	return r.repo.Update(context.Background(), run)
}

// WaitForRun polls the registry for run completion with timeout
func (r *SubagentRegistry) WaitForRun(runID string, timeout time.Duration) *RunSnapshot {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		run, err := r.repo.GetByID(context.Background(), runID)
		if err != nil {
			return nil
		}

		if run.EndedAt != nil {
			snapshot := &RunSnapshot{
				StartedAt: run.StartedAt,
				EndedAt:   run.EndedAt,
			}
			if run.OutcomeStatus != "" {
				snapshot.Status = run.OutcomeStatus
				snapshot.Error = run.OutcomeError
			} else {
				snapshot.Status = "ok"
			}
			return snapshot
		}

		if time.Now().After(deadline) {
			return nil
		}

		<-ticker.C
	}
}

// RunSnapshot represents the status of a subagent run
type RunSnapshot struct {
	Status    string     `json:"status"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// Helper function to check if session is a subagent session
func isSubagentSession(sessionID string) bool {
	return len(sessionID) > len("agent:main:subagent:") &&
		sessionID[:len("agent:main:subagent:")] == "agent:main:subagent:"
}

// --- Conversion helpers ---

func recordToDomain(r *SubagentRunRecord) *domain.SubagentRun {
	run := &domain.SubagentRun{
		RunID:               r.RunID,
		ChildSessionKey:     r.ChildSessionKey,
		RequesterSessionKey: r.RequesterSessionKey,
		RequesterDisplayKey: r.RequesterDisplayKey,
		Task:                r.Task,
		Cleanup:             r.Cleanup,
		Label:               r.Label,
		ParentMessageID:     r.ParentMessageID,
		CreatedAt:           r.CreatedAt,
		StartedAt:           r.StartedAt,
		EndedAt:             r.EndedAt,
		ArchiveAtMs:         r.ArchiveAtMs,
		CleanupCompletedAt:  r.CleanupCompletedAt,
		CleanupHandled:      r.CleanupHandled,
	}
	if r.Outcome != nil {
		run.OutcomeStatus = r.Outcome.Status
		run.OutcomeError = r.Outcome.Error
	}
	return run
}

func domainToRecord(d *domain.SubagentRun) *SubagentRunRecord {
	record := &SubagentRunRecord{
		RunID:               d.RunID,
		ChildSessionKey:     d.ChildSessionKey,
		RequesterSessionKey: d.RequesterSessionKey,
		RequesterDisplayKey: d.RequesterDisplayKey,
		Task:                d.Task,
		Cleanup:             d.Cleanup,
		Label:               d.Label,
		ParentMessageID:     d.ParentMessageID,
		CreatedAt:           d.CreatedAt,
		StartedAt:           d.StartedAt,
		EndedAt:             d.EndedAt,
		ArchiveAtMs:         d.ArchiveAtMs,
		CleanupCompletedAt:  d.CleanupCompletedAt,
		CleanupHandled:      d.CleanupHandled,
	}
	if d.OutcomeStatus != "" {
		record.Outcome = &SubagentOutcome{
			Status: d.OutcomeStatus,
			Error:  d.OutcomeError,
		}
	}
	return record
}

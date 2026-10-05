package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"memdoor/pkg/domain"
	"memdoor/pkg/repository"
)

type subagentRunRepository struct {
	db *sql.DB
}

func NewSubagentRunRepository(db *sql.DB) repository.SubagentRunRepository {
	return &subagentRunRepository{db: db}
}

func (r *subagentRunRepository) Create(ctx context.Context, run *domain.SubagentRun) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO subagent_runs (run_id, child_session_key, requester_session_key, requester_display_key,
		 task, cleanup, label, parent_message_id, created_at, started_at, ended_at,
		 outcome_status, outcome_error, archive_at_ms, cleanup_completed_at, cleanup_handled)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.RunID, run.ChildSessionKey, run.RequesterSessionKey, run.RequesterDisplayKey,
		run.Task, run.Cleanup, run.Label, run.ParentMessageID,
		run.CreatedAt.UnixMilli(), nullableTimeMs(run.StartedAt), nullableTimeMs(run.EndedAt),
		run.OutcomeStatus, run.OutcomeError, run.ArchiveAtMs,
		nullableTimeMs(run.CleanupCompletedAt), run.CleanupHandled)
	if err != nil {
		return fmt.Errorf("insert subagent run: %w", err)
	}
	return nil
}

func (r *subagentRunRepository) Update(ctx context.Context, run *domain.SubagentRun) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE subagent_runs SET started_at=?, ended_at=?, outcome_status=?, outcome_error=?,
		 archive_at_ms=?, cleanup_completed_at=?, cleanup_handled=? WHERE run_id=?`,
		nullableTimeMs(run.StartedAt), nullableTimeMs(run.EndedAt),
		run.OutcomeStatus, run.OutcomeError, run.ArchiveAtMs,
		nullableTimeMs(run.CleanupCompletedAt), run.CleanupHandled, run.RunID)
	if err != nil {
		return fmt.Errorf("update subagent run: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("subagent run %s not found", run.RunID)
	}
	return nil
}

func (r *subagentRunRepository) GetByID(ctx context.Context, runID string) (*domain.SubagentRun, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT run_id, child_session_key, requester_session_key, requester_display_key,
		 task, cleanup, label, parent_message_id, created_at, started_at, ended_at,
		 outcome_status, outcome_error, archive_at_ms, cleanup_completed_at, cleanup_handled
		 FROM subagent_runs WHERE run_id=?`, runID)
	return scanSubagentRun(row)
}

func (r *subagentRunRepository) GetByChildSession(ctx context.Context, childSessionKey string) (*domain.SubagentRun, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT run_id, child_session_key, requester_session_key, requester_display_key,
		 task, cleanup, label, parent_message_id, created_at, started_at, ended_at,
		 outcome_status, outcome_error, archive_at_ms, cleanup_completed_at, cleanup_handled
		 FROM subagent_runs WHERE child_session_key=?`, childSessionKey)
	return scanSubagentRun(row)
}

func (r *subagentRunRepository) List(ctx context.Context) ([]*domain.SubagentRun, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT run_id, child_session_key, requester_session_key, requester_display_key,
		 task, cleanup, label, parent_message_id, created_at, started_at, ended_at,
		 outcome_status, outcome_error, archive_at_ms, cleanup_completed_at, cleanup_handled
		 FROM subagent_runs ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query subagent runs: %w", err)
	}
	defer rows.Close()

	var runs []*domain.SubagentRun
	for rows.Next() {
		run, err := scanSubagentRunRow(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *subagentRunRepository) Delete(ctx context.Context, runID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM subagent_runs WHERE run_id=?`, runID)
	if err != nil {
		return fmt.Errorf("delete subagent run: %w", err)
	}
	return nil
}

func (r *subagentRunRepository) DeleteArchived(ctx context.Context, beforeMs int64) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM subagent_runs WHERE archive_at_ms > 0 AND archive_at_ms < ?`, beforeMs)
	if err != nil {
		return 0, fmt.Errorf("delete archived subagent runs: %w", err)
	}
	return result.RowsAffected()
}

func nullableTimeMs(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}

func scanSubagentRun(row *sql.Row) (*domain.SubagentRun, error) {
	var run domain.SubagentRun
	var createdMs int64
	var startedMs, endedMs, cleanupMs *int64
	if err := row.Scan(&run.RunID, &run.ChildSessionKey, &run.RequesterSessionKey, &run.RequesterDisplayKey,
		&run.Task, &run.Cleanup, &run.Label, &run.ParentMessageID,
		&createdMs, &startedMs, &endedMs,
		&run.OutcomeStatus, &run.OutcomeError, &run.ArchiveAtMs,
		&cleanupMs, &run.CleanupHandled); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subagent run not found")
		}
		return nil, fmt.Errorf("scan subagent run: %w", err)
	}
	run.CreatedAt = time.UnixMilli(createdMs)
	run.StartedAt = msToTime(startedMs)
	run.EndedAt = msToTime(endedMs)
	run.CleanupCompletedAt = msToTime(cleanupMs)
	return &run, nil
}

func scanSubagentRunRow(rows *sql.Rows) (*domain.SubagentRun, error) {
	var run domain.SubagentRun
	var createdMs int64
	var startedMs, endedMs, cleanupMs *int64
	if err := rows.Scan(&run.RunID, &run.ChildSessionKey, &run.RequesterSessionKey, &run.RequesterDisplayKey,
		&run.Task, &run.Cleanup, &run.Label, &run.ParentMessageID,
		&createdMs, &startedMs, &endedMs,
		&run.OutcomeStatus, &run.OutcomeError, &run.ArchiveAtMs,
		&cleanupMs, &run.CleanupHandled); err != nil {
		return nil, fmt.Errorf("scan subagent run: %w", err)
	}
	run.CreatedAt = time.UnixMilli(createdMs)
	run.StartedAt = msToTime(startedMs)
	run.EndedAt = msToTime(endedMs)
	run.CleanupCompletedAt = msToTime(cleanupMs)
	return &run, nil
}

func msToTime(ms *int64) *time.Time {
	if ms == nil {
		return nil
	}
	t := time.UnixMilli(*ms)
	return &t
}

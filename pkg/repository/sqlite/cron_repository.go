package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"memdoor/pkg/domain"
	"memdoor/pkg/repository"
)

type cronJobRepository struct {
	db *sql.DB
}

// NewCronJobRepository creates a new SQLite-backed cron job repository
func NewCronJobRepository(db *sql.DB) repository.CronJobRepository {
	return &cronJobRepository{db: db}
}

func (r *cronJobRepository) Create(ctx context.Context, job *domain.CronJob) error {
	now := time.Now().Unix()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO cron_jobs (`+cronColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.WorkspaceID, job.AgentID, job.Schedule, job.Message, job.Enabled, now, now,
		job.Workdir, job.SessionKey, job.MaxRuns, job.RunCount, unixOrZero(job.ExpiresAt))
	if err != nil {
		return fmt.Errorf("insert cron job: %w", err)
	}
	job.CreatedAt = time.Unix(now, 0)
	job.UpdatedAt = time.Unix(now, 0)
	return nil
}

func (r *cronJobRepository) Update(ctx context.Context, job *domain.CronJob) error {
	now := time.Now().Unix()
	result, err := r.db.ExecContext(ctx,
		`UPDATE cron_jobs SET agent_id=?, schedule=?, message=?, enabled=?, updated_at=?,
		 workdir=?, session_key=?, max_runs=?, run_count=?, expires_at=? WHERE id=?`,
		job.AgentID, job.Schedule, job.Message, job.Enabled, now,
		job.Workdir, job.SessionKey, job.MaxRuns, job.RunCount, unixOrZero(job.ExpiresAt), job.ID)
	if err != nil {
		return fmt.Errorf("update cron job: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("cron job %s not found", job.ID)
	}
	job.UpdatedAt = time.Unix(now, 0)
	return nil
}

func (r *cronJobRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM cron_jobs WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete cron job: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("cron job %s not found", id)
	}
	return nil
}

func (r *cronJobRepository) GetByID(ctx context.Context, id string) (*domain.CronJob, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+cronColumns+` FROM cron_jobs WHERE id=?`, id)
	return readCronJob(row)
}

func (r *cronJobRepository) List(ctx context.Context, workspaceID string) ([]*domain.CronJob, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+cronColumns+` FROM cron_jobs WHERE workspace_id=? ORDER BY id`,
		workspaceID)
	if err != nil {
		return nil, fmt.Errorf("query cron jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.CronJob
	for rows.Next() {
		job, err := readCronJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// cronColumns is the row, in one place: a SELECT that drifts from the scanner
// decodes a job into the wrong fields, silently.
const cronColumns = `id, workspace_id, agent_id, schedule, message, enabled, created_at, updated_at, workdir, session_key, max_runs, run_count, expires_at`

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// scanner is a *sql.Row or *sql.Rows: one Scan.
type scanner interface {
	Scan(dest ...interface{}) error
}

func readCronJob(row scanner) (*domain.CronJob, error) {
	var job domain.CronJob
	var createdAt, updatedAt, expiresAt int64
	if err := row.Scan(&job.ID, &job.WorkspaceID, &job.AgentID, &job.Schedule, &job.Message, &job.Enabled,
		&createdAt, &updatedAt, &job.Workdir, &job.SessionKey, &job.MaxRuns, &job.RunCount, &expiresAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("cron job not found")
		}
		return nil, fmt.Errorf("scan cron job: %w", err)
	}
	job.CreatedAt = time.Unix(createdAt, 0)
	job.UpdatedAt = time.Unix(updatedAt, 0)
	if expiresAt > 0 {
		job.ExpiresAt = time.Unix(expiresAt, 0)
	}
	return &job, nil
}

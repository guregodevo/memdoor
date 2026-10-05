package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"memdoor/pkg/domain"
	"memdoor/pkg/repository"
)

type cronHistoryRepository struct {
	db *sql.DB
}

func NewCronHistoryRepository(db *sql.DB) repository.CronHistoryRepository {
	return &cronHistoryRepository{db: db}
}

func (r *cronHistoryRepository) Record(ctx context.Context, record *domain.CronRunRecord) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO cron_history (id, job_id, agent_id, session_key, start_time, end_time, duration_ms, success, error, message)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.JobID, record.AgentID, record.SessionKey,
		record.StartTime.UnixMilli(), record.EndTime.UnixMilli(),
		record.DurationMs, record.Success, record.Error, record.Message)
	if err != nil {
		return fmt.Errorf("insert cron history: %w", err)
	}
	return nil
}

func (r *cronHistoryRepository) ListByJob(ctx context.Context, jobID string, limit int) ([]*domain.CronRunRecord, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, job_id, agent_id, session_key, start_time, end_time, duration_ms, success, error, message
		 FROM cron_history WHERE job_id = ? ORDER BY end_time DESC LIMIT ?`, jobID, limit)
	if err != nil {
		return nil, fmt.Errorf("query cron history: %w", err)
	}
	defer rows.Close()
	return scanCronHistoryRows(rows)
}

func (r *cronHistoryRepository) ListRecent(ctx context.Context, limit int) ([]*domain.CronRunRecord, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, job_id, agent_id, session_key, start_time, end_time, duration_ms, success, error, message
		 FROM cron_history ORDER BY end_time DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query cron history: %w", err)
	}
	defer rows.Close()
	return scanCronHistoryRows(rows)
}

func (r *cronHistoryRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM cron_history WHERE end_time < ?`, before.UnixMilli())
	if err != nil {
		return 0, fmt.Errorf("delete old cron history: %w", err)
	}
	return result.RowsAffected()
}

func scanCronHistoryRows(rows *sql.Rows) ([]*domain.CronRunRecord, error) {
	var records []*domain.CronRunRecord
	for rows.Next() {
		var r domain.CronRunRecord
		var startMs, endMs int64
		if err := rows.Scan(&r.ID, &r.JobID, &r.AgentID, &r.SessionKey,
			&startMs, &endMs, &r.DurationMs, &r.Success, &r.Error, &r.Message); err != nil {
			return nil, fmt.Errorf("scan cron history: %w", err)
		}
		r.StartTime = time.UnixMilli(startMs)
		r.EndTime = time.UnixMilli(endMs)
		records = append(records, &r)
	}
	return records, rows.Err()
}

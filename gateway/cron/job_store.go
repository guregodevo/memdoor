package cron

import (
	"context"

	"memdoor/gateway/config"
	"memdoor/pkg/domain"
	"memdoor/pkg/repository"
)

// JobStore is the interface used by the cron Scheduler for persistent job storage.
// Implementations: file-based Store (legacy), DBJobStore (SQLite, Raft-replicated).
type JobStore interface {
	ListJobs() []*config.CronJob
	AddJob(job *config.CronJob) error
	// UpdateJob rewrites a stored job — a run counted, a job turned off.
	UpdateJob(job *config.CronJob) error
	RemoveJob(jobID string) error
	GetJob(jobID string) (*config.CronJob, error)
}

// DBJobStore adapts repository.CronJobRepository to the JobStore interface.
// Converts between domain.CronJob (DB) and config.CronJob (scheduler).
type DBJobStore struct {
	repo        repository.CronJobRepository
	workspaceID string
}

// NewDBJobStore creates a SQLite-backed job store.
func NewDBJobStore(repo repository.CronJobRepository, workspaceID string) JobStore {
	return &DBJobStore{repo: repo, workspaceID: workspaceID}
}

func (s *DBJobStore) ListJobs() []*config.CronJob {
	jobs, err := s.repo.List(context.Background(), s.workspaceID)
	if err != nil {
		return nil
	}
	result := make([]*config.CronJob, len(jobs))
	for i, j := range jobs {
		result[i] = domainToConfigJob(j)
	}
	return result
}

func (s *DBJobStore) AddJob(job *config.CronJob) error {
	return s.repo.Create(context.Background(), configToDomainJob(job, s.workspaceID))
}

func (s *DBJobStore) RemoveJob(jobID string) error {
	return s.repo.Delete(context.Background(), jobID)
}

func (s *DBJobStore) GetJob(jobID string) (*config.CronJob, error) {
	dj, err := s.repo.GetByID(context.Background(), jobID)
	if err != nil {
		return nil, err
	}
	return domainToConfigJob(dj), nil
}

// UpdateJob rewrites the stored row for a job the scheduler already holds.
func (s *DBJobStore) UpdateJob(job *config.CronJob) error {
	return s.repo.Update(context.Background(), configToDomainJob(job, s.workspaceID))
}

func domainToConfigJob(dj *domain.CronJob) *config.CronJob {
	return &config.CronJob{
		ID:       dj.ID,
		Schedule: dj.Schedule,
		AgentID:  dj.AgentID,
		Message:  dj.Message,
		Enabled:  dj.Enabled,

		Workdir:    dj.Workdir,
		SessionKey: dj.SessionKey,
		MaxRuns:    dj.MaxRuns,
		RunCount:   dj.RunCount,
		ExpiresAt:  dj.ExpiresAt,
	}
}

func configToDomainJob(cj *config.CronJob, workspaceID string) *domain.CronJob {
	return &domain.CronJob{
		ID:          cj.ID,
		WorkspaceID: workspaceID,
		AgentID:     cj.AgentID,
		Schedule:    cj.Schedule,
		Message:     cj.Message,
		Enabled:     cj.Enabled,

		Workdir:    cj.Workdir,
		SessionKey: cj.SessionKey,
		MaxRuns:    cj.MaxRuns,
		RunCount:   cj.RunCount,
		ExpiresAt:  cj.ExpiresAt,
	}
}

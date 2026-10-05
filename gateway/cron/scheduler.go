package cron

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"memdoor/gateway/config"
	"memdoor/gateway/logs"
	"memdoor/gateway/routing"
	"memdoor/pkg/domain"
	"memdoor/pkg/repository"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// Scheduler manages cron jobs with per-agent assignment
// Pattern: OpenClaw src/gateway/server-cron.ts
//
// Features:
// - Per-agent job assignment (jobs run in specific agent contexts)
// - Cron expression scheduling (e.g., "0 9 * * *" = 9am daily)
// - Job execution tracking and logging
// - Concurrent job execution with limits
// - Persistent job storage
type Scheduler struct {
	cfg      *config.Config
	cron     *cron.Cron
	jobs     map[string]cron.EntryID          // job ID -> cron entry ID
	defs     map[string]*config.CronJob       // job ID -> what it is, for Jobs()
	executor JobExecutor                      // Function to execute jobs
	store    JobStore                         // Persistent job store
	history  repository.CronHistoryRepository // Execution history (SQLite-backed)
	mu       sync.RWMutex
	log      *logs.EventLogger
}

// JobExecutor is a function that executes a cron job
// Pattern: OpenClaw runIsolatedAgentJob callback
type JobExecutor func(ctx context.Context, job *config.CronJob, agentID string, sessionKey string) error

// cronParser accepts BOTH standard 5-field crontab ("0 * * * *") and the
// 6-field form with a leading seconds field ("*/30 * * * * *"). Seconds are
// optional and default to 0, so a 5-field expression means exactly what it does
// in every other cron on earth. This is what a person — or a small model, which
// the roadmap notes gets cron fields wrong — actually types; requiring 6 fields
// silently stored-but-never-scheduled a valid-looking schedule. The scheduler
// (AddFunc) and the create-time validator share this one parser so they can
// never disagree about what a schedule string means.
var cronParser = cron.NewParser(
	cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// ValidateSchedule reports whether expr is a schedule the scheduler can run
// (5- or 6-field crontab, or a named descriptor). Callers use it to reject a
// bad schedule at the boundary — before persisting a job that could never
// fire — using the exact parser the scheduler itself uses.
func ValidateSchedule(expr string) error {
	if _, err := cronParser.Parse(expr); err != nil {
		return fmt.Errorf("invalid cron schedule %q: %w", expr, err)
	}
	return nil
}

// NewScheduler creates a new cron scheduler
// Pattern: OpenClaw buildGatewayCronService
func NewScheduler(cfg *config.Config, executor JobExecutor, store JobStore, history repository.CronHistoryRepository, verbose bool) (*Scheduler, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	if executor == nil {
		return nil, fmt.Errorf("job executor is required")
	}

	// Accept 5-field crontab and 6-field (seconds) alike — see cronParser.
	c := cron.New(cron.WithParser(cronParser))

	log := logs.New("Cron")

	scheduler := &Scheduler{
		cfg:      cfg,
		cron:     c,
		jobs:     make(map[string]cron.EntryID),
		defs:     make(map[string]*config.CronJob),
		executor: executor,
		store:    store,
		history:  history,
		log:      log,
	}

	log.Debug("Cron scheduler initialized")

	return scheduler, nil
}

// Start starts the cron scheduler
// Pattern: OpenClaw cron service start
func (s *Scheduler) Start() error {
	if s.cfg.Cron == nil || !s.cfg.Cron.Enabled {
		s.log.Debug("Cron scheduler disabled, not starting")
		return nil
	}

	// Register all configured jobs
	if err := s.registerConfiguredJobs(); err != nil {
		return fmt.Errorf("failed to register cron jobs: %w", err)
	}

	// Start the scheduler
	s.cron.Start()

	s.log.Debug("Cron scheduler started", slog.Int("job_count", len(s.jobs)))

	return nil
}

// Stop stops the cron scheduler
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()

	s.log.Debug("Cron scheduler stopped")
}

// registerConfiguredJobs registers all jobs from configuration and persistent store
// Pattern: OpenClaw job registration from config + Memdoor persistent storage
func (s *Scheduler) registerConfiguredJobs() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// First, register jobs from config (legacy)
	if s.cfg.Cron != nil {
		for _, job := range s.cfg.Cron.Jobs {
			// Skip disabled jobs
			if !job.Enabled {
				s.log.Debug("Skipping disabled cron job", slog.String("job_id", job.ID))
				continue
			}

			// Validate job configuration
			if err := s.validateJob(&job); err != nil {
				s.log.Warn("Invalid cron job",
					slog.String("job_id", job.ID),
					slog.String("error", err.Error()))
				continue
			}

			// Register job
			if err := s.addJobLocked(&job); err != nil {
				s.log.Warn("Failed to register cron job",
					slog.String("job_id", job.ID),
					slog.String("error", err.Error()))
				continue
			}
		}
	}

	// Second, register jobs from persistent store (takes precedence)
	if s.store != nil {
		storedJobs := s.store.ListJobs()
		for _, job := range storedJobs {
			// Skip disabled jobs
			if !job.Enabled {
				s.log.Debug("Skipping disabled stored job", slog.String("job_id", job.ID))
				continue
			}

			// Validate job configuration
			if err := s.validateJob(job); err != nil {
				s.log.Warn("Invalid stored job",
					slog.String("job_id", job.ID),
					slog.String("error", err.Error()))
				continue
			}

			// If job already registered from config, skip (store takes precedence, but already registered)
			// Actually, we should override config jobs with store jobs
			// Remove config job if exists, then add store job
			if _, exists := s.jobs[job.ID]; exists {
				// Remove old entry
				s.cron.Remove(s.jobs[job.ID])
				s.log.Debug("Overriding config job with stored version", slog.String("job_id", job.ID))
			}

			// Register stored job
			if err := s.addJobLocked(job); err != nil {
				s.log.Warn("Failed to register stored job",
					slog.String("job_id", job.ID),
					slog.String("error", err.Error()))
				continue
			}
		}
	}

	return nil
}

// validateJob validates a cron job configuration
func (s *Scheduler) validateJob(job *config.CronJob) error {
	if job.ID == "" {
		return fmt.Errorf("job ID is required")
	}

	if job.Schedule == "" {
		return fmt.Errorf("job schedule is required")
	}

	if job.Message == "" {
		return fmt.Errorf("job message is required")
	}

	// Validate with the SAME parser the scheduler uses, so a schedule that
	// validates here always adds cleanly in addJobLocked (5- or 6-field).
	if err := ValidateSchedule(job.Schedule); err != nil {
		return err
	}

	return nil
}

// addJobLocked adds a job to the scheduler (must hold lock)
func (s *Scheduler) addJobLocked(job *config.CronJob) error {
	// Resolve agent ID
	agentID := s.resolveJobAgentID(job)

	// Build session key for cron job
	// Pattern: OpenClaw cron:{jobId} session key format
	sessionKey := routing.BuildAgentCronSessionKey(agentID, job.ID)

	// Create job wrapper
	wrapper := s.createJobWrapper(job, agentID, sessionKey)

	// Add to cron scheduler
	entryID, err := s.cron.AddFunc(job.Schedule, wrapper)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	// Track entry ID
	s.jobs[job.ID] = entryID
	s.defs[job.ID] = job

	s.log.Debug("Registered cron job",
		slog.String("job_id", job.ID),
		slog.String("agent_id", agentID),
		slog.String("schedule", job.Schedule))

	return nil
}

// resolveJobAgentID resolves which agent should run a job
// Pattern: OpenClaw resolveCronAgent
func (s *Scheduler) resolveJobAgentID(job *config.CronJob) string {
	// If job specifies an agent, use it
	if job.AgentID != "" {
		// Normalize and verify agent exists
		normalizedID := routing.NormalizeAgentID(job.AgentID)
		if s.cfg.GetAgent(normalizedID) != nil {
			return normalizedID
		}

		// Agent not found, log warning and fall back to default
		s.log.Warn("Cron job specifies unknown agent, using default",
			slog.String("job_id", job.ID),
			slog.String("specified_agent", job.AgentID))
	}

	// Use default agent
	defaultAgent := s.cfg.GetDefaultAgent()
	if defaultAgent != nil {
		return routing.NormalizeAgentID(defaultAgent.ID)
	}

	// Fallback to "main"
	return routing.DefaultAgentID
}

// createJobWrapper creates a wrapper function for job execution
// Pattern: OpenClaw job execution with error handling and logging + history tracking
func (s *Scheduler) createJobWrapper(job *config.CronJob, agentID string, sessionKey string) func() {
	var running atomic.Bool
	return func() {
		// A POLL MUST NOT PILE UP. A check that takes longer than its interval
		// would otherwise start again beside itself, and two turns in one
		// session is how a wait for CI becomes a fork bomb.
		if !running.CompareAndSwap(false, true) {
			s.log.Debug("Cron job still running, skipping this tick", slog.String("job_id", job.ID))
			return
		}
		defer running.Store(false)

		// Bounds first: the fix for a loop is a counter, not a better prompt.
		if done, why := s.boundsReached(job); done {
			s.log.Info("Cron job finished: "+why, slog.String("job_id", job.ID))
			if err := s.RemoveJob(job.ID); err != nil {
				s.log.WithError(err).Warn("Could not remove a finished job", slog.String("job_id", job.ID))
			}
			return
		}
		s.countRun(job)

		startTime := time.Now()

		// Info, not Debug: a scheduled run that leaves no line in the log is a
		// run nobody can prove happened (2026-10-01: two minutes of "did it
		// fire?" with nothing to read).
		s.log.Info("Executing cron job",
			slog.String("job_id", job.ID),
			slog.String("agent_id", agentID),
			slog.String("message", job.Message))

		// Hang backstop, not a pacer — shares the agent-execution timeout so a
		// long run isn't cut off. See config.AgentBackstopTimeout.
		ctx, cancel := context.WithTimeout(context.Background(), config.AgentBackstopTimeout())
		defer cancel()

		// Execute job
		err := s.executor(ctx, job, agentID, sessionKey)

		endTime := time.Now()
		duration := endTime.Sub(startTime)

		// Record run in history
		if s.history != nil {
			record := &domain.CronRunRecord{
				ID:         uuid.New().String(),
				JobID:      job.ID,
				AgentID:    agentID,
				SessionKey: sessionKey,
				StartTime:  startTime,
				EndTime:    endTime,
				DurationMs: duration.Milliseconds(),
				Success:    err == nil,
				Message:    job.Message,
			}

			if err != nil {
				record.Error = err.Error()
			}

			if histErr := s.history.Record(context.Background(), record); histErr != nil {
				s.log.Warn("Failed to record run history",
					slog.String("job_id", job.ID),
					slog.String("error", histErr.Error()))
			}
		}

		if err != nil {
			s.log.WithError(err).Error("Cron job failed",
				slog.String("job_id", job.ID),
				slog.Duration("duration", duration))
		} else {
			s.log.Debug("Cron job completed successfully",
				slog.String("job_id", job.ID),
				slog.Duration("duration", duration))
		}

		// The last run takes the job with it: waiting for the next tick to
		// notice left a finished job in `cron list` for one more interval.
		if done, why := s.boundsReached(job); done {
			s.log.Info("Cron job finished: "+why, slog.String("job_id", job.ID))
			if err := s.RemoveJob(job.ID); err != nil {
				s.log.WithError(err).Warn("Could not remove a finished job", slog.String("job_id", job.ID))
			}
		}
	}
}

// boundsReached answers whether a job has run its count or outlived its
// expiry, and says which, for the log line that explains its removal.
func (s *Scheduler) boundsReached(job *config.CronJob) (bool, string) {
	if job.MaxRuns > 0 && job.RunCount >= job.MaxRuns {
		return true, fmt.Sprintf("ran %d of %d times", job.RunCount, job.MaxRuns)
	}
	if !job.ExpiresAt.IsZero() && time.Now().After(job.ExpiresAt) {
		return true, "its time is up"
	}
	return false, ""
}

// countRun records one run, in memory and in the store, so a restart does not
// hand a bounded job a fresh budget.
func (s *Scheduler) countRun(job *config.CronJob) {
	job.RunCount++
	if s.store == nil {
		return
	}
	if err := s.store.UpdateJob(job); err != nil {
		s.log.WithError(err).Warn("Could not record a run", slog.String("job_id", job.ID))
	}
}

// AddJob adds a new job to the scheduler and persists it to the store
// Pattern: Runtime job registration (for dynamic jobs)
func (s *Scheduler) AddJob(job *config.CronJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate job
	if err := s.validateJob(job); err != nil {
		return fmt.Errorf("invalid job: %w", err)
	}

	// Check if job already exists
	if _, exists := s.jobs[job.ID]; exists {
		return fmt.Errorf("job %s already exists", job.ID)
	}

	// Add to persistent store (if available)
	if s.store != nil {
		if err := s.store.AddJob(job); err != nil {
			return fmt.Errorf("failed to persist job: %w", err)
		}
	}

	// Add to scheduler
	if err := s.addJobLocked(job); err != nil {
		// Rollback store if scheduler fails
		if s.store != nil {
			s.store.RemoveJob(job.ID)
		}
		return err
	}

	return nil
}

// ScheduleExisting registers a job that is ALREADY persisted.
//
// AddJob persists and schedules together, which is right when the scheduler
// owns the data. It is not right here: the scheduler's store is constructed
// with an empty workspace, so persisting through it writes the job under no
// workspace at all and `cron list` — which queries by workspace — never sees
// it. Only the HTTP handler knows the workspace the job belongs to.
//
// So the handler persists, and this schedules. One write, one schedule, and
// the workspace survives (2026-08-24: delegating persistence to the scheduler
// made jobs invisible to the very command that lists them).
func (s *Scheduler) ScheduleExisting(job *config.CronJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validateJob(job); err != nil {
		return fmt.Errorf("invalid job: %w", err)
	}
	if _, exists := s.jobs[job.ID]; exists {
		return fmt.Errorf("job %s already scheduled", job.ID)
	}
	return s.addJobLocked(job)
}

// UnscheduleExisting stops running a job without touching the store, for the
// same reason: the handler owns the row.
func (s *Scheduler) UnscheduleExisting(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, exists := s.jobs[jobID]
	if !exists {
		return nil // not scheduled; deleting the row is still correct
	}
	s.cron.Remove(entryID)
	delete(s.jobs, jobID)
	delete(s.defs, jobID)
	return nil
}

// RemoveJob removes a job from the scheduler and persistent store
func (s *Scheduler) RemoveJob(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, exists := s.jobs[jobID]
	if !exists {
		return fmt.Errorf("job %s not found", jobID)
	}

	// Remove from cron scheduler
	s.cron.Remove(entryID)

	// Remove from tracking
	delete(s.jobs, jobID)
	delete(s.defs, jobID)

	// Remove from persistent store (if available)
	if s.store != nil {
		if err := s.store.RemoveJob(jobID); err != nil {
			// Log error but don't fail - job is already removed from scheduler
			s.log.Warn("Failed to remove job from store",
				slog.String("job_id", jobID),
				slog.String("error", err.Error()))
		}
	}

	s.log.Debug("Removed cron job", slog.String("job_id", jobID))

	return nil
}

// ListJobs returns all registered job IDs
func (s *Scheduler) ListJobs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]string, 0, len(s.jobs))
	for jobID := range s.jobs {
		jobs = append(jobs, jobID)
	}

	return jobs
}

// Jobs is every registered job, for a caller that needs more than the ids —
// the agent's cron tool lists what is scheduled and how far through it is.
func (s *Scheduler) Jobs() []*config.CronJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*config.CronJob, 0, len(s.defs))
	for _, job := range s.defs {
		out = append(out, job)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// GetJobCount returns the number of registered jobs
func (s *Scheduler) GetJobCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.jobs)
}

// TriggerJob manually triggers a job execution (for testing)
// Pattern: Testing support - allows manual job triggering
func (s *Scheduler) TriggerJob(jobID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find the job in config
	if s.cfg.Cron == nil {
		return fmt.Errorf("cron not configured")
	}

	var targetJob *config.CronJob
	for i := range s.cfg.Cron.Jobs {
		if s.cfg.Cron.Jobs[i].ID == jobID {
			targetJob = &s.cfg.Cron.Jobs[i]
			break
		}
	}

	if targetJob == nil {
		return fmt.Errorf("job %s not found", jobID)
	}

	// Verify job is registered
	if _, exists := s.jobs[jobID]; !exists {
		return fmt.Errorf("job %s not registered in scheduler", jobID)
	}

	// Resolve agent ID and session key
	agentID := s.resolveJobAgentID(targetJob)
	sessionKey := routing.BuildAgentCronSessionKey(agentID, targetJob.ID)

	// Execute job wrapper immediately
	wrapper := s.createJobWrapper(targetJob, agentID, sessionKey)
	go wrapper()

	return nil
}

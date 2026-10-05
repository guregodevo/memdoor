package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"memdoor/gateway/config"
	"memdoor/gateway/cron"
	"net/http"
	"strings"
	"time"

	"memdoor/pkg/domain"
	"memdoor/pkg/shared"
)

// cronWorkspaceID returns the workspace a cron request is scoped to, mapping the
// internal-service placeholder to the real single-tenant workspace.
//
// A tool running inside the gateway reaches /api/cron over loopback with
// X-Internal-Service; the auth middleware stamps that context with the literal
// WorkspaceID "default" (pkg/authorization/middleware.go). But a token request
// — every CLI `cron list` — carries the user's real workspace, which on a
// single-tenant gateway is domain.DefaultWorkspaceID (a UUID). Stored under
// "default", queried under the UUID, an agent-created job was invisible to
// `cron list`. Normalising both to DefaultWorkspaceID makes them agree.
//
// Known limit: the middleware discards the agent's ACTUAL workspace for every
// internal call, so on a genuinely multi-tenant gateway an agent's cron job
// still lands in the default workspace rather than the agent's own. Propagating
// the agent's workspace through the internal-service path is a broader change.
func cronWorkspaceID(ctx context.Context) string {
	ws := ""
	if execCtx := shared.GetExecutionContext(ctx); execCtx != nil {
		ws = execCtx.WorkspaceID
	}
	if ws == "" || ws == "default" {
		return domain.DefaultWorkspaceID
	}
	return ws
}

// handleCronJobs handles GET (list) and POST (create) for cron jobs
func (cs *ChatServer) handleCronJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		// Listing scheduled jobs requires authentication.
		if _, ok := requireAuth(w, r); !ok {
			return
		}
		jobs, err := cs.cronRepo.List(ctx, cronWorkspaceID(ctx))
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"jobs": jobs})

	case http.MethodPost:
		// Creating a scheduled job runs an agent on a timer — admin only.
		if !requireAdmin(w, r, cs.authzService) {
			return
		}
		var req domain.CronJob
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.ID == "" {
			http.Error(w, `{"error": "id is required"}`, http.StatusBadRequest)
			return
		}
		if req.Schedule == "" {
			http.Error(w, `{"error": "schedule is required"}`, http.StatusBadRequest)
			return
		}
		// Reject an unschedulable expression up front — a job that can never
		// fire must not be stored (it would sit in `cron list` forever, doing
		// nothing). This is distinct from the valid-but-scheduler-down case
		// below, which stores and warns.
		if err := cron.ValidateSchedule(req.Schedule); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), http.StatusBadRequest)
			return
		}
		req.WorkspaceID = cronWorkspaceID(ctx)
		// THE HANDLER OWNS THE ROW; THE SCHEDULER OWNS THE TIMER.
		//
		// Only this code knows the job's workspace: the scheduler's store is
		// built with an empty one, so persisting through the scheduler writes
		// the job under no workspace and `cron list` — which queries by
		// workspace — never sees it.
		//
		// So: persist here, then schedule. A job that is stored but not
		// scheduled says so, rather than letting the caller believe a schedule
		// nothing is executing (2026-08-24: jobs were stored, listed, and
		// never fired).
		if err := cs.cronRepo.Create(ctx, &req); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
			return
		}
		scheduled, warning := false, "no scheduler running — this job starts at the next gateway restart"
		if cs.onCronJobAdded != nil {
			if err := cs.onCronJobAdded(&req); err != nil {
				warning = fmt.Sprintf("stored but NOT scheduled: %v", err)
			} else {
				scheduled, warning = true, ""
			}
		}
		w.WriteHeader(http.StatusCreated)
		out := map[string]interface{}{"job": req, "scheduled": scheduled}
		if warning != "" {
			out["warning"] = warning
		}
		json.NewEncoder(w).Encode(out)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCronJob handles DELETE for individual cron jobs: /api/cron/{id}
func (cs *ChatServer) handleCronJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	jobID := strings.TrimPrefix(r.URL.Path, "/api/cron/")
	if jobID == "" {
		http.Error(w, `{"error": "job ID is required"}`, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Reading a scheduled job requires authentication.
		if _, ok := requireAuth(w, r); !ok {
			return
		}
		job, err := cs.cronRepo.GetByID(ctx, jobID)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(job)

	case http.MethodDelete:
		if !requireAdmin(w, r, cs.authzService) {
			return
		}
		// Stop the timer first, then delete the row: the reverse order can
		// fire a job whose row is already gone.
		if cs.onCronJobRemoved != nil {
			_ = cs.onCronJobRemoved(jobID)
		}
		if err := cs.cronRepo.Delete(ctx, jobID); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": jobID})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCronHistory handles GET /api/cron-history?job_id=X&limit=N
func (cs *ChatServer) handleCronHistory(w http.ResponseWriter, r *http.Request) {
	log := getChatLogger()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Reading cron run history requires authentication.
	if _, ok := requireAuth(w, r); !ok {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	jobID := r.URL.Query().Get("job_id")
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := fmt.Sscanf(l, "%d", &limit); n != 1 || err != nil {
			limit = 10
		}
	}

	log.Debug("Cron history request",
		slog.String("job_id", jobID),
		slog.Int("limit", limit))

	var records []*domain.CronRunRecord
	var err error
	if jobID != "" {
		records, err = cs.cronHistoryRepo.ListByJob(ctx, jobID, limit)
	} else {
		records, err = cs.cronHistoryRepo.ListRecent(ctx, limit)
	}
	if err != nil {
		log.WithError(err).Warn("Cron history query failed")
		http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
		return
	}

	log.Debug("Cron history response",
		slog.Int("count", len(records)))

	json.NewEncoder(w).Encode(map[string]interface{}{"records": records})
}

// handleCronStats handles GET /api/cron-stats?job_id=X
func (cs *ChatServer) handleCronStats(w http.ResponseWriter, r *http.Request) {
	log := getChatLogger()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Reading cron stats requires authentication.
	if _, ok := requireAuth(w, r); !ok {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	jobID := r.URL.Query().Get("job_id")

	log.Debug("Cron stats request",
		slog.String("job_id", jobID))

	// Get all history records and compute stats
	var records []*domain.CronRunRecord
	var err error
	if jobID != "" {
		records, err = cs.cronHistoryRepo.ListByJob(ctx, jobID, 0)
	} else {
		records, err = cs.cronHistoryRepo.ListRecent(ctx, 0)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
		return
	}

	// Compute stats per job
	type jobStats struct {
		JobID        string  `json:"jobId"`
		TotalRuns    int     `json:"totalRuns"`
		SuccessCount int     `json:"successCount"`
		FailureCount int     `json:"failureCount"`
		SuccessRate  float64 `json:"successRate"`
		AvgDuration  int64   `json:"avgDuration"`
		LastRun      string  `json:"lastRun,omitempty"`
		LastSuccess  bool    `json:"lastSuccess"`
	}

	statsMap := make(map[string]*jobStats)
	for _, rec := range records {
		s, ok := statsMap[rec.JobID]
		if !ok {
			s = &jobStats{JobID: rec.JobID}
			statsMap[rec.JobID] = s
		}
		s.TotalRuns++
		if rec.Success {
			s.SuccessCount++
		} else {
			s.FailureCount++
		}
		s.AvgDuration += rec.DurationMs

		if s.LastRun == "" || rec.EndTime.After(parseTime(s.LastRun)) {
			s.LastRun = rec.EndTime.Format(time.RFC3339)
			s.LastSuccess = rec.Success
		}
	}

	allStats := make([]jobStats, 0, len(statsMap))
	for _, s := range statsMap {
		if s.TotalRuns > 0 {
			s.AvgDuration = s.AvgDuration / int64(s.TotalRuns)
			s.SuccessRate = float64(s.SuccessCount) / float64(s.TotalRuns) * 100
		}
		allStats = append(allStats, *s)
	}

	log.Debug("Cron stats response",
		slog.Int("jobs", len(allStats)),
		slog.Int("total_records", len(records)))

	json.NewEncoder(w).Encode(map[string]interface{}{"stats": allStats})
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// wireCronHooks connects the HTTP handlers to the running scheduler.
//
// The handlers live on chatServer, which does not import the cron package;
// the gateway owns the scheduler and lends it in. Same shape as the resume and
// retarget hooks: the side that HAS the capability supplies it, rather than the
// side that needs it reaching across.
func (s *Server) wireCronHooks(cs *ChatServer) {
	if cs == nil || s.cronScheduler == nil {
		return
	}
	cs.onCronJobAdded = func(j *domain.CronJob) error {
		return s.cronScheduler.ScheduleExisting(&config.CronJob{
			ID:       j.ID,
			Schedule: j.Schedule,
			AgentID:  j.AgentID,
			Message:  j.Message,
			Enabled:  j.Enabled,
			// Where to work, where to answer and the bounds travel too: a
			// scheduled workflow lost its project here and failed "names no
			// project directory" every minute (live 2026-10-04).
			Workdir:    j.Workdir,
			SessionKey: j.SessionKey,
			MaxRuns:    j.MaxRuns,
			RunCount:   j.RunCount,
			ExpiresAt:  j.ExpiresAt,
		})
	}
	cs.onCronJobRemoved = func(id string) error {
		return s.cronScheduler.UnscheduleExisting(id)
	}
}

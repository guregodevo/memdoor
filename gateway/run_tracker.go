package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/queue"
	"memdoor/gateway/subagents"
)

// RunStatus represents the status of an agent run
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusTimeout   RunStatus = "timeout"
)

// AgentRun represents an async agent execution
type AgentRun struct {
	RunID            string                 `json:"run_id"`
	SessionKey       string                 `json:"session_key"`
	Message          string                 `json:"message"`
	Status           RunStatus              `json:"status"`
	Response         *AgentResponse         `json:"response,omitempty"`
	Error            string                 `json:"error,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	StartedAt        *time.Time             `json:"started_at,omitempty"`
	CompletedAt      *time.Time             `json:"completed_at,omitempty"`
	RequesterSession string                 `json:"requester_session,omitempty"`
	AnnounceBack     bool                   `json:"announce_back"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// RunTracker tracks async agent runs
// Pattern: OpenClaw's run tracking for sessions_send/sessions_spawn
type RunTracker struct {
	runs    map[string]*AgentRun
	mu      sync.RWMutex
	log     *logs.EventLogger
	enqueue queue.JobEnqueuer // Use interface instead of concrete implementation
}

// NewRunTracker creates a new run tracker
func NewRunTracker(verbose bool, enqueuer queue.JobEnqueuer) *RunTracker {
	return &RunTracker{
		runs:    make(map[string]*AgentRun),
		log:     logs.New("Agent"),
		enqueue: enqueuer,
	}
}

// CreateRun creates a new agent run
func (rt *RunTracker) CreateRun(runID, sessionKey, message string, announceBack bool, requesterSession string) *AgentRun {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	run := &AgentRun{
		RunID:            runID,
		SessionKey:       sessionKey,
		Message:          message,
		Status:           RunStatusPending,
		CreatedAt:        time.Now(),
		RequesterSession: requesterSession,
		AnnounceBack:     announceBack,
		Metadata:         make(map[string]interface{}),
	}

	rt.runs[runID] = run

	rt.log.Debug("Created run",
		slog.String("run_id", runID),
		slog.String("session", sessionKey),
		slog.Bool("announce_back", announceBack),
		slog.String("requester", requesterSession))

	return run
}

// GetRun retrieves a run by ID
func (rt *RunTracker) GetRun(runID string) (*AgentRun, error) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	run, ok := rt.runs[runID]
	if !ok {
		return nil, fmt.Errorf("run not found: %s", runID)
	}

	return run, nil
}

// UpdateRunStatus updates the status of a run
func (rt *RunTracker) UpdateRunStatus(runID string, status RunStatus) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	run, ok := rt.runs[runID]
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}

	run.Status = status

	now := time.Now()
	switch status {
	case RunStatusRunning:
		run.StartedAt = &now
	case RunStatusCompleted, RunStatusFailed, RunStatusTimeout:
		run.CompletedAt = &now
	}

	rt.log.Debug("Run status updated",
		slog.String("run_id", runID),
		slog.String("status", string(status)))

	return nil
}

// SetRunResponse sets the response for a completed run
func (rt *RunTracker) SetRunResponse(runID string, response *AgentResponse) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	run, ok := rt.runs[runID]
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}

	run.Response = response
	run.Status = RunStatusCompleted
	now := time.Now()
	run.CompletedAt = &now

	rt.log.Debug("Run completed successfully",
		slog.String("run_id", runID))

	return nil
}

// SetRunError sets an error for a failed run
func (rt *RunTracker) SetRunError(runID string, err error) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	run, ok := rt.runs[runID]
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}

	run.Error = err.Error()
	run.Status = RunStatusFailed
	now := time.Now()
	run.CompletedAt = &now

	rt.log.WithError(err).Warn("Run failed",
		slog.String("run_id", runID))

	return nil
}

// CleanupRun removes a completed run after a delay
func (rt *RunTracker) CleanupRun(runID string, delay time.Duration) {
	time.AfterFunc(delay, func() {
		rt.mu.Lock()
		defer rt.mu.Unlock()

		delete(rt.runs, runID)

		rt.log.Debug("Cleaned up run",
			slog.String("run_id", runID))
	})
}

// ExecuteAsync executes an agent run asynchronously
// Pattern: OpenClaw's async agent execution
// Routes through queue to prevent race conditions on same session
func (rt *RunTracker) ExecuteAsync(
	ctx context.Context,
	runtime *AgentRuntime,
	sessionMgr *SessionManager,
	run *AgentRun,
) {
	// Mark as running
	if err := rt.UpdateRunStatus(run.RunID, RunStatusRunning); err != nil {
		rt.log.WithError(err).Warn("Failed to update run status")
		return
	}

	go func() {
		// Enqueue A2A subagent execution through queue for per-session serialization
		// This prevents concurrent executions on the same session
		responseChan := make(chan *AgentResponse, 1)
		errorChan := make(chan error, 1)

		responseWriter := func(resp interface{}) error {
			// Handle both success (*AgentResponse) and error (error) responses
			if agentResp, ok := resp.(*AgentResponse); ok {
				responseChan <- agentResp
			} else if err, ok := resp.(error); ok {
				errorChan <- err
			} else {
				errorChan <- fmt.Errorf("invalid response type: %T", resp)
			}
			return nil
		}

		queueJob := &queue.AgentJob{
			SessionKey:     run.SessionKey,
			Message:        run.Message,
			GlobalLane:     queue.LaneMain,
			EnqueueTime:    time.Now(),
			Context:        ctx,
			ResponseWriter: responseWriter,
		}

		if err := rt.enqueue.EnqueueJob(queueJob); err != nil {
			rt.log.WithError(err).Warn("Failed to enqueue A2A subagent execution",
				slog.String("run_id", run.RunID))
			rt.SetRunError(run.RunID, err)
			return
		}

		// Wait for response from queue execution
		var response *AgentResponse
		select {
		case response = <-responseChan:
			// Success
		case err := <-errorChan:
			rt.SetRunError(run.RunID, err)
			return
		case <-time.After(jobWaitTimeout()):
			rt.log.Warn("A2A subagent execution timeout", slog.String("run_id", run.RunID))
			rt.SetRunError(run.RunID, fmt.Errorf("execution timed out"))
			return
		}

		// Store response
		if err := rt.SetRunResponse(run.RunID, response); err != nil {
			rt.log.WithError(err).Warn("Failed to set run response")
			return
		}

		// Announce back to requester if requested
		if run.AnnounceBack && run.RequesterSession != "" {
			rt.announceResult(ctx, runtime, sessionMgr, run, response)
		}

		// Clean up after 5 minutes
		rt.CleanupRun(run.RunID, 5*time.Minute)
	}()
}

// WaitForRun waits for a run to complete with a timeout
// Returns nil if timeout occurs, otherwise returns the run snapshot
// Pattern: OpenClaw src/gateway/server-methods/agent-job.ts - waitForAgentJob
func (rt *RunTracker) WaitForRun(runID string, timeout time.Duration) *subagents.RunSnapshot {
	deadline := time.Now().Add(timeout)
	pollInterval := 100 * time.Millisecond

	for time.Now().Before(deadline) {
		rt.mu.RLock()
		run, exists := rt.runs[runID]
		rt.mu.RUnlock()

		if !exists {
			// Run not found - return error status
			return &subagents.RunSnapshot{
				Status: "error",
				Error:  "run not found",
			}
		}

		// Check if run is complete
		if run.Status == RunStatusCompleted || run.Status == RunStatusFailed || run.Status == RunStatusTimeout {
			status := "ok"
			if run.Status == RunStatusFailed || run.Status == RunStatusTimeout {
				status = "error"
			}

			return &subagents.RunSnapshot{
				Status:    status,
				StartedAt: run.StartedAt,
				EndedAt:   run.CompletedAt,
				Error:     run.Error,
			}
		}

		// Poll again after interval
		time.Sleep(pollInterval)
	}

	// Timeout
	return nil
}

// announceResult announces the result back to the requester session
// Pattern: OpenClaw's announce-back mechanism
// Routes through queue to prevent race conditions on same session
func (rt *RunTracker) announceResult(
	ctx context.Context,
	runtime *AgentRuntime,
	sessionMgr *SessionManager,
	run *AgentRun,
	response *AgentResponse,
) {
	// Format announcement message
	announcement := fmt.Sprintf(
		"[Announce] Run %s completed for session %s:\n\n%s",
		run.RunID,
		run.SessionKey,
		response.Text,
	)

	// Create a new run for the announcement (no further announce-back)
	announceRunID := run.RunID + "-announce"
	_ = rt.CreateRun(announceRunID, run.RequesterSession, announcement, false, "")
	rt.UpdateRunStatus(announceRunID, RunStatusRunning)

	// Enqueue A2A announcement through queue for per-session serialization
	// This prevents concurrent executions on the same session
	responseChan := make(chan *AgentResponse, 1)
	errorChan := make(chan error, 1)

	responseWriter := func(resp interface{}) error {
		// Handle both success (*AgentResponse) and error (error) responses
		if agentResp, ok := resp.(*AgentResponse); ok {
			responseChan <- agentResp
		} else if err, ok := resp.(error); ok {
			errorChan <- err
		} else {
			errorChan <- fmt.Errorf("invalid response type: %T", resp)
		}
		return nil
	}

	queueJob := &queue.AgentJob{
		SessionKey:     run.RequesterSession,
		Message:        announcement,
		GlobalLane:     queue.LaneMain,
		EnqueueTime:    time.Now(),
		Context:        ctx,
		ResponseWriter: responseWriter,
	}

	if err := rt.enqueue.EnqueueJob(queueJob); err != nil {
		rt.log.WithError(err).Warn("Failed to enqueue A2A announcement",
			slog.String("run_id", announceRunID))
		rt.SetRunError(announceRunID, err)
		return
	}

	// Wait for response from queue execution
	var announceResp *AgentResponse
	select {
	case announceResp = <-responseChan:
		// Success
	case err := <-errorChan:
		rt.SetRunError(announceRunID, err)
		rt.log.WithError(err).Warn("Failed to announce result")
		return
	case <-time.After(jobWaitTimeout()):
		rt.log.Warn("A2A announcement timeout", slog.String("run_id", announceRunID))
		rt.SetRunError(announceRunID, fmt.Errorf("announcement timed out"))
		return
	}

	rt.SetRunResponse(announceRunID, announceResp)
	rt.CleanupRun(announceRunID, 1*time.Minute)

	rt.log.Debug("Announced result to requester",
		slog.String("run_id", run.RunID),
		slog.String("requester", run.RequesterSession))
}

package rpc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/queue"
	"memdoor/gateway/subagents"
)

// QueueManager interface for testing
type QueueManager interface {
	EnqueueJob(job *queue.AgentJob) error
}

// AgentHandler implements the "agent" RPC method
// Pattern: OpenClaw src/gateway/server-methods/agent.ts - agent handler
//
// This method:
// 1. Validates params
// 2. Returns immediately with runID and "accepted" status
// 3. Queues the job for execution (non-blocking)
// 4. Job executes in background via queue
//
// Response format:
//
//	{
//	  "run_id": "run-abc123",
//	  "status": "accepted",
//	  "accepted_at": 1234567890
//	}
func AgentHandler(params AgentParams, queueManager QueueManager, verbose bool) (AgentResult, error) {
	log := logs.New("RPC")

	// Validate required fields
	if params.Message == "" {
		return AgentResult{}, fmt.Errorf("message is required")
	}
	if params.SessionKey == "" {
		return AgentResult{}, fmt.Errorf("session_key is required")
	}
	if params.IdempotencyKey == "" {
		return AgentResult{}, fmt.Errorf("idempotency_key is required")
	}

	// Generate run ID (use idempotency key as run ID)
	runID := params.IdempotencyKey

	// TODO: Check for duplicate run (idempotency)
	// For now, accept all requests

	log.Debug("Creating run",
		slog.String("run_id", runID),
		slog.String("session", params.SessionKey),
		slog.String("lane", params.Lane))

	// Determine global lane from request
	// Pattern: OpenClaw lane selection
	globalLane := queue.LaneMain
	if params.Lane == "cron" {
		globalLane = queue.LaneCron
	} else if params.Lane == "subagent" {
		globalLane = queue.LaneSubagent
	} else if params.Lane == "nested" {
		globalLane = queue.LaneNested
	}

	// Enqueue job for execution (non-blocking)
	// Pattern: OpenClaw's enqueueCommandInLane()
	job := &queue.AgentJob{
		SessionKey:  params.SessionKey,
		Message:     params.Message,
		GlobalLane:  globalLane,
		EnqueueTime: time.Now(),
		Context:     context.Background(),
		// ResponseWriter is nil for RPC calls - responses come via events
	}

	err := queueManager.EnqueueJob(job)
	if err != nil {
		log.WithError(err).Warn("Failed to enqueue job")
		return AgentResult{}, fmt.Errorf("failed to enqueue job: %w", err)
	}

	// Return immediately with "accepted" status
	// Pattern: OpenClaw agent handler line 349-360
	result := AgentResult{
		RunID:      runID,
		Status:     "accepted",
		AcceptedAt: time.Now().UnixMilli(),
	}

	log.Debug("Run accepted and queued",
		slog.String("run_id", runID))

	return result, nil
}

// AgentWaitHandler implements the "agent.wait" RPC method
// Pattern: OpenClaw src/gateway/server-methods/agent.ts - agent.wait handler
//
// This method:
// 1. Waits for a run to complete (blocking)
// 2. Returns run status when complete or timeout
// 3. Default timeout: 30s
//
// Response format:
//
//	{
//	  "run_id": "run-abc123",
//	  "status": "ok",  // or "error", "timeout"
//	  "started_at": "2026-02-18T...",
//	  "ended_at": "2026-02-18T...",
//	  "error": "error message if status=error"
//	}
func AgentWaitHandler(params AgentWaitParams, runTracker RunTracker, verbose bool) (AgentWaitResult, error) {
	log := logs.New("RPC")

	// Validate required fields
	if params.RunID == "" {
		return AgentWaitResult{}, fmt.Errorf("run_id is required")
	}

	// Default timeout: 30s (OpenClaw pattern line 492-494)
	timeoutMs := params.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 30000 // 30s default
	}

	log.Debug("Waiting for run",
		slog.String("run_id", params.RunID),
		slog.Int("timeout_ms", timeoutMs))

	// Wait for run to complete
	snapshot := runTracker.WaitForRun(params.RunID, time.Duration(timeoutMs)*time.Millisecond)

	if snapshot == nil {
		// Timeout
		log.Debug("Run timed out",
			slog.String("run_id", params.RunID))
		return AgentWaitResult{
			RunID:  params.RunID,
			Status: "timeout",
		}, nil
	}

	// Run completed
	result := AgentWaitResult{
		RunID:     params.RunID,
		Status:    snapshot.Status,
		StartedAt: snapshot.StartedAt,
		EndedAt:   snapshot.EndedAt,
	}

	if snapshot.Error != "" {
		result.Error = snapshot.Error
	}

	log.Debug("Run completed",
		slog.String("run_id", params.RunID),
		slog.String("status", snapshot.Status))

	return result, nil
}

// RunTracker defines the interface for tracking agent runs
// This allows us to wait for runs to complete
// Pattern: Interface-based dependency injection (OpenClaw pattern)
type RunTracker interface {
	WaitForRun(runID string, timeout time.Duration) *subagents.RunSnapshot
}

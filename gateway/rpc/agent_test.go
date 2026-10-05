package rpc

import (
	"errors"
	"testing"
	"time"

	"memdoor/gateway/queue"
	"memdoor/gateway/subagents"
)

// MockQueueManager for testing
type MockQueueManager struct {
	enqueuedJobs []*queue.AgentJob
	shouldFail   bool
}

func (m *MockQueueManager) EnqueueJob(job *queue.AgentJob) error {
	if m.shouldFail {
		return errors.New("queue full")
	}
	m.enqueuedJobs = append(m.enqueuedJobs, job)
	return nil
}

func (m *MockQueueManager) GetQueueDepth() int {
	return len(m.enqueuedJobs)
}

func (m *MockQueueManager) Start() {}
func (m *MockQueueManager) Stop()  {}

// MockRunTracker for testing
type MockRunTracker struct {
	runs map[string]*subagents.RunSnapshot
}

func (m *MockRunTracker) WaitForRun(runID string, timeout time.Duration) *subagents.RunSnapshot {
	if snapshot, ok := m.runs[runID]; ok {
		return snapshot
	}
	// Simulate timeout
	time.Sleep(10 * time.Millisecond)
	return nil
}

func TestAgentHandler_Success(t *testing.T) {
	mockQueue := &MockQueueManager{
		enqueuedJobs: make([]*queue.AgentJob, 0),
	}

	params := AgentParams{
		Message:        "test message",
		SessionKey:     "test-session",
		IdempotencyKey: "test-run-123",
		Lane:           "main",
	}

	result, err := AgentHandler(params, mockQueue, false)

	// Should succeed
	if err != nil {
		t.Fatalf("AgentHandler failed: %v", err)
	}

	// Check result
	if result.RunID != "test-run-123" {
		t.Errorf("Expected runID 'test-run-123', got '%s'", result.RunID)
	}

	if result.Status != "accepted" {
		t.Errorf("Expected status 'accepted', got '%s'", result.Status)
	}

	if result.AcceptedAt == 0 {
		t.Error("Expected AcceptedAt to be set")
	}

	// Check job was enqueued
	if len(mockQueue.enqueuedJobs) != 1 {
		t.Fatalf("Expected 1 enqueued job, got %d", len(mockQueue.enqueuedJobs))
	}

	job := mockQueue.enqueuedJobs[0]
	if job.SessionKey != "test-session" {
		t.Errorf("Expected sessionKey 'test-session', got '%s'", job.SessionKey)
	}

	if job.Message != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", job.Message)
	}

	if job.GlobalLane != queue.LaneMain {
		t.Errorf("Expected lane LaneMain, got %s", job.GlobalLane)
	}
}

func TestAgentHandler_MissingMessage(t *testing.T) {
	mockQueue := &MockQueueManager{}

	params := AgentParams{
		SessionKey:     "test-session",
		IdempotencyKey: "test-run-123",
	}

	_, err := AgentHandler(params, mockQueue, false)

	if err == nil {
		t.Fatal("Expected error for missing message")
	}

	if err.Error() != "message is required" {
		t.Errorf("Expected 'message is required' error, got '%s'", err.Error())
	}
}

func TestAgentHandler_MissingSessionKey(t *testing.T) {
	mockQueue := &MockQueueManager{}

	params := AgentParams{
		Message:        "test message",
		IdempotencyKey: "test-run-123",
	}

	_, err := AgentHandler(params, mockQueue, false)

	if err == nil {
		t.Fatal("Expected error for missing session_key")
	}

	if err.Error() != "session_key is required" {
		t.Errorf("Expected 'session_key is required' error, got '%s'", err.Error())
	}
}

func TestAgentHandler_MissingIdempotencyKey(t *testing.T) {
	mockQueue := &MockQueueManager{}

	params := AgentParams{
		Message:    "test message",
		SessionKey: "test-session",
	}

	_, err := AgentHandler(params, mockQueue, false)

	if err == nil {
		t.Fatal("Expected error for missing idempotency_key")
	}

	if err.Error() != "idempotency_key is required" {
		t.Errorf("Expected 'idempotency_key is required' error, got '%s'", err.Error())
	}
}

func TestAgentHandler_QueueFull(t *testing.T) {
	mockQueue := &MockQueueManager{
		shouldFail: true,
	}

	params := AgentParams{
		Message:        "test message",
		SessionKey:     "test-session",
		IdempotencyKey: "test-run-123",
	}

	_, err := AgentHandler(params, mockQueue, false)

	if err == nil {
		t.Fatal("Expected error when queue is full")
	}
}

func TestAgentHandler_DifferentLanes(t *testing.T) {
	testCases := []struct {
		lane     string
		expected queue.Lane
	}{
		{"main", queue.LaneMain},
		{"cron", queue.LaneCron},
		{"subagent", queue.LaneSubagent},
		{"nested", queue.LaneNested},
		{"unknown", queue.LaneMain}, // defaults to main
		{"", queue.LaneMain},        // defaults to main
	}

	for _, tc := range testCases {
		t.Run("lane_"+tc.lane, func(t *testing.T) {
			mockQueue := &MockQueueManager{
				enqueuedJobs: make([]*queue.AgentJob, 0),
			}

			params := AgentParams{
				Message:        "test message",
				SessionKey:     "test-session",
				IdempotencyKey: "test-run-" + tc.lane,
				Lane:           tc.lane,
			}

			_, err := AgentHandler(params, mockQueue, false)
			if err != nil {
				t.Fatalf("AgentHandler failed: %v", err)
			}

			job := mockQueue.enqueuedJobs[0]
			if job.GlobalLane != tc.expected {
				t.Errorf("Expected lane %s, got %s", tc.expected, job.GlobalLane)
			}
		})
	}
}

func TestAgentWaitHandler_Timeout(t *testing.T) {
	mockTracker := &MockRunTracker{
		runs: make(map[string]*subagents.RunSnapshot),
	}

	params := AgentWaitParams{
		RunID:     "nonexistent-run",
		TimeoutMs: 50, // 50ms timeout
	}

	result, err := AgentWaitHandler(params, mockTracker, false)

	if err != nil {
		t.Fatalf("AgentWaitHandler failed: %v", err)
	}

	if result.Status != "timeout" {
		t.Errorf("Expected status 'timeout', got '%s'", result.Status)
	}

	if result.RunID != "nonexistent-run" {
		t.Errorf("Expected runID 'nonexistent-run', got '%s'", result.RunID)
	}
}

func TestAgentWaitHandler_Success(t *testing.T) {
	startedAt := time.Now()
	endedAt := startedAt.Add(1 * time.Second)

	mockTracker := &MockRunTracker{
		runs: map[string]*subagents.RunSnapshot{
			"test-run-123": {
				Status:    "ok",
				StartedAt: &startedAt,
				EndedAt:   &endedAt,
			},
		},
	}

	params := AgentWaitParams{
		RunID:     "test-run-123",
		TimeoutMs: 1000,
	}

	result, err := AgentWaitHandler(params, mockTracker, false)

	if err != nil {
		t.Fatalf("AgentWaitHandler failed: %v", err)
	}

	if result.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", result.Status)
	}

	if result.RunID != "test-run-123" {
		t.Errorf("Expected runID 'test-run-123', got '%s'", result.RunID)
	}

	if result.StartedAt == nil {
		t.Error("Expected StartedAt to be set")
	}

	if result.EndedAt == nil {
		t.Error("Expected EndedAt to be set")
	}
}

func TestAgentWaitHandler_Error(t *testing.T) {
	startedAt := time.Now()
	endedAt := startedAt.Add(1 * time.Second)

	mockTracker := &MockRunTracker{
		runs: map[string]*subagents.RunSnapshot{
			"test-run-error": {
				Status:    "error",
				StartedAt: &startedAt,
				EndedAt:   &endedAt,
				Error:     "test error message",
			},
		},
	}

	params := AgentWaitParams{
		RunID:     "test-run-error",
		TimeoutMs: 1000,
	}

	result, err := AgentWaitHandler(params, mockTracker, false)

	if err != nil {
		t.Fatalf("AgentWaitHandler failed: %v", err)
	}

	if result.Status != "error" {
		t.Errorf("Expected status 'error', got '%s'", result.Status)
	}

	if result.Error != "test error message" {
		t.Errorf("Expected error 'test error message', got '%s'", result.Error)
	}
}

func TestAgentWaitHandler_MissingRunID(t *testing.T) {
	mockTracker := &MockRunTracker{
		runs: make(map[string]*subagents.RunSnapshot),
	}

	params := AgentWaitParams{
		TimeoutMs: 1000,
	}

	_, err := AgentWaitHandler(params, mockTracker, false)

	if err == nil {
		t.Fatal("Expected error for missing run_id")
	}

	if err.Error() != "run_id is required" {
		t.Errorf("Expected 'run_id is required' error, got '%s'", err.Error())
	}
}

func TestAgentWaitHandler_DefaultTimeout(t *testing.T) {
	mockTracker := &MockRunTracker{
		runs: make(map[string]*subagents.RunSnapshot),
	}

	params := AgentWaitParams{
		RunID: "test-run",
		// TimeoutMs not specified, should default to 30000ms
	}

	start := time.Now()
	result, err := AgentWaitHandler(params, mockTracker, false)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("AgentWaitHandler failed: %v", err)
	}

	if result.Status != "timeout" {
		t.Errorf("Expected status 'timeout', got '%s'", result.Status)
	}

	// Should timeout but we only wait 10ms in mock, so duration should be < 100ms
	if duration > 100*time.Millisecond {
		t.Errorf("Expected quick timeout, got %v", duration)
	}
}

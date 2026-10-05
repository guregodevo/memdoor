package subagents

import (
	"fmt"
	"testing"
	"time"

	"memdoor/gateway/infra"
)

func TestNewSubagentRegistry(t *testing.T) {
	registry := newTestRegistry(t)
	if registry == nil {
		t.Fatal("Registry is nil")
	}

	all := registry.ListAll()
	if len(all) != 0 {
		t.Errorf("Expected empty registry, got %d runs", len(all))
	}
}

func TestRegisterAndGet(t *testing.T) {
	registry := newTestRegistry(t)

	record := &SubagentRunRecord{
		RunID:               "run-123",
		ChildSessionKey:     "agent:main:subagent:abc",
		RequesterSessionKey: "agent:main:main",
		RequesterDisplayKey: "Main Agent",
		Task:                "Search documentation",
		Cleanup:             "delete",
		Label:               "Doc Search",
		CreatedAt:           time.Now(),
	}

	if err := registry.Register(record); err != nil {
		t.Fatalf("Failed to register: %v", err)
	}

	retrieved, exists := registry.Get("run-123")
	if !exists {
		t.Fatal("Run not found after registration")
	}

	if retrieved.RunID != "run-123" {
		t.Errorf("Expected runID run-123, got %s", retrieved.RunID)
	}

	if retrieved.Task != "Search documentation" {
		t.Errorf("Expected task 'Search documentation', got %s", retrieved.Task)
	}
}

func TestRegisterEmptyRunID(t *testing.T) {
	registry := newTestRegistry(t)

	record := &SubagentRunRecord{
		RunID: "",
		Task:  "Test task",
	}

	// Empty runID should be a no-op (not an error)
	err := registry.Register(record)
	if err != nil {
		t.Errorf("Expected no error for empty runID, got %v", err)
	}
}

func TestOnLifecycleEvent_Start(t *testing.T) {
	registry := newTestRegistry(t)

	record := &SubagentRunRecord{
		RunID:           "run-123",
		ChildSessionKey: "agent:main:subagent:abc",
		Task:            "Test task",
		CreatedAt:       time.Now(),
	}
	registry.Register(record)

	event := &infra.AgentEvent{
		SessionID: "agent:main:subagent:abc",
		Data: map[string]interface{}{
			"event": "start",
		},
	}
	registry.OnLifecycleEvent(event)

	retrieved, _ := registry.Get("run-123")
	if retrieved.StartedAt == nil {
		t.Error("StartedAt should be set after start event")
	}
}

func TestOnLifecycleEvent_End(t *testing.T) {
	registry := newTestRegistry(t)

	record := &SubagentRunRecord{
		RunID:           "run-123",
		ChildSessionKey: "agent:main:subagent:abc",
		Task:            "Test task",
		CreatedAt:       time.Now(),
	}
	registry.Register(record)

	event := &infra.AgentEvent{
		SessionID: "agent:main:subagent:abc",
		Data: map[string]interface{}{
			"event": "end",
		},
	}
	registry.OnLifecycleEvent(event)

	retrieved, _ := registry.Get("run-123")
	if retrieved.EndedAt == nil {
		t.Error("EndedAt should be set after end event")
	}
	if retrieved.Outcome == nil {
		t.Error("Outcome should be set after end event")
	}
	if retrieved.Outcome != nil && retrieved.Outcome.Status != "ok" {
		t.Errorf("Expected outcome status 'ok', got %s", retrieved.Outcome.Status)
	}
}

func TestOnLifecycleEvent_Error(t *testing.T) {
	registry := newTestRegistry(t)

	record := &SubagentRunRecord{
		RunID:           "run-123",
		ChildSessionKey: "agent:main:subagent:abc",
		Task:            "Test task",
		CreatedAt:       time.Now(),
	}
	registry.Register(record)

	event := &infra.AgentEvent{
		SessionID: "agent:main:subagent:abc",
		Data: map[string]interface{}{
			"event":   "error",
			"message": "Task failed",
		},
	}
	registry.OnLifecycleEvent(event)

	retrieved, _ := registry.Get("run-123")
	if retrieved.EndedAt == nil {
		t.Error("EndedAt should be set after error event")
	}
	if retrieved.Outcome == nil {
		t.Error("Outcome should be set after error event")
	}
	if retrieved.Outcome != nil {
		if retrieved.Outcome.Status != "error" {
			t.Errorf("Expected outcome status 'error', got %s", retrieved.Outcome.Status)
		}
		if retrieved.Outcome.Error != "Task failed" {
			t.Errorf("Expected error 'Task failed', got %s", retrieved.Outcome.Error)
		}
	}
}

func TestSweep(t *testing.T) {
	registry := newTestRegistry(t)

	now := time.Now()
	endedTime := now.Add(-1 * time.Hour)

	record1 := &SubagentRunRecord{
		RunID:           "run-1",
		ChildSessionKey: "agent:main:subagent:1",
		Task:            "Task 1",
		CreatedAt:       now.Add(-2 * time.Hour),
		EndedAt:         &endedTime,
		ArchiveAtMs:     now.Add(-30 * time.Minute).UnixMilli(),
	}

	record2 := &SubagentRunRecord{
		RunID:           "run-2",
		ChildSessionKey: "agent:main:subagent:2",
		Task:            "Task 2",
		CreatedAt:       now.Add(-1 * time.Hour),
		EndedAt:         &endedTime,
		ArchiveAtMs:     now.Add(1 * time.Hour).UnixMilli(),
	}

	record3 := &SubagentRunRecord{
		RunID:           "run-3",
		ChildSessionKey: "agent:main:subagent:3",
		Task:            "Task 3",
		CreatedAt:       now,
		EndedAt:         nil,
		ArchiveAtMs:     now.Add(-1 * time.Hour).UnixMilli(),
	}

	registry.Register(record1)
	registry.Register(record2)
	registry.Register(record3)

	all := registry.ListAll()
	if len(all) != 3 {
		t.Errorf("Expected 3 runs before sweep, got %d", len(all))
	}

	registry.Sweep()

	// After sweep: run-1 should be removed (archive_at_ms in past and > 0)
	// run-3 has archive_at_ms in past too, so it's also removed by DeleteArchived
	_, exists1 := registry.Get("run-1")
	if exists1 {
		t.Error("run-1 should have been swept (archived)")
	}

	_, exists2 := registry.Get("run-2")
	if !exists2 {
		t.Error("run-2 should NOT have been swept (archive time in future)")
	}
}

func TestUpdateOutcome(t *testing.T) {
	registry := newTestRegistry(t)

	record := &SubagentRunRecord{
		RunID:           "run-123",
		ChildSessionKey: "agent:main:subagent:abc",
		Task:            "Test task",
		CreatedAt:       time.Now(),
	}
	registry.Register(record)

	outcome := &SubagentOutcome{
		Status: "timeout",
		Error:  "Task timed out after 5 minutes",
	}
	err := registry.UpdateOutcome("run-123", outcome)
	if err != nil {
		t.Fatalf("Failed to update outcome: %v", err)
	}

	retrieved, _ := registry.Get("run-123")
	if retrieved.Outcome == nil {
		t.Fatal("Outcome should be set")
	}
	if retrieved.Outcome.Status != "timeout" {
		t.Errorf("Expected status 'timeout', got %s", retrieved.Outcome.Status)
	}
	if retrieved.Outcome.Error != "Task timed out after 5 minutes" {
		t.Errorf("Expected error message, got %s", retrieved.Outcome.Error)
	}
}

func TestMarkCleanupHandled(t *testing.T) {
	registry := newTestRegistry(t)

	record := &SubagentRunRecord{
		RunID:           "run-123",
		ChildSessionKey: "agent:main:subagent:abc",
		Task:            "Test task",
		CreatedAt:       time.Now(),
		CleanupHandled:  false,
	}
	registry.Register(record)

	err := registry.MarkCleanupHandled("run-123")
	if err != nil {
		t.Fatalf("Failed to mark cleanup handled: %v", err)
	}

	retrieved, _ := registry.Get("run-123")
	if !retrieved.CleanupHandled {
		t.Error("CleanupHandled should be true")
	}
	if retrieved.CleanupCompletedAt == nil {
		t.Error("CleanupCompletedAt should be set")
	}
}

func TestListAll(t *testing.T) {
	registry := newTestRegistry(t)

	for i := 1; i <= 5; i++ {
		record := &SubagentRunRecord{
			RunID:           fmt.Sprintf("run-%d", i),
			ChildSessionKey: fmt.Sprintf("agent:main:subagent:%d", i),
			Task:            fmt.Sprintf("Task %d", i),
			CreatedAt:       time.Now(),
		}
		registry.Register(record)
	}

	all := registry.ListAll()
	if len(all) != 5 {
		t.Errorf("Expected 5 runs, got %d", len(all))
	}
}

func TestIsSubagentSession(t *testing.T) {
	tests := []struct {
		sessionID string
		expected  bool
	}{
		{"agent:main:subagent:abc123", true},
		{"agent:main:subagent:xyz", true},
		{"agent:main:main", false},
		{"agent:main:cron:job1", false},
		{"random-session-id", false},
		{"", false},
	}

	for _, test := range tests {
		result := isSubagentSession(test.sessionID)
		if result != test.expected {
			t.Errorf("isSubagentSession(%q) = %v, expected %v", test.sessionID, result, test.expected)
		}
	}
}

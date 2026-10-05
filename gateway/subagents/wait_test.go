package subagents

import (
	"testing"
	"time"

	"memdoor/gateway/infra"
)

func TestWaitForRun_Success(t *testing.T) {
	registry := newTestRegistry(t)

	runID := "test-run-123"
	sessionKey := "agent:main:subagent:abc123"
	record := &SubagentRunRecord{
		RunID:               runID,
		ChildSessionKey:     sessionKey,
		RequesterSessionKey: "agent:main:main",
		Task:                "test task",
		Cleanup:             "delete",
		Label:               "test",
		CreatedAt:           time.Now(),
		ArchiveAtMs:         time.Now().Add(24 * time.Hour).UnixMilli(),
	}
	if err := registry.Register(record); err != nil {
		t.Fatalf("Failed to register run: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		registry.OnLifecycleEvent(&infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionKey,
			Data:      map[string]interface{}{"event": "start"},
		})

		time.Sleep(200 * time.Millisecond)
		registry.OnLifecycleEvent(&infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionKey,
			Data:      map[string]interface{}{"event": "end"},
		})
	}()

	snapshot := registry.WaitForRun(runID, 5*time.Second)
	if snapshot == nil {
		t.Fatal("Expected snapshot, got nil (timeout)")
	}
	if snapshot.Status != "ok" {
		t.Errorf("Expected status 'ok', got %q", snapshot.Status)
	}
	if snapshot.StartedAt == nil {
		t.Error("Expected StartedAt to be set")
	}
	if snapshot.EndedAt == nil {
		t.Error("Expected EndedAt to be set")
	}
}

func TestWaitForRun_Error(t *testing.T) {
	registry := newTestRegistry(t)

	runID := "test-run-error-456"
	sessionKey := "agent:main:subagent:def456"
	record := &SubagentRunRecord{
		RunID:               runID,
		ChildSessionKey:     sessionKey,
		RequesterSessionKey: "agent:main:main",
		Task:                "failing task",
		Cleanup:             "delete",
		CreatedAt:           time.Now(),
		ArchiveAtMs:         time.Now().Add(24 * time.Hour).UnixMilli(),
	}
	if err := registry.Register(record); err != nil {
		t.Fatalf("Failed to register run: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		registry.OnLifecycleEvent(&infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionKey,
			Data:      map[string]interface{}{"event": "error", "message": "simulated error"},
		})
	}()

	snapshot := registry.WaitForRun(runID, 3*time.Second)
	if snapshot == nil {
		t.Fatal("Expected snapshot, got nil (timeout)")
	}
	if snapshot.Status != "error" {
		t.Errorf("Expected status 'error', got %q", snapshot.Status)
	}
	if snapshot.Error != "simulated error" {
		t.Errorf("Expected error 'simulated error', got %q", snapshot.Error)
	}
}

func TestWaitForRun_Timeout(t *testing.T) {
	registry := newTestRegistry(t)

	runID := "test-run-timeout-789"
	record := &SubagentRunRecord{
		RunID:               runID,
		ChildSessionKey:     "agent:main:subagent:ghi789",
		RequesterSessionKey: "agent:main:main",
		Task:                "never completing task",
		Cleanup:             "delete",
		CreatedAt:           time.Now(),
	}
	if err := registry.Register(record); err != nil {
		t.Fatalf("Failed to register run: %v", err)
	}

	snapshot := registry.WaitForRun(runID, 500*time.Millisecond)
	if snapshot != nil {
		t.Errorf("Expected nil (timeout), got snapshot with status %q", snapshot.Status)
	}
}

func TestWaitForRun_NotFound(t *testing.T) {
	registry := newTestRegistry(t)

	snapshot := registry.WaitForRun("non-existent-run", 500*time.Millisecond)
	if snapshot != nil {
		t.Errorf("Expected nil for non-existent run, got snapshot with status %q", snapshot.Status)
	}
}

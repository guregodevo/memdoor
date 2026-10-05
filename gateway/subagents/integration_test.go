package subagents

import (
	"fmt"
	"testing"
	"time"

	"memdoor/gateway/infra"
)

// Helper function to create a start event
func createStartEvent(sessionKey, runID string) *infra.AgentEvent {
	return &infra.AgentEvent{
		SessionID: sessionKey,
		RunID:     runID,
		Stream:    infra.EventStreamLifecycle,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"event": "start",
		},
	}
}

// Helper function to create an end event
func createEndEvent(sessionKey, runID string) *infra.AgentEvent {
	return &infra.AgentEvent{
		SessionID: sessionKey,
		RunID:     runID,
		Stream:    infra.EventStreamLifecycle,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"event": "end",
		},
	}
}

// Helper function to create an error event
func createErrorEvent(sessionKey, runID, errorMsg string) *infra.AgentEvent {
	return &infra.AgentEvent{
		SessionID: sessionKey,
		RunID:     runID,
		Stream:    infra.EventStreamLifecycle,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"event":   "error",
			"message": errorMsg,
		},
	}
}

// TestSubagentErrorHandling tests how the system handles subagent errors
func TestSubagentErrorHandling(t *testing.T) {
	registry := newTestRegistry(t)

	// Simulate a subagent that encounters an error
	record := &SubagentRunRecord{
		RunID:               "run-error-test",
		ChildSessionKey:     "agent:main:subagent:error-abc",
		RequesterSessionKey: "agent:main:main",
		RequesterDisplayKey: "Main Agent",
		Task:                "Read nonexistent file /tmp/does-not-exist.txt",
		Cleanup:             "delete",
		Label:               "Error Test",
		CreatedAt:           time.Now(),
	}

	if err := registry.Register(record); err != nil {
		t.Fatalf("Failed to register run: %v", err)
	}

	// Simulate lifecycle events
	registry.OnLifecycleEvent(createStartEvent(record.ChildSessionKey, record.RunID))
	registry.OnLifecycleEvent(createErrorEvent(record.ChildSessionKey, record.RunID, "File not found: /tmp/does-not-exist.txt"))

	// Verify error was recorded
	retrieved, found := registry.Get(record.RunID)
	if !found {
		t.Fatal("Run record not found")
	}

	if retrieved.Outcome == nil {
		t.Fatal("Outcome is nil")
	}

	if retrieved.Outcome.Status != "error" {
		t.Errorf("Expected error status, got %s", retrieved.Outcome.Status)
	}

	if retrieved.Outcome.Error != "File not found: /tmp/does-not-exist.txt" {
		t.Errorf("Unexpected error message: %s", retrieved.Outcome.Error)
	}

	t.Log("✅ Error handling test passed - error correctly recorded in registry")
}

// TestKeepModeSessionPreservation tests that sessions are preserved when cleanup="keep"
func TestKeepModeSessionPreservation(t *testing.T) {
	registry := newTestRegistry(t)

	record := &SubagentRunRecord{
		RunID:               "run-keep-test",
		ChildSessionKey:     "agent:main:subagent:keep-abc",
		RequesterSessionKey: "agent:main:main",
		RequesterDisplayKey: "Main Agent",
		Task:                "Analyze codebase structure",
		Cleanup:             "keep", // Keep mode
		Label:               "Codebase Analysis",
		CreatedAt:           time.Now(),
	}

	if err := registry.Register(record); err != nil {
		t.Fatalf("Failed to register run: %v", err)
	}

	// Simulate lifecycle events
	registry.OnLifecycleEvent(createStartEvent(record.ChildSessionKey, record.RunID))
	registry.OnLifecycleEvent(createEndEvent(record.ChildSessionKey, record.RunID))

	// Verify cleanup mode is "keep"
	retrieved, found := registry.Get(record.RunID)
	if !found {
		t.Fatal("Run record not found")
	}

	if retrieved.Cleanup != "keep" {
		t.Errorf("Expected cleanup mode 'keep', got '%s'", retrieved.Cleanup)
	}

	t.Log("✅ Keep mode test passed - cleanup mode correctly set to 'keep'")
}

// TestConcurrentSubagents tests multiple subagents running simultaneously
func TestConcurrentSubagents(t *testing.T) {
	registry := newTestRegistry(t)

	// Create 5 concurrent subagent runs
	numSubagents := 5
	runIDs := make([]string, numSubagents)

	for i := 0; i < numSubagents; i++ {
		runID := fmt.Sprintf("run-concurrent-%d", i)
		runIDs[i] = runID

		record := &SubagentRunRecord{
			RunID:               runID,
			ChildSessionKey:     fmt.Sprintf("agent:main:subagent:concurrent-%d", i),
			RequesterSessionKey: "agent:main:main",
			RequesterDisplayKey: "Main Agent",
			Task:                fmt.Sprintf("Task #%d", i),
			Cleanup:             "delete",
			Label:               fmt.Sprintf("Concurrent Test %d", i),
			CreatedAt:           time.Now(),
		}

		if err := registry.Register(record); err != nil {
			t.Fatalf("Failed to register run %s: %v", runID, err)
		}
	}

	// Simulate all starting
	for i := 0; i < numSubagents; i++ {
		sessionKey := fmt.Sprintf("agent:main:subagent:concurrent-%d", i)
		registry.OnLifecycleEvent(createStartEvent(sessionKey, runIDs[i]))
	}

	// Verify all are running
	for i := 0; i < numSubagents; i++ {
		record, found := registry.Get(runIDs[i])
		if !found {
			t.Errorf("Run %s not found", runIDs[i])
			continue
		}

		if record.StartedAt == nil {
			t.Errorf("Run %s has no start time", runIDs[i])
		}
	}

	// Simulate all completing
	for i := 0; i < numSubagents; i++ {
		sessionKey := fmt.Sprintf("agent:main:subagent:concurrent-%d", i)
		registry.OnLifecycleEvent(createEndEvent(sessionKey, runIDs[i]))
	}

	// Verify all completed successfully
	for i := 0; i < numSubagents; i++ {
		record, found := registry.Get(runIDs[i])
		if !found {
			t.Errorf("Run %s not found after completion", runIDs[i])
			continue
		}

		if record.EndedAt == nil {
			t.Errorf("Run %s has no end time", runIDs[i])
		}

		if record.Outcome == nil || record.Outcome.Status != "ok" {
			t.Errorf("Run %s did not complete successfully", runIDs[i])
		}
	}

	t.Logf("✅ Concurrent subagents test passed - %d subagents ran successfully", numSubagents)
}

// TestRegistryPersistence tests that registry data persists and can be read back
func TestRegistryPersistence(t *testing.T) {
	registry := newTestRegistry(t)

	record := &SubagentRunRecord{
		RunID:               "run-persist-test",
		ChildSessionKey:     "agent:main:subagent:persist-abc",
		RequesterSessionKey: "agent:main:main",
		RequesterDisplayKey: "Main Agent",
		Task:                "Persistence test",
		Cleanup:             "keep",
		Label:               "Persist Test",
		CreatedAt:           time.Now(),
	}

	if err := registry.Register(record); err != nil {
		t.Fatalf("Failed to register run: %v", err)
	}

	// Verify record can be read back
	retrieved, found := registry.Get("run-persist-test")
	if !found {
		t.Fatal("Run record not found after write")
	}

	if retrieved.Task != "Persistence test" {
		t.Errorf("Task mismatch: got '%s', want 'Persistence test'", retrieved.Task)
	}

	if retrieved.Label != "Persist Test" {
		t.Errorf("Label mismatch: got '%s', want 'Persist Test'", retrieved.Label)
	}

	if retrieved.Cleanup != "keep" {
		t.Errorf("Cleanup mode mismatch: got '%s', want 'keep'", retrieved.Cleanup)
	}

	t.Log("✅ Registry persistence test passed - data can be written and read back")
}

// TestRunLifecycleTracking tests that run lifecycle is properly tracked
func TestRunLifecycleTracking(t *testing.T) {
	registry := newTestRegistry(t)

	record := &SubagentRunRecord{
		RunID:               "run-lifecycle-test",
		ChildSessionKey:     "agent:main:subagent:lifecycle-abc",
		RequesterSessionKey: "agent:main:main",
		RequesterDisplayKey: "Main Agent",
		Task:                "Lifecycle tracking test",
		Cleanup:             "delete",
		Label:               "Lifecycle Test",
		CreatedAt:           time.Now(),
	}

	if err := registry.Register(record); err != nil {
		t.Fatalf("Failed to register run: %v", err)
	}

	// Verify initial state (not started)
	retrieved, found := registry.Get(record.RunID)
	if !found {
		t.Fatal("Run record not found")
	}

	if retrieved.StartedAt != nil {
		t.Error("Run should not have started yet")
	}

	if retrieved.EndedAt != nil {
		t.Error("Run should not have ended yet")
	}

	// Simulate start
	registry.OnLifecycleEvent(createStartEvent(record.ChildSessionKey, record.RunID))

	// Verify started
	retrieved, _ = registry.Get(record.RunID)
	if retrieved.StartedAt == nil {
		t.Error("Run should have started")
	}

	if retrieved.EndedAt != nil {
		t.Error("Run should not have ended yet")
	}

	// Simulate end
	registry.OnLifecycleEvent(createEndEvent(record.ChildSessionKey, record.RunID))

	// Verify ended
	retrieved, _ = registry.Get(record.RunID)
	if retrieved.StartedAt == nil {
		t.Error("Run should have started")
	}

	if retrieved.EndedAt == nil {
		t.Error("Run should have ended")
	}

	if retrieved.Outcome == nil || retrieved.Outcome.Status != "ok" {
		t.Error("Run outcome should be 'ok'")
	}

	t.Log("✅ Lifecycle tracking test passed - run lifecycle properly tracked")
}

// TestCleanupHandling tests that cleanup is properly tracked
func TestCleanupHandling(t *testing.T) {
	registry := newTestRegistry(t)

	record := &SubagentRunRecord{
		RunID:               "run-cleanup-test",
		ChildSessionKey:     "agent:main:subagent:cleanup-abc",
		RequesterSessionKey: "agent:main:main",
		RequesterDisplayKey: "Main Agent",
		Task:                "Cleanup tracking test",
		Cleanup:             "delete",
		Label:               "Cleanup Test",
		CreatedAt:           time.Now(),
		CleanupHandled:      false,
	}

	if err := registry.Register(record); err != nil {
		t.Fatalf("Failed to register run: %v", err)
	}

	// Simulate completion
	registry.OnLifecycleEvent(createStartEvent(record.ChildSessionKey, record.RunID))
	registry.OnLifecycleEvent(createEndEvent(record.ChildSessionKey, record.RunID))

	// Mark cleanup as handled
	if err := registry.MarkCleanupHandled(record.RunID); err != nil {
		t.Fatalf("Failed to mark cleanup handled: %v", err)
	}

	// Verify cleanup was marked
	retrieved, found := registry.Get(record.RunID)
	if !found {
		t.Fatal("Run record not found")
	}

	if !retrieved.CleanupHandled {
		t.Error("Cleanup should be marked as handled")
	}

	if retrieved.CleanupCompletedAt == nil {
		t.Error("CleanupCompletedAt should be set")
	}

	t.Log("✅ Cleanup handling test passed - cleanup correctly tracked")
}

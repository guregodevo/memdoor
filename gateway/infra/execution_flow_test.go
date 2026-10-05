package infra

import (
	"encoding/json"
	"testing"
	"time"
)

func TestExecutionFlowTracker(t *testing.T) {
	tracker := NewExecutionFlowTracker(false)

	runID := "test-run-123"
	sessionID := "test-session-456"

	// Start tracking
	tracker.Start(runID, sessionID)

	// Record some tool executions
	tracker.RecordToolStart(runID, "Read", 1, `{"file_path": "/test/file.txt"}`)
	time.Sleep(10 * time.Millisecond)
	tracker.RecordToolComplete(runID, "Read", 1, "File contents here", "")

	tracker.RecordToolStart(runID, "Grep", 2, `{"pattern": "test", "path": "/test"}`)
	time.Sleep(15 * time.Millisecond)
	tracker.RecordToolComplete(runID, "Grep", 2, "Found 3 matches", "")

	tracker.RecordToolStart(runID, "Edit", 3, `{"file_path": "/test/file.txt"}`)
	time.Sleep(20 * time.Millisecond)
	tracker.RecordToolComplete(runID, "Edit", 3, "File edited", "")

	// Complete the run
	tracker.Complete(runID)

	// Get the flow
	flow, err := tracker.GetFlow(runID)
	if err != nil {
		t.Fatalf("Failed to get flow: %v", err)
	}

	// Verify flow
	if flow.RunID != runID {
		t.Errorf("Expected runID %s, got %s", runID, flow.RunID)
	}

	if flow.SessionID != sessionID {
		t.Errorf("Expected sessionID %s, got %s", sessionID, flow.SessionID)
	}

	if len(flow.Nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(flow.Nodes))
	}

	// Verify first node
	if flow.Nodes[0].ToolName != "Read" {
		t.Errorf("Expected first tool 'Read', got '%s'", flow.Nodes[0].ToolName)
	}

	if flow.Nodes[0].Duration <= 0 {
		t.Errorf("Expected positive duration, got %d", flow.Nodes[0].Duration)
	}

	if !flow.Nodes[0].Success {
		t.Error("Expected first tool to succeed")
	}

	// Verify input parsing
	if flow.Nodes[0].Input["file_path"] != "/test/file.txt" {
		t.Errorf("Expected file_path '/test/file.txt', got %v", flow.Nodes[0].Input["file_path"])
	}

	t.Logf("Flow tracked successfully with %d nodes", len(flow.Nodes))
}

func TestExecutionFlowMermaid(t *testing.T) {
	tracker := NewExecutionFlowTracker(false)

	runID := "test-run-mermaid"
	sessionID := "test-session"

	tracker.Start(runID, sessionID)

	// Record tool executions
	tracker.RecordToolStart(runID, "Read", 1, `{"file_path": "/test.txt"}`)
	time.Sleep(5 * time.Millisecond)
	tracker.RecordToolComplete(runID, "Read", 1, "Content", "")

	tracker.RecordToolStart(runID, "Write", 2, `{"file_path": "/output.txt"}`)
	time.Sleep(10 * time.Millisecond)
	tracker.RecordToolComplete(runID, "Write", 2, "", "permission denied")

	tracker.Complete(runID)

	flow, err := tracker.GetFlow(runID)
	if err != nil {
		t.Fatalf("Failed to get flow: %v", err)
	}

	// Generate Mermaid
	mermaid := flow.ToMermaidFlowchart()

	// Verify Mermaid contains expected elements
	if mermaid == "" {
		t.Error("Mermaid output is empty")
	}

	if !contains(mermaid, "flowchart TB") {
		t.Error("Mermaid should contain flowchart header")
	}

	if !contains(mermaid, "Read") {
		t.Error("Mermaid should contain Read tool")
	}

	if !contains(mermaid, "Write") {
		t.Error("Mermaid should contain Write tool")
	}

	if !contains(mermaid, "❌") {
		t.Error("Mermaid should mark failed tool with ❌")
	}

	t.Logf("Mermaid diagram:\n%s", mermaid)
}

func TestExecutionFlowJSON(t *testing.T) {
	tracker := NewExecutionFlowTracker(false)

	runID := "test-run-json"
	sessionID := "test-session"

	tracker.Start(runID, sessionID)

	tracker.RecordToolStart(runID, "Bash", 1, `{"command": "ls -la"}`)
	time.Sleep(5 * time.Millisecond)
	tracker.RecordToolComplete(runID, "Bash", 1, "file1\nfile2\nfile3", "")

	tracker.Complete(runID)

	flow, err := tracker.GetFlow(runID)
	if err != nil {
		t.Fatalf("Failed to get flow: %v", err)
	}

	// Generate JSON
	jsonData, err := flow.ToJSON()
	if err != nil {
		t.Fatalf("Failed to generate JSON: %v", err)
	}

	// Verify JSON is valid
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Verify structure
	if parsed["run_id"] != runID {
		t.Errorf("Expected run_id %s, got %v", runID, parsed["run_id"])
	}

	nodes, ok := parsed["nodes"].([]interface{})
	if !ok || len(nodes) != 1 {
		t.Errorf("Expected 1 node in JSON, got %v", nodes)
	}

	t.Logf("JSON output:\n%s", string(jsonData))
}

func TestGetLatestFlow(t *testing.T) {
	tracker := NewExecutionFlowTracker(false)

	// Create multiple flows
	tracker.Start("run-1", "session-1")
	tracker.RecordToolStart("run-1", "Read", 1, "{}")
	tracker.RecordToolComplete("run-1", "Read", 1, "data", "")
	tracker.Complete("run-1")

	time.Sleep(10 * time.Millisecond) // Ensure different timestamps

	tracker.Start("run-2", "session-2")
	tracker.RecordToolStart("run-2", "Write", 1, "{}")
	tracker.RecordToolComplete("run-2", "Write", 1, "ok", "")
	tracker.Complete("run-2")

	// Get latest should return run-2
	latest, err := tracker.GetLatestFlow()
	if err != nil {
		t.Fatalf("Failed to get latest flow: %v", err)
	}

	if latest.RunID != "run-2" {
		t.Errorf("Expected latest run to be 'run-2', got '%s'", latest.RunID)
	}

	if len(latest.Nodes) != 1 || latest.Nodes[0].ToolName != "Write" {
		t.Error("Latest flow should contain Write tool")
	}

	t.Logf("Latest flow: %s with %d nodes", latest.RunID, len(latest.Nodes))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

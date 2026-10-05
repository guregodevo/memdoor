package subagents

import (
	"strings"
	"testing"
	"time"
)

// TestBuildAnnouncementMessage_Success tests announcement for successful completion
func TestBuildAnnouncementMessage_Success(t *testing.T) {
	params := AnnounceParams{
		RunID:               "run-123",
		Label:               "analyze logs",
		Task:                "Analyze the error logs from yesterday",
		RequesterSessionKey: "agent:main:main",
		ChildSessionKey:     "agent:main:subagent:abc123",
		FinalOutput:         "Found 3 critical errors related to database timeouts",
		Stats: &SubagentStats{
			RuntimeMs:     5432,
			TokensUsed:    1250,
			EstimatedCost: "$0.05",
		},
		Outcome: &SubagentOutcome{
			Status: "ok",
		},
	}

	message := BuildAnnouncementMessage(params)

	// Verify message structure
	if !strings.Contains(message, `"analyze logs"`) {
		t.Error("Message should contain label in quotes")
	}

	if !strings.Contains(message, "successfully") {
		t.Error("Message should indicate success")
	}

	if !strings.Contains(message, "Analyze the error logs from yesterday") {
		t.Error("Message should contain original task")
	}

	if !strings.Contains(message, "Found 3 critical errors") {
		t.Error("Message should contain final output")
	}

	if !strings.Contains(message, "runtime 5.4s") {
		t.Error("Message should contain runtime stats")
	}

	if !strings.Contains(message, "tokens 1250") {
		t.Error("Message should contain token count")
	}

	if !strings.Contains(message, "est $0.05") {
		t.Error("Message should contain estimated cost")
	}

	if !strings.Contains(message, "sessionKey agent:main:subagent:abc123") {
		t.Error("Message should contain session key")
	}

	if !strings.Contains(message, "NOT the whole task") {
		t.Error("Message should tell the driver this is one step, not the whole task")
	}

	if !strings.Contains(message, "report the final result") {
		t.Error("Message should tell the driver to report only when every step is done")
	}
}

// TestBuildAnnouncementMessage_Error tests announcement for failed completion
func TestBuildAnnouncementMessage_Error(t *testing.T) {
	params := AnnounceParams{
		RunID:               "run-456",
		Label:               "fetch data",
		Task:                "Fetch user data from API",
		RequesterSessionKey: "agent:main:main",
		ChildSessionKey:     "agent:main:subagent:def456",
		FinalOutput:         "", // No output on error
		Stats: &SubagentStats{
			RuntimeMs: 1200,
		},
		Outcome: &SubagentOutcome{
			Status: "error",
			Error:  "API returned 404: endpoint not found",
		},
	}

	message := BuildAnnouncementMessage(params)

	// Verify error handling
	if !strings.Contains(message, "with an error") {
		t.Error("Message should indicate error")
	}

	if !strings.Contains(message, "Error:") {
		t.Error("Message should have Error section")
	}

	if !strings.Contains(message, "API returned 404") {
		t.Error("Message should contain error details")
	}

	if !strings.Contains(message, "runtime 1.2s") {
		t.Error("Message should contain runtime even on error")
	}
}

// TestBuildAnnouncementMessage_NoLabel tests announcement without label
func TestBuildAnnouncementMessage_NoLabel(t *testing.T) {
	params := AnnounceParams{
		RunID:               "run-789",
		Label:               "", // No label
		Task:                "Some task",
		RequesterSessionKey: "agent:main:main",
		ChildSessionKey:     "agent:main:subagent:ghi789",
		FinalOutput:         "Task completed",
		Outcome: &SubagentOutcome{
			Status: "ok",
		},
	}

	message := BuildAnnouncementMessage(params)

	// Without label, should use generic phrasing
	if !strings.Contains(message, "A subagent task just completed") {
		t.Error("Message should have generic intro without label")
	}

	if strings.Contains(message, `""`) {
		t.Error("Message should not contain empty quotes")
	}
}

// TestBuildAnnouncementMessage_MinimalStats tests with minimal stats
func TestBuildAnnouncementMessage_MinimalStats(t *testing.T) {
	params := AnnounceParams{
		RunID:               "run-min",
		Label:               "quick task",
		Task:                "Quick task",
		RequesterSessionKey: "agent:main:main",
		ChildSessionKey:     "agent:main:subagent:min123",
		FinalOutput:         "Done",
		Stats: &SubagentStats{
			RuntimeMs: 500, // Only runtime, no tokens or cost
		},
		Outcome: &SubagentOutcome{
			Status: "ok",
		},
	}

	message := BuildAnnouncementMessage(params)

	// Should still have stats line with session key
	if !strings.Contains(message, "Stats:") {
		t.Error("Message should have Stats section")
	}

	if !strings.Contains(message, "runtime 0.5s") {
		t.Error("Message should show runtime")
	}

	if !strings.Contains(message, "sessionKey") {
		t.Error("Message should always include session key in stats")
	}
}

// TestCalculateStats tests statistics calculation from run record
func TestCalculateStats(t *testing.T) {
	startTime := time.Now().Add(-10 * time.Second)
	endTime := time.Now()

	record := &SubagentRunRecord{
		RunID:           "run-stats",
		ChildSessionKey: "agent:main:subagent:stats123",
		StartedAt:       &startTime,
		EndedAt:         &endTime,
	}

	stats := CalculateStats(record)

	if stats == nil {
		t.Fatal("Expected stats, got nil")
	}

	// Runtime should be approximately 10 seconds (10000ms)
	// Allow some variance due to execution time
	if stats.RuntimeMs < 9900 || stats.RuntimeMs > 10100 {
		t.Errorf("Expected runtime ~10000ms, got %d", stats.RuntimeMs)
	}
}

// TestCalculateStats_NilRecord tests nil handling
func TestCalculateStats_NilRecord(t *testing.T) {
	stats := CalculateStats(nil)

	if stats != nil {
		t.Error("Expected nil for nil record")
	}
}

// TestCalculateStats_NoTimes tests record without timestamps
func TestCalculateStats_NoTimes(t *testing.T) {
	record := &SubagentRunRecord{
		RunID:           "run-notimes",
		ChildSessionKey: "agent:main:subagent:notimes123",
		StartedAt:       nil,
		EndedAt:         nil,
	}

	stats := CalculateStats(record)

	if stats == nil {
		t.Fatal("Expected stats struct, got nil")
	}

	if stats.RuntimeMs != 0 {
		t.Errorf("Expected 0ms runtime for missing times, got %d", stats.RuntimeMs)
	}
}

// TestShouldAnnounce tests announce condition logic
func TestShouldAnnounce(t *testing.T) {
	endTime := time.Now()

	tests := []struct {
		name     string
		record   *SubagentRunRecord
		expected bool
	}{
		{
			name:     "nil record",
			record:   nil,
			expected: false,
		},
		{
			name: "completed run",
			record: &SubagentRunRecord{
				RunID:           "run-complete",
				ChildSessionKey: "agent:main:subagent:complete123",
				EndedAt:         &endTime,
			},
			expected: true,
		},
		{
			name: "incomplete run",
			record: &SubagentRunRecord{
				RunID:           "run-incomplete",
				ChildSessionKey: "agent:main:subagent:incomplete123",
				EndedAt:         nil, // Not ended yet
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldAnnounce(tt.record)
			if result != tt.expected {
				t.Errorf("ShouldAnnounce() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestAnnouncementMessage_Structure tests overall message structure
func TestAnnouncementMessage_Structure(t *testing.T) {
	params := AnnounceParams{
		RunID:               "run-struct",
		Label:               "test task",
		Task:                "Test task description",
		RequesterSessionKey: "agent:main:main",
		ChildSessionKey:     "agent:main:subagent:struct123",
		FinalOutput:         "Test output",
		Stats: &SubagentStats{
			RuntimeMs: 3000,
		},
		Outcome: &SubagentOutcome{
			Status: "ok",
		},
	}

	message := BuildAnnouncementMessage(params)

	// Message should have clear sections
	sections := []string{
		"A subagent task",    // Introduction
		"Task:",              // Task description
		"Findings:",          // Output
		"Stats:",             // Statistics
		"NOT the whole task", // Driver directive: keep driving, don't stop
	}

	for _, section := range sections {
		if !strings.Contains(message, section) {
			t.Errorf("Message missing expected section: %q", section)
		}
	}

	// Should not be empty
	if len(message) == 0 {
		t.Error("Message should not be empty")
	}

	// Should have multiple lines
	lines := strings.Split(message, "\n")
	if len(lines) < 5 {
		t.Errorf("Message should have at least 5 lines, got %d", len(lines))
	}
}

// The closing directive follows the result check's verdict; unjudged keeps
// the driver's "keep going" directive.
func TestAnnouncementDirectiveFollowsVerdict(t *testing.T) {
	base := AnnounceParams{Task: "Create acc4 and run go test.", FinalOutput: "ok acc4/hello"}
	cases := map[string]struct{ want, not string }{
		"":                {"ONE step done", "result check"},
		VerdictDone:       {"do not dispatch more work nobody asked for", "ONE step done"},
		VerdictUnfinished: {"NOT done", "ONE step done"},
	}
	for v, c := range cases {
		p := base
		p.Verdict = v
		msg := BuildAnnouncementMessage(p)
		if !strings.Contains(msg, c.want) || strings.Contains(msg, c.not) {
			t.Errorf("verdict %q: %q", v, msg[len(msg)-200:])
		}
	}
}

package tools

import (
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/subagents"
	"memdoor/pkg/repository/sqlite"

	_ "memdoor/pkg/sqlitedriver"
)

// Mock implementations for testing

type mockQueueEnqueuer struct {
	enqueuedJobs []string
	workdirs     []string        // the requester directory each spawn carried
	timeouts     []time.Duration // the limit each spawn carried
}

func (m *mockQueueEnqueuer) EnqueueSubagentJob(runID, sessionKey, agentID, message string, tools []string, systemPrompt string, workdir string, timeout time.Duration) error {
	m.enqueuedJobs = append(m.enqueuedJobs, runID+":"+sessionKey+":"+agentID+":"+message)
	m.workdirs = append(m.workdirs, workdir)
	m.timeouts = append(m.timeouts, timeout)
	return nil
}

type mockSession struct {
	key          string
	systemPrompt string
	metadata     map[string]interface{}
}

func (m *mockSession) GetKey() string {
	return m.key
}

func (m *mockSession) SetSystemPrompt(prompt string) error {
	m.systemPrompt = prompt
	return nil
}

func (m *mockSession) SetMetadata(key string, value interface{}) {
	if m.metadata == nil {
		m.metadata = make(map[string]interface{})
	}
	m.metadata[key] = value
}

func (m *mockSession) GetMetadata() map[string]interface{} {
	if m.metadata == nil {
		return map[string]interface{}{}
	}
	return m.metadata
}

type mockSessionGetter struct {
	sessions map[string]*mockSession
}

func newMockSessionGetter() *mockSessionGetter {
	return &mockSessionGetter{
		sessions: make(map[string]*mockSession),
	}
}

func (m *mockSessionGetter) GetOrCreateSession(sessionKey, kind string) (Session, error) {
	if session, exists := m.sessions[sessionKey]; exists {
		return session, nil
	}
	session := &mockSession{key: sessionKey}
	m.sessions[sessionKey] = session
	return session, nil
}

// Test helper to create a test tool
func newTestSessionsSpawnTool(t *testing.T) (*SessionsSpawnTool, *subagents.SubagentRegistry, *mockQueueEnqueuer, *mockSessionGetter) {
	t.Helper()
	logs.InitGlobalLoggerDefault(false)
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared&_journal_mode=WAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS subagent_runs (
		run_id TEXT PRIMARY KEY,
		child_session_key TEXT NOT NULL,
		requester_session_key TEXT NOT NULL DEFAULT '',
		requester_display_key TEXT NOT NULL DEFAULT '',
		task TEXT NOT NULL DEFAULT '',
		cleanup TEXT NOT NULL DEFAULT '',
		label TEXT NOT NULL DEFAULT '',
		parent_message_id INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		started_at INTEGER,
		ended_at INTEGER,
		outcome_status TEXT NOT NULL DEFAULT '',
		outcome_error TEXT NOT NULL DEFAULT '',
		archive_at_ms INTEGER NOT NULL DEFAULT 0,
		cleanup_completed_at INTEGER,
		cleanup_handled INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		t.Fatal(err)
	}

	repo := sqlite.NewSubagentRunRepository(db)
	registry := subagents.NewSubagentRegistry(repo, false)

	queueEnqueuer := &mockQueueEnqueuer{enqueuedJobs: []string{}}
	sessionGetter := newMockSessionGetter()

	// Resolver returns a non-empty palette so spawns don't fail-fast in tests.
	resolveConfig := func(agentID string) ([]string, string) {
		return []string{"read_file", "bash"}, "test agent prompt"
	}
	tool := NewSessionsSpawnTool(registry, queueEnqueuer, sessionGetter, resolveConfig, false)
	return tool, registry, queueEnqueuer, sessionGetter
}

func TestSessionsSpawnTool_ValidSpawn(t *testing.T) {
	tool, registry, queue, sessions := newTestSessionsSpawnTool(t)

	input := SessionsSpawnInput{
		Task:    "Search documentation for API patterns",
		Label:   "Doc Search",
		AgentID: "main",
		Cleanup: "delete",
	}

	inputJSON, _ := json.Marshal(input)
	result, err := tool.Execute(inputJSON, "agent:main:main", "main", "/workspace", "")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// The result is one line naming the run and its session.
	if !strings.Contains(result, "spawned as main") {
		t.Errorf("result should say what was spawned, got %q", result)
	}
	runID, sessionKey := spawnedRun(t, result)
	if !strings.HasPrefix(sessionKey, "agent:main:subagent:") {
		t.Errorf("sessionKey should have correct format, got: %s", sessionKey)
	}

	// Check registry
	record, exists := registry.Get(runID)
	if !exists {
		t.Error("Run should be registered in registry")
	} else {
		if record.Task != "Search documentation for API patterns" {
			t.Errorf("Expected task to be registered, got: %s", record.Task)
		}
		if record.Label != "Doc Search" {
			t.Errorf("Expected label 'Doc Search', got: %s", record.Label)
		}
		if record.Cleanup != "delete" {
			t.Errorf("Expected cleanup 'delete', got: %s", record.Cleanup)
		}
	}

	// Check queue
	if len(queue.enqueuedJobs) != 1 {
		t.Errorf("Expected 1 enqueued job, got %d", len(queue.enqueuedJobs))
	}

	// Check session created
	if _, exists := sessions.sessions[sessionKey]; !exists {
		t.Error("Session should be created")
	} else {
		session := sessions.sessions[sessionKey]
		if session.systemPrompt == "" {
			t.Error("System prompt should be set")
		}
		if !strings.Contains(session.systemPrompt, "Search documentation for API patterns") {
			t.Error("System prompt should contain the task")
		}
	}
}

func TestSessionsSpawnTool_MinimalParams(t *testing.T) {
	tool, registry, _, _ := newTestSessionsSpawnTool(t)

	input := SessionsSpawnInput{
		Task: "Run tests",
	}

	inputJSON, _ := json.Marshal(input)
	result, err := tool.Execute(inputJSON, "agent:main:main", "main", "/workspace", "")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runID, _ := spawnedRun(t, result)
	record, _ := registry.Get(runID)

	// Defaults should be applied
	if record.Cleanup != "delete" {
		t.Errorf("Expected default cleanup 'delete', got: %s", record.Cleanup)
	}
}

func TestSessionsSpawnTool_CrossAgent(t *testing.T) {
	tool, registry, _, _ := newTestSessionsSpawnTool(t)

	input := SessionsSpawnInput{
		Task:    "Analyze code",
		AgentID: "agent2", // Different from requester
	}

	inputJSON, _ := json.Marshal(input)
	result, err := tool.Execute(inputJSON, "agent:main:main", "main", "/workspace", "")

	if err != nil {
		t.Fatalf("Expected no error for cross-agent spawn, got: %v", err)
	}

	if !strings.Contains(result, "spawned as agent2") {
		t.Errorf("Expected agent2 to be spawned, got: %q", result)
	}
	runID, sessionKey := spawnedRun(t, result)
	if !strings.HasPrefix(sessionKey, "agent:agent2:subagent:") {
		t.Errorf("sessionKey should use target agentId, got: %s", sessionKey)
	}
	record, _ := registry.Get(runID)
	if !strings.Contains(record.ChildSessionKey, "agent2") {
		t.Error("Record should use target agent ID in child session key")
	}
}

func TestSessionsSpawnTool_ValidationErrors(t *testing.T) {
	tool, _, _, _ := newTestSessionsSpawnTool(t)

	tests := []struct {
		name        string
		input       SessionsSpawnInput
		expectedErr string
	}{
		{
			name:        "missing task",
			input:       SessionsSpawnInput{Task: ""},
			expectedErr: "task is required",
		},
		{
			name:        "invalid cleanup",
			input:       SessionsSpawnInput{Task: "Test", Cleanup: "invalid"},
			expectedErr: "cleanup must be 'delete' or 'keep'",
		},
		{
			name:        "negative timeout",
			input:       SessionsSpawnInput{Task: "Test", RunTimeoutSeconds: -1},
			expectedErr: "runTimeoutSeconds must be between 0 and 3600",
		},
		{
			name:        "timeout too large",
			input:       SessionsSpawnInput{Task: "Test", RunTimeoutSeconds: 5000},
			expectedErr: "runTimeoutSeconds must be between 0 and 3600",
		},
		{
			name:        "invalid thinking mode",
			input:       SessionsSpawnInput{Task: "Test", Thinking: "invalid"},
			expectedErr: "thinking must be 'enabled', 'disabled', or 'auto'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, _ := json.Marshal(tt.input)
			_, err := tool.Execute(inputJSON, "agent:main:main", "main", "/workspace", "")

			if err == nil {
				t.Error("Expected error, got nil")
			} else if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error containing %q, got: %v", tt.expectedErr, err)
			}
		})
	}
}

func TestSessionsSpawnTool_TaskTooLong(t *testing.T) {
	tool, _, _, _ := newTestSessionsSpawnTool(t)

	// Create a task that's too long
	longTask := strings.Repeat("a", 10001)
	input := SessionsSpawnInput{Task: longTask}

	inputJSON, _ := json.Marshal(input)
	_, err := tool.Execute(inputJSON, "agent:main:main", "main", "/workspace", "")

	if err == nil {
		t.Error("Expected error for task too long")
	} else if !strings.Contains(err.Error(), "task is too long") {
		t.Errorf("Expected 'task is too long' error, got: %v", err)
	}
}

func TestSessionsSpawnTool_LabelInPrompt(t *testing.T) {
	tool, _, _, sessions := newTestSessionsSpawnTool(t)

	input := SessionsSpawnInput{
		Task:  "Test task",
		Label: "Test Label",
	}

	inputJSON, _ := json.Marshal(input)
	result, _ := tool.Execute(inputJSON, "agent:main:main", "main", "/workspace", "")

	_, sessionKey := spawnedRun(t, result)
	session := sessions.sessions[sessionKey]

	if !strings.Contains(session.systemPrompt, "Test Label") {
		t.Error("System prompt should contain the label")
	}
}

func TestSessionsSpawnTool_WorkspaceDirInPrompt(t *testing.T) {
	tool, _, _, sessions := newTestSessionsSpawnTool(t)

	input := SessionsSpawnInput{Task: "Test task"}

	inputJSON, _ := json.Marshal(input)
	result, _ := tool.Execute(inputJSON, "agent:main:main", "main", "/custom/workspace", "")

	_, sessionKey := spawnedRun(t, result)
	session := sessions.sessions[sessionKey]

	if !strings.Contains(session.systemPrompt, "/custom/workspace") {
		t.Error("System prompt should contain the workspace directory")
	}
}

func TestSessionsSpawnTool_KeepCleanup(t *testing.T) {
	tool, registry, _, _ := newTestSessionsSpawnTool(t)

	input := SessionsSpawnInput{
		Task:    "Test task",
		Cleanup: "keep",
	}

	inputJSON, _ := json.Marshal(input)
	result, _ := tool.Execute(inputJSON, "agent:main:main", "main", "/workspace", "")

	runID, _ := spawnedRun(t, result)
	record, _ := registry.Get(runID)

	if record.Cleanup != "keep" {
		t.Errorf("Expected cleanup 'keep', got: %s", record.Cleanup)
	}
}

func TestSessionsSpawnTool_JSONSchemaGeneration(t *testing.T) {
	// Test that the schema is generated correctly and can be marshaled
	schemaJSON, err := json.MarshalIndent(SessionsSpawnInputSchema, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal schema: %v", err)
	}

	// Schema should have required properties
	schemaStr := string(schemaJSON)
	if len(schemaStr) == 0 {
		t.Error("SessionsSpawnInputSchema should not be empty")
	}

	if !strings.Contains(schemaStr, "task") {
		t.Error("Schema should contain 'task' field")
	}
}

func TestSessionsSpawnTool_BindToContext(t *testing.T) {
	tool, _, _, _ := newTestSessionsSpawnTool(t)

	// Bind to context
	boundFunc := tool.BindToContext("agent:main:main", "main", "/workspace")

	input := SessionsSpawnInput{Task: "Test task"}
	inputJSON, _ := json.Marshal(input)

	result, err := boundFunc(inputJSON)
	if err != nil {
		t.Fatalf("Bound function should work, got error: %v", err)
	}

	if _, session := spawnedRun(t, result); session == "" {
		t.Errorf("the bound function should spawn a run, got: %q", result)
	}
}

func TestGenerateRunID(t *testing.T) {
	runID1 := generateRunID()
	runID2 := generateRunID()

	// Should start with "run-"
	if !strings.HasPrefix(runID1, "run-") {
		t.Errorf("runID should start with 'run-', got: %s", runID1)
	}

	// Should be unique
	if runID1 == runID2 {
		t.Error("Generated run IDs should be unique")
	}

	// Should have reasonable length (run- + 8 chars = 12)
	if len(runID1) != 12 {
		t.Errorf("Expected runID length 12, got: %d", len(runID1))
	}
}

// The requester's working directory rides on the spawned job. Before this, a
// coder spawned by another agent ran in the coder's scratch dir and its files
// never landed beside the requester's work (2026-09-02).
func TestSpawnCarriesRequesterWorkdir(t *testing.T) {
	tool, _, queueEnqueuer, _ := newTestSessionsSpawnTool(t)
	inputJSON := json.RawMessage(`{"task":"write dur.go","agentId":"main"}`)
	if _, err := tool.Execute(inputJSON, "agent:main:main", "main", "/workspace", "/Users/g/memdoor-clip"); err != nil {
		t.Fatal(err)
	}
	if len(queueEnqueuer.workdirs) != 1 || queueEnqueuer.workdirs[0] != "/Users/g/memdoor-clip" {
		t.Errorf("spawned job carried workdirs %v, want the requester's", queueEnqueuer.workdirs)
	}
}

// The requester's runTimeoutSeconds rides on the spawned job, where the
// executor enforces it. Before this it was echoed back and forgotten: a
// sub-session ran eighteen minutes past its 900 s (2026-09-14).
func TestSpawnCarriesRunTimeout(t *testing.T) {
	tool, _, queueEnqueuer, _ := newTestSessionsSpawnTool(t)
	for _, c := range []struct {
		input string
		want  time.Duration
	}{
		{`{"task":"cut the film","agentId":"main","runTimeoutSeconds":900}`, 15 * time.Minute},
		{`{"task":"cut the film","agentId":"main"}`, 5 * time.Minute},
	} {
		if _, err := tool.Execute(json.RawMessage(c.input), "agent:main:main", "main", "/workspace", ""); err != nil {
			t.Fatal(err)
		}
	}
	if len(queueEnqueuer.timeouts) != 2 || queueEnqueuer.timeouts[0] != 15*time.Minute || queueEnqueuer.timeouts[1] != 5*time.Minute {
		t.Errorf("spawned jobs carried timeouts %v, want 15m then the 5m default", queueEnqueuer.timeouts)
	}
}

// The child session remembers where its requester works, so the run's
// announcement (a turn on the requester) runs there and not in the coder's
// scratch directory (2026-09-15).
func TestSpawnRecordsTheRequesterWorkdirOnTheChild(t *testing.T) {
	tool, _, _, sessions := newTestSessionsSpawnTool(t)
	if _, err := tool.Execute(json.RawMessage(`{"task":"cut the film","agentId":"main"}`), "agent:main:main", "main", "/workspace", "/Users/g/zapping/hn"); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, sess := range sessions.sessions {
		if strings.Contains(sess.key, ":subagent:") {
			found = true
			if got := sess.metadata[RequesterWorkdirKey]; got != "/Users/g/zapping/hn" {
				t.Fatalf("child session metadata %s = %v, want the requester's workdir", RequesterWorkdirKey, got)
			}
		}
	}
	if !found {
		t.Fatal("no child session was created")
	}
}

// spawnedRun reads the run id and the child session out of the tool's line.
func spawnedRun(t *testing.T, result string) (runID, session string) {
	t.Helper()
	m := regexp.MustCompile(`▶ (run-[0-9a-f]+)\b.*\(session (\S+),`).FindStringSubmatch(result)
	if m == nil {
		t.Fatalf("no run in %q", result)
	}
	return m[1], m[2]
}

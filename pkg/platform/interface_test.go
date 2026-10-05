package platform

import (
	"context"
	"testing"
	"time"

	"memdoor/pkg/shared"
)

// TestPublishedInterface_Contracts validates interface design and contracts
// These tests verify that the Published Interface can be implemented and used correctly

// Mock implementations for testing interface contracts

type mockAgentExecutor struct {
	processMessageCalled bool
	eventStream          *mockEventStream
}

func (m *mockAgentExecutor) ProcessMessage(
	ctx context.Context,
	userMessage string,
	session SessionContext,
	runID string,
	extraSystemPrompt string,
) (*ExecutionResult, error) {
	m.processMessageCalled = true
	return &ExecutionResult{
		Text: "Mock response",
		ToolsExecuted: []ToolExecution{
			{
				Name:       "mock_tool",
				Input:      `{"query": "test"}`,
				Output:     "Mock output",
				Error:      "",
				DurationMs: 100,
			},
		},
		Error: "",
		Metadata: map[string]interface{}{
			"model": "claude-sonnet-4-5",
		},
	}, nil
}

func (m *mockAgentExecutor) EventStream() EventStream {
	return m.eventStream
}

type mockEventStream struct {
	subscribers map[string][]func(event shared.Event)
	emitCalled  bool
}

func newMockEventStream() *mockEventStream {
	return &mockEventStream{
		subscribers: make(map[string][]func(event shared.Event)),
	}
}

func (m *mockEventStream) Subscribe(runID string, listener func(event shared.Event)) func() {
	m.subscribers[runID] = append(m.subscribers[runID], listener)

	// For testing purposes, return an unsubscribe function that clears the subscriber list
	// In a real implementation, this would remove only the specific listener
	unsubscribed := false
	return func() {
		if !unsubscribed {
			// Clear all listeners for this runID (simplified for testing)
			delete(m.subscribers, runID)
			unsubscribed = true
		}
	}
}

func (m *mockEventStream) Emit(runID string, event shared.Event) {
	m.emitCalled = true
	for _, listener := range m.subscribers[runID] {
		listener(event)
	}
}

type mockSessionContext struct {
	id       string
	typ      string
	messages []SessionMessage
	metadata map[string]interface{}
}

func (m *mockSessionContext) GetID() string {
	return m.id
}

func (m *mockSessionContext) GetType() string {
	return m.typ
}

func (m *mockSessionContext) GetMessages() []SessionMessage {
	return m.messages
}

func (m *mockSessionContext) GetMetadata() map[string]interface{} {
	return m.metadata
}

type mockRuntimeFactory struct {
	createExecutorCalled bool
}

func (m *mockRuntimeFactory) CreateExecutor(apiKey string, config *AgentConfiguration) (AgentExecutor, error) {
	m.createExecutorCalled = true
	return &mockAgentExecutor{
		eventStream: newMockEventStream(),
	}, nil
}

// Tests

func TestAgentExecutor_ProcessMessage(t *testing.T) {
	t.Run("interface contract can be implemented", func(t *testing.T) {
		executor := &mockAgentExecutor{
			eventStream: newMockEventStream(),
		}

		session := &mockSessionContext{
			id:  "session-123",
			typ: "channel",
			messages: []SessionMessage{
				{Role: "user", Content: "Hello", Timestamp: time.Now().UnixMilli()},
			},
			metadata: map[string]interface{}{
				"channel_id": "channel-123",
			},
		}

		result, err := executor.ProcessMessage(
			context.Background(),
			"Test message",
			session,
			"run-123",
			"",
		)

		if err != nil {
			t.Fatalf("ProcessMessage() error: %v", err)
		}

		if result.Text == "" {
			t.Errorf("ExecutionResult.Text is empty")
		}

		if !executor.processMessageCalled {
			t.Errorf("ProcessMessage was not called")
		}
	})
}

func TestSessionContext_Interface(t *testing.T) {
	t.Run("interface contract provides session information", func(t *testing.T) {
		session := &mockSessionContext{
			id:  "session-456",
			typ: "channel",
			messages: []SessionMessage{
				{Role: "user", Content: "Message 1", Timestamp: time.Now().UnixMilli()},
				{Role: "assistant", Content: "Response 1", Timestamp: time.Now().UnixMilli()},
			},
			metadata: map[string]interface{}{
				"channel_id": "channel-456",
			},
		}

		if session.GetID() != "session-456" {
			t.Errorf("GetID() = %v, want 'session-456'", session.GetID())
		}

		if session.GetType() != "channel" {
			t.Errorf("GetType() = %v, want 'channel'", session.GetType())
		}

		if len(session.GetMessages()) != 2 {
			t.Errorf("GetMessages() count = %v, want 2", len(session.GetMessages()))
		}

		if session.GetMetadata()["channel_id"] != "channel-456" {
			t.Errorf("GetMetadata()[channel_id] = %v, want 'channel-456'", session.GetMetadata()["channel_id"])
		}
	})
}

func TestExecutionResult_Structure(t *testing.T) {
	t.Run("execution result contains expected fields", func(t *testing.T) {
		result := &ExecutionResult{
			Text: "Test response",
			ToolsExecuted: []ToolExecution{
				{
					Name:       "web_search",
					Input:      `{"query": "test"}`,
					Output:     "Search results",
					Error:      "",
					DurationMs: 500,
				},
			},
			Error: "",
			Metadata: map[string]interface{}{
				"tokens_used": 1500,
				"model":       "claude-sonnet-4-5",
			},
		}

		if result.Text != "Test response" {
			t.Errorf("Text = %v, want 'Test response'", result.Text)
		}

		if len(result.ToolsExecuted) != 1 {
			t.Errorf("ToolsExecuted count = %v, want 1", len(result.ToolsExecuted))
		}

		if result.ToolsExecuted[0].Name != "web_search" {
			t.Errorf("ToolsExecuted[0].Name = %v, want 'web_search'", result.ToolsExecuted[0].Name)
		}

		if result.Metadata["model"] != "claude-sonnet-4-5" {
			t.Errorf("Metadata[model] = %v, want 'claude-sonnet-4-5'", result.Metadata["model"])
		}
	})
}

func TestAgentConfiguration_Structure(t *testing.T) {
	t.Run("agent configuration contains expected fields", func(t *testing.T) {
		config := &AgentConfiguration{
			ID:           "agent-123",
			Name:         "Test Agent",
			Profile:      "coding",
			Model:        "claude-sonnet-4-5",
			SystemPrompt: "You are a helpful assistant",
			AllowedTools: []string{"web_search", "read_file"},
			DeniedTools:  []string{"delete_file"},
			Metadata: map[string]interface{}{
				"temperature": 0.7,
				"max_tokens":  4000,
			},
		}

		if config.ID != "agent-123" {
			t.Errorf("ID = %v, want 'agent-123'", config.ID)
		}

		if config.Profile != "coding" {
			t.Errorf("Profile = %v, want 'coding'", config.Profile)
		}

		if len(config.AllowedTools) != 2 {
			t.Errorf("AllowedTools count = %v, want 2", len(config.AllowedTools))
		}

		if len(config.DeniedTools) != 1 {
			t.Errorf("DeniedTools count = %v, want 1", len(config.DeniedTools))
		}
	})
}

func TestRuntimeFactory_CreateExecutor(t *testing.T) {
	t.Run("factory creates executor instances", func(t *testing.T) {
		factory := &mockRuntimeFactory{}

		config := &AgentConfiguration{
			ID:      "agent-123",
			Name:    "Test Agent",
			Profile: "full",
			Model:   "claude-sonnet-4-5",
		}

		executor, err := factory.CreateExecutor("test-api-key", config)
		if err != nil {
			t.Fatalf("CreateExecutor() error: %v", err)
		}

		if executor == nil {
			t.Errorf("CreateExecutor() returned nil executor")
		}

		if !factory.createExecutorCalled {
			t.Errorf("CreateExecutor was not called")
		}
	})
}

func TestSessionMessage_Structure(t *testing.T) {
	t.Run("session message contains expected fields", func(t *testing.T) {
		timestamp := time.Now().UnixMilli()
		msg := SessionMessage{
			Role:      "user",
			Content:   "Test message",
			Timestamp: timestamp,
		}

		if msg.Role != "user" {
			t.Errorf("Role = %v, want 'user'", msg.Role)
		}

		if msg.Content != "Test message" {
			t.Errorf("Content = %v, want 'Test message'", msg.Content)
		}

		if msg.Timestamp != timestamp {
			t.Errorf("Timestamp = %v, want %v", msg.Timestamp, timestamp)
		}
	})
}

func TestToolExecution_Structure(t *testing.T) {
	t.Run("tool execution tracks invocation details", func(t *testing.T) {
		toolExec := ToolExecution{
			Name:       "web_search",
			Input:      `{"query": "golang testing"}`,
			Output:     "Search completed",
			Error:      "",
			DurationMs: 350,
		}

		if toolExec.Name != "web_search" {
			t.Errorf("Name = %v, want 'web_search'", toolExec.Name)
		}

		if toolExec.DurationMs != 350 {
			t.Errorf("DurationMs = %v, want 350", toolExec.DurationMs)
		}

		if toolExec.Error != "" {
			t.Errorf("Error should be empty for successful execution")
		}
	})

	t.Run("tool execution captures errors", func(t *testing.T) {
		toolExec := ToolExecution{
			Name:       "read_file",
			Input:      `{"path": "/nonexistent"}`,
			Output:     "",
			Error:      "file not found",
			DurationMs: 10,
		}

		if toolExec.Error != "file not found" {
			t.Errorf("Error = %v, want 'file not found'", toolExec.Error)
		}

		if toolExec.Output != "" {
			t.Errorf("Output should be empty for failed execution")
		}
	})
}

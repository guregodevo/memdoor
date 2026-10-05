package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"memdoor/pkg/platform"
)

func TestRemoteExecutor_Execute_Success(t *testing.T) {
	// Create mock OpenAI-compatible server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json")
		}

		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Expected Authorization header")
		}

		// Parse request
		var req ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		// Verify request contents
		if req.Model != "gpt-4" {
			t.Errorf("Expected model gpt-4, got %s", req.Model)
		}

		if len(req.Messages) != 1 {
			t.Errorf("Expected 1 message, got %d", len(req.Messages))
		}

		// Send mock response
		resp := ChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4",
			Choices: []ChatCompletionChoice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: ChatCompletionUsage{
				PromptTokens:     10,
				CompletionTokens: 8,
				TotalTokens:      18,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create executor
	executor := NewRemoteExecutor()

	// Create config
	apiKey := "test-api-key"
	config := &platform.RemoteAgentConfig{
		Endpoint:   server.URL,
		APIKey:     &apiKey,
		Model:      "gpt-4",
		Timeout:    30,
		MaxRetries: 3,
	}

	// Create messages
	messages := []platform.SessionMessage{
		{
			Role:      "user",
			Content:   "Hello",
			Timestamp: time.Now().UnixMilli(),
		},
	}

	// Execute
	ctx := context.Background()
	result, err := executor.Execute(ctx, config, messages, nil)

	// Verify results
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Text != "Hello! How can I help you?" {
		t.Errorf("Expected response text, got %s", result.Text)
	}

	if result.Usage.TotalTokens != 18 {
		t.Errorf("Expected 18 total tokens, got %d", result.Usage.TotalTokens)
	}
}

func TestRemoteExecutor_Execute_WithToolCalls(t *testing.T) {
	// Create mock server that returns tool calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatCompletionResponse{
			ID:      "chatcmpl-456",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4",
			Choices: []ChatCompletionChoice{
				{
					Index: 0,
					Message: ChatMessage{
						Role: "assistant",
						ToolCalls: []ToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: FunctionCall{
									Name:      "get_weather",
									Arguments: `{"location":"San Francisco"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: ChatCompletionUsage{
				PromptTokens:     15,
				CompletionTokens: 20,
				TotalTokens:      35,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	executor := NewRemoteExecutor()
	apiKey := "test-key"
	config := &platform.RemoteAgentConfig{
		Endpoint:   server.URL,
		APIKey:     &apiKey,
		Model:      "gpt-4",
		Timeout:    30,
		MaxRetries: 1,
	}

	messages := []platform.SessionMessage{
		{Role: "user", Content: "What's the weather?", Timestamp: time.Now().UnixMilli()},
	}

	tools := []platform.ToolDefinition{
		{
			Name:        "get_weather",
			Description: "Get the current weather",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "The city name",
					},
				},
			},
		},
	}

	result, err := executor.Execute(context.Background(), config, messages, tools)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(result.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(result.ToolCalls))
	}

	if result.ToolCalls[0].Name != "get_weather" {
		t.Errorf("Expected tool call name get_weather, got %s", result.ToolCalls[0].Name)
	}
}

func TestRemoteExecutor_Execute_ErrorResponse(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		errResp := ErrorResponse{
			Error: ErrorDetail{
				Message: "Invalid API key",
				Type:    "invalid_request_error",
			},
		}
		json.NewEncoder(w).Encode(errResp)
	}))
	defer server.Close()

	executor := NewRemoteExecutor()
	apiKey := "invalid-key"
	config := &platform.RemoteAgentConfig{
		Endpoint:   server.URL,
		APIKey:     &apiKey,
		Model:      "gpt-4",
		Timeout:    10,
		MaxRetries: 0, // No retries for faster test
	}

	messages := []platform.SessionMessage{
		{Role: "user", Content: "Hello", Timestamp: time.Now().UnixMilli()},
	}

	_, err := executor.Execute(context.Background(), config, messages, nil)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !contains(err.Error(), "Invalid API key") {
		t.Errorf("Expected error message about invalid API key, got %v", err)
	}
}

func TestRemoteExecutor_Execute_Timeout(t *testing.T) {
	// Create mock server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := NewRemoteExecutor()
	apiKey := "test-key"
	config := &platform.RemoteAgentConfig{
		Endpoint:   server.URL,
		APIKey:     &apiKey,
		Model:      "gpt-4",
		Timeout:    1, // 1 second timeout
		MaxRetries: 0,
	}

	messages := []platform.SessionMessage{
		{Role: "user", Content: "Hello", Timestamp: time.Now().UnixMilli()},
	}

	_, err := executor.Execute(context.Background(), config, messages, nil)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}

func TestRemoteExecutor_Health_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatCompletionResponse{
			ID:      "chatcmpl-health",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4",
			Choices: []ChatCompletionChoice{
				{
					Index:        0,
					Message:      ChatMessage{Role: "assistant", Content: "pong"},
					FinishReason: "stop",
				},
			},
			Usage: ChatCompletionUsage{TotalTokens: 2},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	executor := NewRemoteExecutor()
	apiKey := "test-key"
	config := &platform.RemoteAgentConfig{
		Endpoint: server.URL,
		APIKey:   &apiKey,
		Model:    "gpt-4",
		Timeout:  10,
	}

	status, err := executor.Health(context.Background(), config)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !status.Healthy {
		t.Errorf("Expected healthy status, got unhealthy: %s", status.Error)
	}

	if status.LatencyMs < 0 {
		t.Errorf("Expected non-negative latency, got %d", status.LatencyMs)
	}
}

func TestRemoteExecutor_ValidateConfig(t *testing.T) {
	executor := NewRemoteExecutor()

	tests := []struct {
		name      string
		config    *platform.RemoteAgentConfig
		expectErr bool
	}{
		{
			name:      "nil config",
			config:    nil,
			expectErr: true,
		},
		{
			name: "missing endpoint",
			config: &platform.RemoteAgentConfig{
				Model: "gpt-4",
			},
			expectErr: true,
		},
		{
			name: "missing model",
			config: &platform.RemoteAgentConfig{
				Endpoint: "https://api.openai.com",
			},
			expectErr: true,
		},
		{
			name: "non-https endpoint",
			config: &platform.RemoteAgentConfig{
				Endpoint: "http://example.com",
				Model:    "gpt-4",
			},
			expectErr: true,
		},
		{
			name: "localhost http allowed",
			config: &platform.RemoteAgentConfig{
				Endpoint: "http://localhost:8080",
				Model:    "gpt-4",
			},
			expectErr: false,
		},
		{
			name: "negative timeout",
			config: &platform.RemoteAgentConfig{
				Endpoint: "https://api.openai.com",
				Model:    "gpt-4",
				Timeout:  -1,
			},
			expectErr: true,
		},
		{
			name: "timeout too large",
			config: &platform.RemoteAgentConfig{
				Endpoint: "https://api.openai.com",
				Model:    "gpt-4",
				Timeout:  700,
			},
			expectErr: true,
		},
		{
			name: "valid config",
			config: &platform.RemoteAgentConfig{
				Endpoint:   "https://api.openai.com/v1/chat/completions",
				Model:      "gpt-4",
				Timeout:    180,
				MaxRetries: 3,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.validateConfig(tt.config)
			if tt.expectErr && err == nil {
				t.Errorf("Expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestRemoteExecutor_ResolveAPIKey(t *testing.T) {
	executor := NewRemoteExecutor()

	// Set test environment variable
	t.Setenv("TEST_API_KEY", "secret-key-123")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain API key",
			input:    "sk-12345",
			expected: "sk-12345",
		},
		{
			name:     "environment variable",
			input:    "${TEST_API_KEY}",
			expected: "secret-key-123",
		},
		{
			name:     "non-existent env var",
			input:    "${NONEXISTENT}",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.resolveAPIKey(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestRemoteExecutor_IsRetriableError(t *testing.T) {
	executor := NewRemoteExecutor()

	tests := []struct {
		name      string
		err       error
		retriable bool
	}{
		{
			name:      "nil error",
			err:       nil,
			retriable: false,
		},
		{
			name:      "timeout error",
			err:       context.DeadlineExceeded,
			retriable: true,
		},
		{
			name:      "auth error (4xx)",
			err:       &testError{"API error (status 401)"},
			retriable: false,
		},
		{
			name:      "server error (5xx)",
			err:       &testError{"API error (status 500)"},
			retriable: true,
		},
		{
			name:      "rate limit error",
			err:       &testError{"API error (status 429)"},
			retriable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.isRetriableError(tt.err)
			if result != tt.retriable {
				t.Errorf("Expected retriable=%v, got %v", tt.retriable, result)
			}
		})
	}
}

// Helper types and functions

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

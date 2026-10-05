package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"memdoor/pkg/platform"
	"memdoor/pkg/secrets"
)

// RemoteExecutor implements platform.RemoteAgentExecutor for OpenAI-compatible endpoints
type RemoteExecutor struct {
	client *http.Client
}

// NewRemoteExecutor creates a new remote agent executor
func NewRemoteExecutor() *RemoteExecutor {
	return &RemoteExecutor{
		client: &http.Client{
			Timeout: 180 * time.Second, // Default timeout, overridden per request
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Execute sends a message to a remote agent and returns the response
func (e *RemoteExecutor) Execute(
	ctx context.Context,
	config *platform.RemoteAgentConfig,
	messages []platform.SessionMessage,
	tools []platform.ToolDefinition,
) (*platform.RemoteExecutionResult, error) {
	// Validate configuration
	if err := e.validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid remote agent config: %w", err)
	}

	// Translate platform messages to OpenAI format
	openaiMessages := e.translateMessagesToOpenAI(messages)

	// Build request
	// Thinking models (Gemma 4) need more tokens when tools are present
	// because reasoning consumes tokens before the actual response
	maxTokens := 2048
	req := ChatCompletionRequest{
		Model:     config.Model,
		Messages:  openaiMessages,
		MaxTokens: &maxTokens,
	}
	if config.Temperature > 0 {
		t := config.Temperature
		req.Temperature = &t
	}
	if config.TopP > 0 {
		p := config.TopP
		req.TopP = &p
	}
	if config.FrequencyPenalty > 0 {
		fp := config.FrequencyPenalty
		req.FrequencyPenalty = &fp
	}

	// Add tools if provided
	if len(tools) > 0 {
		req.Tools = e.translateToolsToOpenAI(tools)
		maxTokens = 4096 // More room for thinking + tool calls + response
		req.MaxTokens = &maxTokens
		if config.ToolChoice != "" {
			req.ToolChoice = config.ToolChoice
		}
	}

	// Apply timeout from config
	timeout := time.Duration(config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 180 * time.Second // Default
	}

	// Security: Enforce maximum timeout (600s as per security requirements)
	if timeout > 600*time.Second {
		timeout = 600 * time.Second
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute request with retry logic
	maxRetries := config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3 // Default
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctxWithTimeout.Done():
				return nil, fmt.Errorf("request cancelled during retry backoff: %w", ctxWithTimeout.Err())
			case <-time.After(backoff):
			}
		}

		result, err := e.executeRequest(ctxWithTimeout, config, &req)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Don't retry on certain errors (e.g., invalid request, auth failure)
		if !e.isRetriableError(err) {
			break
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries+1, lastErr)
}

// Health checks if a remote endpoint is healthy and responding
func (e *RemoteExecutor) Health(ctx context.Context, config *platform.RemoteAgentConfig) (*platform.HealthStatus, error) {
	// Validate configuration
	if err := e.validateConfig(config); err != nil {
		return &platform.HealthStatus{
			Healthy: false,
			Error:   fmt.Sprintf("invalid config: %v", err),
		}, nil
	}

	// Send minimal health check request
	req := ChatCompletionRequest{
		Model: config.Model,
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: "ping",
			},
		},
		MaxTokens: intPtr(1), // Minimal response to test connectivity
	}

	start := time.Now()

	// Use short timeout for health checks
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := e.executeRequest(ctxWithTimeout, config, &req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return &platform.HealthStatus{
			Healthy:   false,
			LatencyMs: latency,
			Error:     err.Error(),
		}, nil
	}

	return &platform.HealthStatus{
		Healthy:   true,
		LatencyMs: latency,
		Error:     "",
		Metadata: map[string]interface{}{
			"endpoint": config.Endpoint,
			"model":    config.Model,
		},
	}, nil
}

// executeRequest performs a single HTTP request to the remote endpoint
func (e *RemoteExecutor) executeRequest(
	ctx context.Context,
	config *platform.RemoteAgentConfig,
	req *ChatCompletionRequest,
) (*platform.RemoteExecutionResult, error) {
	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")

	// Add API key if provided
	if config.APIKey != nil {
		apiKey := e.resolveAPIKey(*config.APIKey)
		if apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		}
	}

	// Add custom headers
	for key, value := range config.CustomHeaders {
		httpReq.Header.Set(key, value)
	}

	// Execute request
	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Security: Limit response size to 10MB
	limitedReader := io.LimitReader(resp.Body, 10*1024*1024)
	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error responses
	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err != nil {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}
		return nil, fmt.Errorf("API error: %s (type: %s)", errResp.Error.Message, errResp.Error.Type)
	}

	// Parse success response
	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Validate response
	if err := e.validateResponse(&chatResp); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	// Translate response to platform format
	return e.translateResponseToPlatform(&chatResp), nil
}

// validateConfig validates remote agent configuration
func (e *RemoteExecutor) validateConfig(config *platform.RemoteAgentConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	if config.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}

	// Validate endpoint is HTTPS (security requirement)
	if !strings.HasPrefix(config.Endpoint, "https://") && !strings.HasPrefix(config.Endpoint, "http://localhost") && !strings.HasPrefix(config.Endpoint, "http://127.0.0.1") {
		return fmt.Errorf("endpoint must use HTTPS (or localhost for testing)")
	}

	if config.Model == "" {
		return fmt.Errorf("model is required")
	}

	// Validate timeout
	if config.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}

	if config.Timeout > 600 {
		return fmt.Errorf("timeout cannot exceed 600 seconds")
	}

	// Validate retries
	if config.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative")
	}

	if config.MaxRetries > 10 {
		return fmt.Errorf("max_retries cannot exceed 10")
	}

	return nil
}

// validateResponse validates the OpenAI response
func (e *RemoteExecutor) validateResponse(resp *ChatCompletionResponse) error {
	if resp == nil {
		return fmt.Errorf("response is nil")
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no choices in response")
	}

	// Validate first choice (we only use the first one)
	choice := resp.Choices[0]
	if choice.Message.Content == "" && choice.Message.ReasoningContent == "" && len(choice.Message.ToolCalls) == 0 {
		return fmt.Errorf("choice has no content or tool calls")
	}

	return nil
}

// resolveAPIKey resolves an API key from environment variable or SecretRef.
// Supports: plain strings, ${VAR_NAME} env references, and SecretRef JSON.
func (e *RemoteExecutor) resolveAPIKey(apiKey string) string {
	// Check if it's an environment variable reference: ${VAR_NAME}
	if strings.HasPrefix(apiKey, "${") && strings.HasSuffix(apiKey, "}") {
		varName := strings.TrimSuffix(strings.TrimPrefix(apiKey, "${"), "}")
		return os.Getenv(varName)
	}

	// Try resolving as SecretRef JSON
	resolver := secrets.DefaultResolver()
	if resolved, err := resolver.ResolveString(context.Background(), apiKey); err == nil {
		return resolved
	}

	return apiKey
}

// isRetriableError determines if an error is transient and should be retried
func (e *RemoteExecutor) isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context deadline exceeded
	if err == context.DeadlineExceeded {
		return true
	}

	errStr := err.Error()

	// Retry on network errors
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "temporary failure") ||
		strings.Contains(errStr, "EOF") {
		return true
	}

	// Retry on 5xx server errors
	if strings.Contains(errStr, "status 5") {
		return true
	}

	// Retry on rate limiting
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") {
		return true
	}

	// Don't retry on 4xx client errors (except 429)
	if strings.Contains(errStr, "status 4") {
		return false
	}

	return false
}

// Helper functions

func intPtr(i int) *int {
	return &i
}

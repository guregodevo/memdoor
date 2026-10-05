package platform

import (
	"context"

	"memdoor/pkg/shared"
)

// Package platform defines the Published Interface for the Multi-Agent Platform
//
// The platform exposes THREE stable interfaces for different consumer types:
//
// 1. HTTP REST API (platform/http/interface.go)
//    - Purpose: Request-response operations
//    - Consumers: CLI, external integrations, scripts
//    - Contract: RESTful endpoints (/api/channels, /api/messages, /api/agents)
//    - Implementation: gateway/chat_server.go
//
// 2. WebSocket Protocol (platform/ws/interface.go)
//    - Purpose: Real-time bidirectional communication
//    - Consumers: TUI, Web UI, real-time integrations
//    - Contract: WebSocket messages with protocol.Message format
//    - Implementation: gateway/server_websocket.go, gateway/client/websocket.go
//
// 3. Go Interface (THIS FILE)
//    - Purpose: Go code integration
//    - Consumers: Internal services, future Go SDKs
//    - Contract: AgentExecutor, RemoteAgentExecutor interfaces
//    - Implementation: gateway/platform/agent_runtime.go
//
// BACKWARDS COMPATIBILITY GUARANTEE:
// All three interfaces are stable contracts for downstream consumers.
// Internal implementation may change, but interfaces will remain backwards compatible.
//
// Breaking changes to ANY interface require:
// - Major version bump
// - Migration guide
// - Deprecation period (minimum 3 months)

// AgentExecutor is the main interface for executing AI agents
//
// This is the primary entry point for downstream consumers (Team Collaboration domain)
// to trigger agent executions.
//
// Implementations: gateway.AgentRuntime
type AgentExecutor interface {
	// ProcessMessage executes an agent with a user message
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - userMessage: The user's input message
	//   - session: Conversation session containing history
	//   - runID: Unique identifier for this execution run
	//   - extraSystemPrompt: Optional additional system instructions
	//
	// Returns:
	//   - ExecutionResult: The agent's response and execution metadata
	//   - error: Error if execution failed
	//
	// Guarantees:
	//   - Thread-safe (can be called concurrently)
	//   - Idempotent for same runID (same input → same output)
	//   - Events emitted via EventStream() before return
	ProcessMessage(
		ctx context.Context,
		userMessage string,
		session SessionContext,
		runID string,
		extraSystemPrompt string,
	) (*ExecutionResult, error)

	// EventStream returns the event stream for real-time progress updates
	//
	// Returns a stream that emits shared.Event for execution progress.
	// Consumers should listen to this stream before calling ProcessMessage.
	//
	// Event types emitted:
	//   - EventExecutionStarted: Execution began
	//   - EventThinkingStarted/Completed: Agent reasoning
	//   - EventToolCallStarted/Completed/Failed: Tool invocations
	//   - EventResponseGenerated: Final response
	//   - EventExecutionCompleted/Failed: Execution finished
	//
	// Guarantees:
	//   - Events emitted in chronological order
	//   - All events for a runID emitted before ProcessMessage returns
	//   - Stream remains valid for lifetime of executor
	EventStream() EventStream
}

// SessionContext represents a conversation session
//
// This is a simplified view of gateway.Session for the Published Interface.
// Consumers provide session state, platform maintains it internally.
type SessionContext interface {
	// GetID returns the unique session identifier
	GetID() string

	// GetType returns the session type ("main", "channel", "group")
	GetType() string

	// GetMessages returns the conversation history
	// Returns SessionMessage entries in chronological order
	GetMessages() []SessionMessage

	// GetMetadata returns session-specific metadata
	GetMetadata() map[string]interface{}
}

// SessionMessage represents a single message in conversation history
//
// This matches the structure used by gateway.SessionMessage
type SessionMessage struct {
	// Role identifies who sent the message
	// Values: "user" (human), "assistant" (agent), "system" (platform)
	Role string

	// Content is the message text
	Content string

	// Timestamp is when the message was created (Unix milliseconds)
	// Compatible with JavaScript: new Date(timestamp)
	Timestamp int64

	// ToolCalls holds tool invocations for assistant messages (remote agent tool loop)
	ToolCalls []RemoteToolCall `json:"tool_calls,omitempty"`

	// ToolCallID links a tool result message to its tool call (remote agent tool loop)
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ExecutionResult contains the agent's response and execution metadata
//
// Returned by ProcessMessage after execution completes
type ExecutionResult struct {
	// Text is the agent's response text
	Text string

	// ToolsExecuted lists all tools invoked during execution
	// Useful for audit trails and debugging
	ToolsExecuted []ToolExecution

	// Model is the model that answered the turn's last inference, as the
	// vendor names it — shown to the person and logged per turn.
	Model string

	// Error contains error message if execution failed
	// Empty string if successful
	Error string

	// Metadata contains execution-specific data
	// Examples: token usage, execution time, model used
	Metadata map[string]interface{}
}

// ToolExecution tracks a single tool invocation
//
// Used for audit trails, debugging, and billing
type ToolExecution struct {
	// Name is the tool identifier (e.g., "web_search", "read_file")
	Name string

	// Input is the tool parameters (JSON string)
	Input string

	// Output is the tool result (JSON string or plain text)
	Output string

	// Error contains error message if tool failed
	// Empty string if successful
	Error string

	// DurationMs is execution time in milliseconds
	DurationMs int64
}

// EventStream provides real-time execution progress events
//
// Consumers subscribe to this stream to receive updates as agent executes.
// This enables real-time UI updates, progress indicators, and logging.
type EventStream interface {
	// Subscribe registers a listener for execution events
	//
	// The listener function is called for each event.
	// Events are delivered in chronological order.
	//
	// Parameters:
	//   - runID: Filter events for specific execution (empty = all executions)
	//   - listener: Callback function receiving events
	//
	// Returns:
	//   - Unsubscribe function to stop receiving events
	//
	// Guarantees:
	//   - Listener called synchronously (events not buffered)
	//   - Events delivered in order for each runID
	//   - Safe to subscribe/unsubscribe concurrently
	Subscribe(runID string, listener func(event shared.Event)) func()

	// Emit sends an event to all subscribers
	//
	// Platform-internal method (not for consumers).
	// Included in interface for completeness.
	Emit(runID string, event shared.Event)
}

// AgentConfiguration represents agent-specific settings
//
// Used to customize agent behavior, tools, and model selection.
// Maps to config.AgentConfig internally.
type AgentConfiguration struct {
	// ID is the unique agent identifier
	ID string

	// Name is the human-readable agent name
	Name string

	// Profile is the tool access profile
	// Values: "minimal", "coding", "messaging", "full"
	// See config.ToolProfileID for details
	Profile string

	// Model is the LLM model to use
	// Examples: "claude-sonnet-4-5", "claude-opus-4"
	// Empty string uses platform default
	Model string

	// SystemPrompt is optional custom instructions for the agent
	// Prepended to all conversations
	SystemPrompt string

	// AllowedTools explicitly lists allowed tools (overrides profile)
	// Empty list means use profile defaults
	AllowedTools []string

	// DeniedTools explicitly lists denied tools (overrides profile)
	// Takes precedence over AllowedTools
	DeniedTools []string

	// Metadata contains agent-specific configuration
	// Examples: temperature, max_tokens, custom parameters
	Metadata map[string]interface{}
}

// RuntimeFactory creates AgentExecutor instances
//
// Factory interface for dependency injection and testing
type RuntimeFactory interface {
	// CreateExecutor creates a new agent executor
	//
	// Parameters:
	//   - apiKey: Anthropic API key
	//   - config: Agent-specific configuration
	//
	// Returns:
	//   - AgentExecutor: Ready-to-use executor
	//   - error: Error if initialization failed
	//
	// Guarantees:
	//   - Each executor is independent (no shared state)
	//   - Safe to create multiple executors concurrently
	CreateExecutor(apiKey string, config *AgentConfiguration) (AgentExecutor, error)
}

// VERSIONING AND COMPATIBILITY
//
// This Published Interface follows Semantic Versioning:
//
// - MAJOR version: Incompatible API changes (breaking changes)
// - MINOR version: Backwards compatible functionality additions
// - PATCH version: Backwards compatible bug fixes
//
// Current Version: 1.0.0
//
// Compatibility Promise:
// - 1.x.x versions will remain backwards compatible
// - Deprecations announced 3 months before removal
// - Migration guides provided for breaking changes
// - Security fixes backported to supported versions
//
// Supported Versions:
// - 1.0.x: Current stable (guaranteed support)
// - Future releases: TBD based on adoption

// RemoteAgentExecutor executes agents on remote endpoints using OpenAI-compatible protocol
//
// This interface lets an agent run on an OpenAI-compatible endpoint
// (a third-party service or a custom deployment).
//
// Implementations: gateway.RemoteAgentRuntime (to be implemented in Phase 2)
type RemoteAgentExecutor interface {
	// Execute sends a message to a remote agent and returns the response
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - config: Remote agent configuration (endpoint, API key, model)
	//   - messages: Conversation history in platform format
	//   - tools: Optional tool definitions for remote agent
	//
	// Returns:
	//   - RemoteExecutionResult: The remote agent's response
	//   - error: Error if execution failed (network, timeout, etc.)
	//
	// Guarantees:
	//   - Thread-safe (can be called concurrently)
	//   - Respects timeout in config
	//   - Validates response format (OpenAI-compatible)
	//   - Security validated (HTTPS, rate limits, etc.)
	Execute(
		ctx context.Context,
		config *RemoteAgentConfig,
		messages []SessionMessage,
		tools []ToolDefinition,
	) (*RemoteExecutionResult, error)

	// Health checks if a remote endpoint is healthy and responding
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - config: Remote agent configuration to test
	//
	// Returns:
	//   - HealthStatus: Health check result with latency and errors
	//   - error: Error if health check failed
	//
	// Use this before adding a remote agent to verify connectivity
	Health(ctx context.Context, config *RemoteAgentConfig) (*HealthStatus, error)
}

// RemoteAgentConfig holds configuration for remote agents
//
// This matches pkg/domain/RemoteAgentConfig for compatibility
type RemoteAgentConfig struct {
	// Endpoint is the OpenAI-compatible API URL
	// Example: "https://api.openai.com/v1/chat/completions"
	Endpoint string

	// APIKey is optional authentication token
	// Can reference environment variable: "${OPENAI_API_KEY}"
	APIKey *string

	// Model is the model name to pass in requests
	// Example: "gpt-4", "custom-model-v1"
	Model string

	// Timeout is request timeout in seconds (default: 180s)
	Timeout int

	// MaxRetries is retry count for failed requests (default: 3)
	MaxRetries int

	// CustomHeaders are additional HTTP headers
	// Example: {"X-API-Version": "v1"}
	CustomHeaders map[string]string

	// Sampling parameters (optional — let endpoint use defaults if zero)
	Temperature      float64 // Sampling temperature (0.0 = deterministic, 1.0 = default)
	TopP             float64 // Nucleus sampling
	FrequencyPenalty float64 // Penalize repetition (0.0 to 2.0)

	// ToolChoice controls how the model decides to call tools.
	// Valid values: "" (default/auto), "auto", "required", "none".
	// Use "required" for fine-tuned models that don't reliably pick tools via "auto".
	ToolChoice string
}

// ToolDefinition represents a tool available to agents
//
// Used for both local and remote agent tool calls
type ToolDefinition struct {
	// Name is the unique tool identifier
	Name string

	// Description explains what the tool does
	Description string

	// InputSchema is JSON schema for tool parameters
	InputSchema interface{}
}

// RemoteExecutionResult contains the remote agent's response
//
// Returned by RemoteAgentExecutor.Execute()
type RemoteExecutionResult struct {
	// Text is the agent's response text
	Text string

	// ToolCalls lists any tool invocations requested by remote agent
	// Platform will execute these and send results back
	ToolCalls []RemoteToolCall

	// Usage tracks token consumption for billing
	Usage TokenUsage

	// Metadata contains execution-specific data
	// Examples: model used, latency, response ID
	Metadata map[string]interface{}
}

// RemoteToolCall represents a tool invocation from remote agent
//
// Remote agents can request tool execution via OpenAI tool_calls format
type RemoteToolCall struct {
	// ID is the unique call identifier (for matching results)
	ID string

	// Name is the tool to invoke
	Name string

	// Arguments is JSON-encoded tool parameters
	Arguments string
}

// TokenUsage tracks token consumption for billing and metrics
//
// Compatible with OpenAI usage format
type TokenUsage struct {
	// PromptTokens is input tokens consumed
	PromptTokens int

	// CompletionTokens is output tokens generated
	CompletionTokens int

	// TotalTokens is sum of prompt + completion
	TotalTokens int
}

// RemoteToolExecutor provides tool dispatch for remote agents
//
// Remote agents (vLLM, OpenAI-compatible endpoints) can request tool execution
// via tool_calls. This interface allows the domain layer to dispatch those calls
// to the infrastructure layer without importing gateway packages.
//
// Implemented by gateway/AgentRuntime (duck typing).
type RemoteToolExecutor interface {
	// GetToolDefinitions returns platform-format tool definitions for a remote agent.
	// agentTools is the agent's allowed tool list (from buddy.Tools).
	// Returns only tools the agent is authorized to use.
	GetToolDefinitions(ctx context.Context, agentTools []string) []ToolDefinition

	// ExecuteToolCall executes a single tool call and returns the result string.
	// toolName is the tool to invoke, arguments is JSON-encoded parameters.
	ExecuteToolCall(ctx context.Context, toolName string, arguments string) (string, error)
}

// HealthStatus represents the health check result for a remote endpoint
//
// Used by RemoteAgentExecutor.Health()
type HealthStatus struct {
	// Healthy indicates if the endpoint is responding correctly
	Healthy bool

	// LatencyMs is round-trip time in milliseconds
	LatencyMs int64

	// Error contains error message if unhealthy
	// Empty string if healthy
	Error string

	// Metadata contains health check details
	// Examples: endpoint version, capabilities
	Metadata map[string]interface{}
}

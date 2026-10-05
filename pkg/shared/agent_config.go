package shared

// AgentCapabilities represents the core configuration for an AI agent
// SHARED KERNEL: Used by both Team Collaboration and Multi-Agent Platform contexts
//
// This struct captures the essential capabilities of an agent that must remain
// consistent across both contexts to maintain model integrity.
//
// Team Collaboration uses this for:
//   - Displaying agent capabilities in UI
//   - Routing messages to appropriate agents
//   - Validating agent can handle requests
//
// Multi-Agent Platform uses this for:
//   - Runtime execution configuration
//   - Tool filtering and availability
//   - Model selection and parameters
type AgentCapabilities struct {
	// ID uniquely identifies the agent
	ID string

	// Name is the display name of the agent
	Name string

	// Skills are domain-specific capabilities (e.g., "content_writing", "data_analysis")
	// Used for agent selection and system prompt generation
	Skills []string

	// Tools are the specific tools this agent can use (e.g., "web_search", "bash")
	// Platform will filter available tools based on this list
	Tools []string

	// Model is the AI model identifier (e.g., "claude-sonnet-4-5", "gpt-4")
	Model string

	// Temperature controls randomness in model responses (0.0 - 2.0)
	// Lower values make output more focused and deterministic
	Temperature float64
}

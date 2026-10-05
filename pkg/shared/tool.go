package shared

// ToolMetadata represents the metadata for a tool definition
// SHARED KERNEL: Used by both Team Collaboration (display) and Multi-Agent Platform (execution)
//
// Team Collaboration uses this for:
//   - Displaying tool capabilities in agent profiles
//   - Tracking which tools an agent can use
//   - Audit trail of tool invocations
//
// Multi-Agent Platform uses this for:
//   - Tool registration and discovery
//   - Anthropic API tool parameter generation
//   - Tool filtering and access control
//
// Note: The actual execution function is platform-specific and NOT part of shared kernel
type ToolMetadata struct {
	// Name is the unique identifier for the tool (e.g., "web_search", "read_file")
	Name string

	// Description explains what the tool does (shown to the LLM and in UI)
	Description string

	// InputSchema defines the JSON schema for tool parameters
	// This is stored as a generic map to avoid coupling to Anthropic SDK
	// Platform code converts this to anthropic.ToolInputSchemaParam
	InputSchema ToolInputSchema

	// Category groups tools for UI organization and access control
	// Examples: "filesystem", "web", "memory", "sessions", "runtime"
	Category ToolCategory
}

// ToolInputSchema represents the JSON schema for tool input parameters
// This is a simplified representation that both contexts can use
type ToolInputSchema struct {
	// Properties defines the parameters the tool accepts
	Properties map[string]interface{}

	// Required lists which parameters are mandatory
	Required []string
}

// ToolCategory classifies tools into broad groups for UI rendering and access control
// SHARED KERNEL: Unified tool classification across both contexts
type ToolCategory string

const (
	ToolCategoryFilesystem ToolCategory = "filesystem" // read_file, write_file, edit_file, etc.
	ToolCategoryWeb        ToolCategory = "web"        // web_search, web_fetch
	ToolCategoryMemory     ToolCategory = "memory"     // Memory storage and retrieval
	ToolCategoryRuntime    ToolCategory = "runtime"    // bash, agent_log
	ToolCategorySessions   ToolCategory = "sessions"   // sessions_spawn, sessions_send, etc.
	ToolCategoryUI         ToolCategory = "ui"         // browser, canvas, graph
	ToolCategoryContext    ToolCategory = "context"    // context, status, execution_flow
	ToolCategoryData       ToolCategory = "data"       // BigQuery, Styx, Hades
	ToolCategoryOther      ToolCategory = "other"      // Uncategorized tools
)

// ToolRegistry maintains a catalog of available tools
// SHARED KERNEL: Both contexts can query tool availability
type ToolRegistry struct {
}

package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"memdoor/gateway/memory"
	"memdoor/pkg/shared"
)

// ===============================================
// TOOL: memory
// ===============================================

// MemoryInput represents the input for memory tool
type MemoryInput struct {
	Action   string `json:"action" jsonschema_description:"Action to perform: 'store', 'search', 'retrieve', 'list', 'update', 'delete'"`
	Content  string `json:"content,omitempty" jsonschema_description:"Content to store (required for 'store' action)"`
	Query    string `json:"query,omitempty" jsonschema_description:"Search query (required for 'search' action)"`
	ID       string `json:"id,omitempty" jsonschema_description:"Memory ID (required for 'retrieve' and 'delete' actions)"`
	Tags     string `json:"tags,omitempty" jsonschema_description:"Comma-separated tags (optional for 'store', filter for 'list')"`
	Metadata string `json:"metadata,omitempty" jsonschema_description:"JSON metadata object (optional for 'store')"`
	Limit    int    `json:"limit,omitempty" jsonschema_description:"Maximum number of results (default: 10 for search, unlimited for list)"`
}

var MemoryInputSchema = GenerateSchema[MemoryInput]()

// MemoryDefinition defines the memory tool
var MemoryDefinition = ToolDefinition{
	Name: "memory",
	Description: `Store and retrieve long-term memories using keyword search (BM25).

Actions:
- store: Save a new memory with optional tags and metadata
- search: Find memories using BM25 keyword search
- retrieve: Get a specific memory by ID
- list: List all memories (optionally filtered by tags)
- update: Update an existing memory's content, tags, or metadata by ID
- delete: Remove a memory by ID

Pattern: OpenClaw's memory tool with BM25 search backend
Storage: Main database (Raft-replicated in cluster mode)

Examples:
  store: {"action": "store", "content": "User prefers dark mode", "tags": "preference,ui"}
  search: {"action": "search", "query": "dark mode", "limit": 5}
  retrieve: {"action": "retrieve", "id": "mem_1234567890"}
  list: {"action": "list", "tags": "preference"}
  update: {"action": "update", "id": "mem_1234567890", "content": "Updated content", "metadata": "{\"success_count\": 5}"}
  delete: {"action": "delete", "id": "mem_1234567890"}`,
	InputSchema: MemoryInputSchema,
	Function:    Memory,
}

// getMemoryStore returns a memory store instance for the specified agent
// Pattern: OpenClaw ~/.openclaw/memory/{agentId}.sqlite
func getMemoryStore(agentID string, verbose bool) (memory.MemoryStore, error) {
	// Default to "main" if no agent ID provided
	if agentID == "" {
		agentID = "main"
	}

	// Memory database path: ~/.memdoor/memory/{agentId}.sqlite
	memoryDir := shared.MemdoorHome("memory")
	dbPath := filepath.Join(memoryDir, fmt.Sprintf("%s.sqlite", agentID))

	// Create memory directory if it doesn't exist
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Create BM25 store
	store, err := memory.NewBM25Store(dbPath, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory store: %w", err)
	}

	return store, nil
}

// Memory implements the memory tool (CLI version - uses "main" agent)
func Memory(input json.RawMessage) (string, error) {
	return MemoryWithAgentID(input, "main", false)
}

// MemoryWithAgentID implements the memory tool with per-agent database support
// Pattern: OpenClaw per-agent memory storage
func MemoryWithAgentID(input json.RawMessage, agentID string, verbose bool) (string, error) {
	store, err := getMemoryStore(agentID, verbose)
	if err != nil {
		return "", fmt.Errorf("failed to initialize memory store: %w", err)
	}
	defer store.Close()
	return MemoryWithStore(input, store)
}

// MemoryWithStore executes the memory tool using the provided store
func MemoryWithStore(input json.RawMessage, store memory.MemoryStore) (string, error) {
	var params MemoryInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	if params.Action == "" {
		return "", fmt.Errorf("action is required")
	}

	switch params.Action {
	case "store":
		return memoryStore(store, params)
	case "search":
		return memorySearch(store, params)
	case "retrieve":
		return memoryRetrieve(store, params)
	case "list":
		return memoryList(store, params)
	case "update":
		return memoryUpdate(store, params)
	case "delete":
		return memoryDelete(store, params)
	default:
		return "", fmt.Errorf("unknown action: %s (valid actions: store, search, retrieve, list, update, delete)", params.Action)
	}
}

// memoryStore handles the 'store' action
func memoryStore(store memory.MemoryStore, params MemoryInput) (string, error) {
	// Validate content
	if params.Content == "" {
		return "", fmt.Errorf("content is required for 'store' action")
	}

	// Parse tags
	var tags []string
	if params.Tags != "" {
		tags = strings.Split(params.Tags, ",")
		// Trim whitespace from each tag
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
	}

	// Parse metadata
	var metadata map[string]interface{}
	if params.Metadata != "" {
		if err := json.Unmarshal([]byte(params.Metadata), &metadata); err != nil {
			return "", fmt.Errorf("invalid metadata JSON: %w", err)
		}
	}

	// Store memory
	id, err := store.Store(params.Content, tags, metadata)
	if err != nil {
		return "", fmt.Errorf("failed to store memory: %w", err)
	}

	// Format response
	response := fmt.Sprintf("Memory stored successfully\nID: %s\nContent: %s", id, params.Content)
	if len(tags) > 0 {
		response += fmt.Sprintf("\nTags: %s", strings.Join(tags, ", "))
	}

	return response, nil
}

// memorySearch handles the 'search' action
func memorySearch(store memory.MemoryStore, params MemoryInput) (string, error) {
	// Validate query
	if params.Query == "" {
		return "", fmt.Errorf("query is required for 'search' action")
	}

	// Set default limit
	limit := params.Limit
	if limit <= 0 {
		limit = 10
	}

	// Search memories
	results, err := store.Search(params.Query, limit)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	// Format results
	if len(results) == 0 {
		return fmt.Sprintf("No memories found for query: %s", params.Query), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d memories for query: %s\n\n", len(results), params.Query))

	for i, result := range results {
		output.WriteString(fmt.Sprintf("%d. [Score: %.2f] %s\n", i+1, result.Score, result.ID))
		output.WriteString(fmt.Sprintf("   Content: %s\n", result.Content))
		if len(result.Tags) > 0 {
			output.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(result.Tags, ", ")))
		}
		output.WriteString(fmt.Sprintf("   Created: %s\n", result.CreatedAt.Format("2006-01-02 15:04:05")))
		if i < len(results)-1 {
			output.WriteString("\n")
		}
	}

	return output.String(), nil
}

// memoryRetrieve handles the 'retrieve' action
func memoryRetrieve(store memory.MemoryStore, params MemoryInput) (string, error) {
	// Validate ID
	if params.ID == "" {
		return "", fmt.Errorf("id is required for 'retrieve' action")
	}

	// Retrieve memory
	mem, err := store.Retrieve(params.ID)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve memory: %w", err)
	}

	// Format response
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Memory: %s\n", mem.ID))
	output.WriteString(fmt.Sprintf("Content: %s\n", mem.Content))
	if len(mem.Tags) > 0 {
		output.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(mem.Tags, ", ")))
	}
	if len(mem.Metadata) > 0 {
		metadataJSON, _ := json.MarshalIndent(mem.Metadata, "", "  ")
		output.WriteString(fmt.Sprintf("Metadata: %s\n", string(metadataJSON)))
	}
	output.WriteString(fmt.Sprintf("Created: %s\n", mem.CreatedAt.Format("2006-01-02 15:04:05")))
	output.WriteString(fmt.Sprintf("Updated: %s\n", mem.UpdatedAt.Format("2006-01-02 15:04:05")))

	return output.String(), nil
}

// memoryList handles the 'list' action
func memoryList(store memory.MemoryStore, params MemoryInput) (string, error) {
	// Parse tags filter
	var tags []string
	if params.Tags != "" {
		tags = strings.Split(params.Tags, ",")
		// Trim whitespace from each tag
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
	}

	// List memories
	memories, err := store.List(tags)
	if err != nil {
		return "", fmt.Errorf("failed to list memories: %w", err)
	}

	// Format results
	if len(memories) == 0 {
		if len(tags) > 0 {
			return fmt.Sprintf("No memories found with tags: %s", strings.Join(tags, ", ")), nil
		}
		return "No memories stored yet", nil
	}

	var output strings.Builder
	if len(tags) > 0 {
		output.WriteString(fmt.Sprintf("Found %d memories with tags: %s\n\n", len(memories), strings.Join(tags, ", ")))
	} else {
		output.WriteString(fmt.Sprintf("Found %d memories\n\n", len(memories)))
	}

	// Apply limit if specified
	limit := len(memories)
	if params.Limit > 0 && params.Limit < limit {
		limit = params.Limit
	}

	for i := 0; i < limit; i++ {
		mem := memories[i]
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, mem.ID))
		output.WriteString(fmt.Sprintf("   Content: %s\n", mem.Content))
		if len(mem.Tags) > 0 {
			output.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(mem.Tags, ", ")))
		}
		output.WriteString(fmt.Sprintf("   Created: %s\n", mem.CreatedAt.Format("2006-01-02 15:04:05")))
		if i < limit-1 {
			output.WriteString("\n")
		}
	}

	if params.Limit > 0 && params.Limit < len(memories) {
		output.WriteString(fmt.Sprintf("\n... and %d more", len(memories)-params.Limit))
	}

	return output.String(), nil
}

// memoryUpdate handles the 'update' action
func memoryUpdate(store memory.MemoryStore, params MemoryInput) (string, error) {
	if params.ID == "" {
		return "", fmt.Errorf("id is required for 'update' action")
	}

	// Parse optional content
	var content *string
	if params.Content != "" {
		content = &params.Content
	}

	// Parse optional tags
	var tags []string
	if params.Tags != "" {
		tags = strings.Split(params.Tags, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
	}

	// Parse optional metadata
	var metadata map[string]interface{}
	if params.Metadata != "" {
		if err := json.Unmarshal([]byte(params.Metadata), &metadata); err != nil {
			return "", fmt.Errorf("invalid metadata JSON: %w", err)
		}
	}

	if content == nil && tags == nil && metadata == nil {
		return "", fmt.Errorf("at least one of content, tags, or metadata is required for 'update' action")
	}

	if err := store.Update(params.ID, content, tags, metadata); err != nil {
		return "", fmt.Errorf("failed to update memory: %w", err)
	}

	response := fmt.Sprintf("Memory updated successfully: %s", params.ID)
	if content != nil {
		response += fmt.Sprintf("\nContent: %s", *content)
	}
	if tags != nil {
		response += fmt.Sprintf("\nTags: %s", strings.Join(tags, ", "))
	}

	return response, nil
}

// memoryDelete handles the 'delete' action
func memoryDelete(store memory.MemoryStore, params MemoryInput) (string, error) {
	// Validate ID
	if params.ID == "" {
		return "", fmt.Errorf("id is required for 'delete' action")
	}

	// Delete memory
	if err := store.Delete(params.ID); err != nil {
		return "", fmt.Errorf("failed to delete memory: %w", err)
	}

	return fmt.Sprintf("Memory deleted successfully: %s", params.ID), nil
}

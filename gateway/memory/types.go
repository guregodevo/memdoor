package memory

import (
	"time"
)

// Memory represents a stored memory record
// Pattern: OpenClaw src/memory/types.ts
type Memory struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Tags      []string               `json:"tags,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// SearchResult represents a memory search result with relevance score
type SearchResult struct {
	Memory
	Score float64 `json:"score"` // Relevance score (higher = more relevant)
}

// MemoryStore defines the interface for memory storage and retrieval
// Pattern: Interface-based design for future extensibility
// Implementations:
//   - BM25Store (Phase 1): SQLite FTS5 keyword search
//   - VectorStore (Phase 2): SQLite + embeddings for semantic search
type MemoryStore interface {
	// Store saves a new memory and returns its ID
	Store(content string, tags []string, metadata map[string]interface{}) (string, error)

	// Search finds memories matching the query
	// Returns results sorted by relevance score
	Search(query string, limit int) ([]SearchResult, error)

	// Retrieve gets a specific memory by ID
	Retrieve(id string) (*Memory, error)

	// List returns all memories, optionally filtered by tags
	List(tags []string) ([]Memory, error)

	// Update modifies an existing memory's content, tags, and/or metadata
	// nil values are left unchanged
	Update(id string, content *string, tags []string, metadata map[string]interface{}) error

	// Delete removes a memory by ID
	Delete(id string) error

	// Close closes the underlying storage
	Close() error
}

// SearchProvider defines the interface for search implementations
// Pattern: Strategy pattern for swappable search backends
type SearchProvider interface {
	// Search performs the search and returns scored results
	Search(query string, limit int) ([]SearchResult, error)

	// Name returns the provider name (e.g., "bm25", "vector")
	Name() string
}

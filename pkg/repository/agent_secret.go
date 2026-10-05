package repository

import "context"

// AgentSecret represents a secret scoped to a specific agent
type AgentSecret struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	Name      string `json:"name"`
	Value     string `json:"-"` // Never serialized to JSON
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
}

// AgentSecretRepository manages per-agent secrets
type AgentSecretRepository interface {
	Set(ctx context.Context, agentID, name, value, createdBy string) error
	Get(ctx context.Context, agentID, name string) (string, error)
	List(ctx context.Context, agentID string) ([]string, error) // Returns names only
	Delete(ctx context.Context, agentID, name string) error
	DeleteAll(ctx context.Context, agentID string) error
	Count(ctx context.Context, agentID string) (int, error)
}

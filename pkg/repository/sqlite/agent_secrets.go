package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"memdoor/pkg/repository"
	"memdoor/pkg/secrets"
)

type agentSecretRepository struct {
	db *sql.DB
}

func newAgentSecretRepository(db *sql.DB) repository.AgentSecretRepository {
	return &agentSecretRepository{db: db}
}

func (r *agentSecretRepository) Set(ctx context.Context, agentID, name, value, createdBy string) error {
	encrypted, err := secrets.Encrypt(value)
	if err != nil {
		return fmt.Errorf("failed to encrypt secret: %w", err)
	}

	id := uuid.New().String()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO agent_secrets (id, agent_id, name, value, created_by)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(agent_id, name) DO UPDATE SET value = excluded.value
	`, id, agentID, name, encrypted, createdBy)
	if err != nil {
		return fmt.Errorf("failed to set agent secret: %w", err)
	}
	return nil
}

func (r *agentSecretRepository) Get(ctx context.Context, agentID, name string) (string, error) {
	var encrypted string
	err := r.db.QueryRowContext(ctx, `
		SELECT value FROM agent_secrets WHERE agent_id = ? AND name = ?
	`, agentID, name).Scan(&encrypted)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("secret %q not found for agent %q", name, agentID)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get agent secret: %w", err)
	}

	value, err := secrets.Decrypt(encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret: %w", err)
	}
	return value, nil
}

func (r *agentSecretRepository) List(ctx context.Context, agentID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT name FROM agent_secrets WHERE agent_id = ? ORDER BY name
	`, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent secrets: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan secret name: %w", err)
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (r *agentSecretRepository) Delete(ctx context.Context, agentID, name string) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM agent_secrets WHERE agent_id = ? AND name = ?
	`, agentID, name)
	if err != nil {
		return fmt.Errorf("failed to delete agent secret: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("secret %q not found for agent %q", name, agentID)
	}
	return nil
}

func (r *agentSecretRepository) DeleteAll(ctx context.Context, agentID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM agent_secrets WHERE agent_id = ?
	`, agentID)
	if err != nil {
		return fmt.Errorf("failed to delete all agent secrets: %w", err)
	}
	return nil
}

func (r *agentSecretRepository) Count(ctx context.Context, agentID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agent_secrets WHERE agent_id = ?
	`, agentID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count agent secrets: %w", err)
	}
	return count, nil
}

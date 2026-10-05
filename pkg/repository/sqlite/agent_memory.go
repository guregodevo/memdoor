package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"memdoor/pkg/domain"
	"memdoor/pkg/repository"
)

type agentMemoryRepository struct {
	db *sql.DB
}

func NewAgentMemoryRepository(db *sql.DB) repository.AgentMemoryRepository {
	return &agentMemoryRepository{db: db}
}

func (r *agentMemoryRepository) Store(ctx context.Context, mem *domain.AgentMemory) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_memories (id, agent_id, content, tags, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, mem.ID, mem.AgentID, mem.Content, mem.Tags, mem.Metadata, mem.CreatedAt, mem.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert agent memory: %w", err)
	}
	return nil
}

func (r *agentMemoryRepository) GetByID(ctx context.Context, id string) (*domain.AgentMemory, error) {
	var mem domain.AgentMemory
	err := r.db.QueryRowContext(ctx, `
		SELECT id, agent_id, content, tags, metadata, created_at, updated_at
		FROM agent_memories WHERE id = ?
	`, id).Scan(&mem.ID, &mem.AgentID, &mem.Content, &mem.Tags, &mem.Metadata, &mem.CreatedAt, &mem.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("memory not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get agent memory: %w", err)
	}
	return &mem, nil
}

func (r *agentMemoryRepository) Search(ctx context.Context, agentID, ftsQuery string, limit int) ([]*domain.AgentMemory, []float64, error) {
	if limit <= 0 {
		limit = 10
	}
	memories, scores, err := r.searchOnce(ctx, agentID, ftsQuery, limit)
	// AN INDEX THAT HAS DRIFTED IS REPAIRABLE, NOT AN ERROR TO SHOW.
	//
	// The FTS table is external-content: its entries hold rowids that must
	// still exist in agent_memories. A migration that rebuilds the table
	// renumbers those rowids, and a write while the sync triggers are
	// dropped is invisible to it — either leaves entries pointing at rows
	// that are gone, and the search fails with "missing row N from content
	// table". Live 2026-09-05 that reached a person as "Tool memory
	// failed". The rows are the truth and the index is derived from them,
	// so rebuild it and ask again.
	if isFTSDrift(err) {
		if _, rerr := r.db.ExecContext(ctx,
			`INSERT INTO agent_memories_fts(agent_memories_fts) VALUES('rebuild')`); rerr == nil {
			memories, scores, err = r.searchOnce(ctx, agentID, ftsQuery, limit)
		}
	}
	return memories, scores, err
}

// searchOnce runs the query and reads it to the end. FTS5 reports a
// drifted index while the rows are being READ, not when the statement is
// issued, so the error can only be judged after iterating.
func (r *agentMemoryRepository) searchOnce(ctx context.Context, agentID, ftsQuery string, limit int) ([]*domain.AgentMemory, []float64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.id, m.agent_id, m.content, m.tags, m.metadata, m.created_at, m.updated_at,
		       -bm25(agent_memories_fts) as score
		FROM agent_memories_fts
		JOIN agent_memories m ON agent_memories_fts.id = m.id
		WHERE agent_memories_fts MATCH ? AND m.agent_id = ?
		ORDER BY bm25(agent_memories_fts)
		LIMIT ?
	`, ftsQuery, agentID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("search agent memories: %w", err)
	}
	defer rows.Close()

	var memories []*domain.AgentMemory
	var scores []float64
	for rows.Next() {
		var mem domain.AgentMemory
		var score float64
		if err := rows.Scan(&mem.ID, &mem.AgentID, &mem.Content, &mem.Tags, &mem.Metadata, &mem.CreatedAt, &mem.UpdatedAt, &score); err != nil {
			return nil, nil, fmt.Errorf("scan agent memory: %w", err)
		}
		memories = append(memories, &mem)
		scores = append(scores, score)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("search agent memories: %w", err)
	}
	return memories, scores, nil
}

func (r *agentMemoryRepository) List(ctx context.Context, agentID string, tags []string) ([]*domain.AgentMemory, error) {
	var query string
	var args []interface{}

	if len(tags) > 0 {
		conditions := make([]string, len(tags))
		for i, tag := range tags {
			conditions[i] = "tags LIKE ?"
			args = append(args, fmt.Sprintf("%%\"%s\"%%", tag))
		}
		query = fmt.Sprintf(`
			SELECT id, agent_id, content, tags, metadata, created_at, updated_at
			FROM agent_memories
			WHERE agent_id = ? AND (%s)
			ORDER BY created_at DESC
		`, strings.Join(conditions, " OR "))
		args = append([]interface{}{agentID}, args...)
	} else {
		query = `
			SELECT id, agent_id, content, tags, metadata, created_at, updated_at
			FROM agent_memories
			WHERE agent_id = ?
			ORDER BY created_at DESC
		`
		args = []interface{}{agentID}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list agent memories: %w", err)
	}
	defer rows.Close()

	var memories []*domain.AgentMemory
	for rows.Next() {
		var mem domain.AgentMemory
		if err := rows.Scan(&mem.ID, &mem.AgentID, &mem.Content, &mem.Tags, &mem.Metadata, &mem.CreatedAt, &mem.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent memory: %w", err)
		}
		memories = append(memories, &mem)
	}
	return memories, rows.Err()
}

func (r *agentMemoryRepository) Update(ctx context.Context, mem *domain.AgentMemory) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE agent_memories SET content = ?, tags = ?, metadata = ?, updated_at = ?
		WHERE id = ?
	`, mem.Content, mem.Tags, mem.Metadata, mem.UpdatedAt, mem.ID)
	if err != nil {
		return fmt.Errorf("update agent memory: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("memory not found: %s", mem.ID)
	}
	return nil
}

func (r *agentMemoryRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM agent_memories WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete agent memory: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("memory not found: %s", id)
	}
	return nil
}

// isFTSDrift reports whether an error is the index disagreeing with the
// rows it indexes, which a rebuild fixes — as opposed to a bad query,
// which it does not.
func isFTSDrift(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "missing row") ||
		strings.Contains(msg, "database disk image is malformed") ||
		strings.Contains(msg, "fts5: corrupt")
}

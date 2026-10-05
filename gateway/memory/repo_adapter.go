package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"memdoor/pkg/domain"
	"memdoor/pkg/repository"
)

// RepoMemoryStore adapts AgentMemoryRepository to the MemoryStore interface.
// All writes go through the repository (which may be Raft-wrapped).
type RepoMemoryStore struct {
	repo    repository.AgentMemoryRepository
	agentID string
}

// NewRepoMemoryStore creates a MemoryStore backed by the main DB repository.
func NewRepoMemoryStore(repo repository.AgentMemoryRepository, agentID string) MemoryStore {
	return &RepoMemoryStore{repo: repo, agentID: agentID}
}

func (s *RepoMemoryStore) Store(content string, tags []string, metadata map[string]interface{}) (string, error) {
	id := fmt.Sprintf("mem_%d", time.Now().UnixNano())
	now := time.Now().Unix()

	tagsJSON, _ := json.Marshal(tags)
	metadataJSON, _ := json.Marshal(metadata)

	mem := &domain.AgentMemory{
		ID:        id,
		AgentID:   s.agentID,
		Content:   content,
		Tags:      string(tagsJSON),
		Metadata:  string(metadataJSON),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Store(context.Background(), mem); err != nil {
		return "", fmt.Errorf("failed to store memory: %w", err)
	}
	return id, nil
}

func (s *RepoMemoryStore) Search(query string, limit int) ([]SearchResult, error) {
	ftsQuery := toFTS5Query(query)
	if ftsQuery == "" {
		return nil, nil
	}

	memories, scores, err := s.repo.Search(context.Background(), s.agentID, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	results := make([]SearchResult, len(memories))
	for i, mem := range memories {
		results[i] = agentMemoryToSearchResult(mem, scores[i])
	}
	return results, nil
}

func (s *RepoMemoryStore) Retrieve(id string) (*Memory, error) {
	mem, err := s.repo.GetByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	m := agentMemoryToMemory(mem)
	return &m, nil
}

func (s *RepoMemoryStore) List(tags []string) ([]Memory, error) {
	memories, err := s.repo.List(context.Background(), s.agentID, tags)
	if err != nil {
		return nil, err
	}
	result := make([]Memory, len(memories))
	for i, mem := range memories {
		result[i] = agentMemoryToMemory(mem)
	}
	return result, nil
}

func (s *RepoMemoryStore) Update(id string, content *string, tags []string, metadata map[string]interface{}) error {
	existing, err := s.repo.GetByID(context.Background(), id)
	if err != nil {
		return err
	}

	if content != nil {
		existing.Content = *content
	}
	if tags != nil {
		tagsJSON, _ := json.Marshal(tags)
		existing.Tags = string(tagsJSON)
	}
	if metadata != nil {
		metadataJSON, _ := json.Marshal(metadata)
		existing.Metadata = string(metadataJSON)
	}
	existing.UpdatedAt = time.Now().Unix()

	return s.repo.Update(context.Background(), existing)
}

func (s *RepoMemoryStore) Delete(id string) error {
	return s.repo.Delete(context.Background(), id)
}

func (s *RepoMemoryStore) Close() error {
	return nil // DB lifecycle managed by the factory
}

func agentMemoryToMemory(mem *domain.AgentMemory) Memory {
	var tags []string
	if err := json.Unmarshal([]byte(mem.Tags), &tags); err != nil {
		tags = []string{}
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(mem.Metadata), &metadata); err != nil {
		metadata = map[string]interface{}{}
	}
	return Memory{
		ID:        mem.ID,
		Content:   mem.Content,
		Tags:      tags,
		Metadata:  metadata,
		CreatedAt: time.Unix(mem.CreatedAt, 0),
		UpdatedAt: time.Unix(mem.UpdatedAt, 0),
	}
}

func agentMemoryToSearchResult(mem *domain.AgentMemory, score float64) SearchResult {
	m := agentMemoryToMemory(mem)
	return SearchResult{Memory: m, Score: score}
}

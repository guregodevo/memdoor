package subagents

import (
	"context"
	"os"
	"sync"
	"testing"

	"memdoor/gateway/logs"
	"memdoor/pkg/domain"
)

func TestMain(m *testing.M) {
	logs.InitGlobalLoggerDefault(false)
	os.Exit(m.Run())
}

// newTestRegistry creates a SubagentRegistry backed by an in-memory mock repository.
func newTestRegistry(t *testing.T) *SubagentRegistry {
	t.Helper()
	return NewSubagentRegistry(&inMemorySubagentRunRepo{
		runs: make(map[string]*domain.SubagentRun),
	}, false)
}

// inMemorySubagentRunRepo implements repository.SubagentRunRepository in memory.
type inMemorySubagentRunRepo struct {
	mu   sync.RWMutex
	runs map[string]*domain.SubagentRun
}

func (r *inMemorySubagentRunRepo) Create(_ context.Context, run *domain.SubagentRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs[run.RunID] = cloneRun(run)
	return nil
}

func (r *inMemorySubagentRunRepo) Update(_ context.Context, run *domain.SubagentRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs[run.RunID] = cloneRun(run)
	return nil
}

func (r *inMemorySubagentRunRepo) GetByID(_ context.Context, runID string) (*domain.SubagentRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if run, ok := r.runs[runID]; ok {
		return cloneRun(run), nil
	}
	return nil, context.DeadlineExceeded // any non-nil error
}

func (r *inMemorySubagentRunRepo) GetByChildSession(_ context.Context, childSessionKey string) (*domain.SubagentRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, run := range r.runs {
		if run.ChildSessionKey == childSessionKey {
			return cloneRun(run), nil
		}
	}
	return nil, context.DeadlineExceeded
}

func (r *inMemorySubagentRunRepo) List(_ context.Context) ([]*domain.SubagentRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.SubagentRun, 0, len(r.runs))
	for _, run := range r.runs {
		result = append(result, cloneRun(run))
	}
	return result, nil
}

func (r *inMemorySubagentRunRepo) Delete(_ context.Context, runID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.runs, runID)
	return nil
}

func (r *inMemorySubagentRunRepo) DeleteArchived(_ context.Context, beforeMs int64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for id, run := range r.runs {
		if run.ArchiveAtMs > 0 && run.ArchiveAtMs <= beforeMs {
			delete(r.runs, id)
			count++
		}
	}
	return count, nil
}

// cloneRun returns a deep copy to avoid shared pointer mutations across reads/writes.
func cloneRun(src *domain.SubagentRun) *domain.SubagentRun {
	cp := *src
	if src.StartedAt != nil {
		t := *src.StartedAt
		cp.StartedAt = &t
	}
	if src.EndedAt != nil {
		t := *src.EndedAt
		cp.EndedAt = &t
	}
	if src.CleanupCompletedAt != nil {
		t := *src.CleanupCompletedAt
		cp.CleanupCompletedAt = &t
	}
	return &cp
}

package cron

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"memdoor/gateway/config"
	"memdoor/gateway/logs"
)

// The scheduler logs through the global logger, which a bare test does not have.
func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "memdoor-cron-test-logs")
	if err := logs.InitGlobalLogger(dir, false); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type memStore struct {
	mu   sync.Mutex
	jobs map[string]*config.CronJob
}

func newMemStore() *memStore { return &memStore{jobs: map[string]*config.CronJob{}} }
func (m *memStore) ListJobs() []*config.CronJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*config.CronJob{}
	for _, j := range m.jobs {
		c := *j
		out = append(out, &c)
	}
	return out
}
func (m *memStore) AddJob(j *config.CronJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := *j
	m.jobs[j.ID] = &c
	return nil
}
func (m *memStore) UpdateJob(j *config.CronJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.jobs[j.ID]; !ok {
		return fmt.Errorf("no job %s", j.ID)
	}
	c := *j
	m.jobs[j.ID] = &c
	return nil
}
func (m *memStore) RemoveJob(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.jobs[id]; !ok {
		return fmt.Errorf("no job %s", id)
	}
	delete(m.jobs, id)
	return nil
}
func (m *memStore) GetJob(id string) (*config.CronJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[id]; ok {
		c := *j
		return &c, nil
	}
	return nil, fmt.Errorf("no job %s", id)
}

// A bounded job runs its count and is then gone — from the scheduler and
// from the store, so a restart does not revive it.
func TestABoundedJobStopsItselfAndIsRemoved(t *testing.T) {
	store := newMemStore()
	var runs int
	var mu sync.Mutex
	exec := func(ctx context.Context, job *config.CronJob, agentID, sessionKey string) error {
		mu.Lock()
		runs++
		mu.Unlock()
		return nil
	}
	s, err := NewScheduler(nil, exec, store, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	job := &config.CronJob{ID: "poll-1", Schedule: "@every 1h", Message: "check", Enabled: true, MaxRuns: 2}
	if err := s.AddJob(job); err != nil {
		t.Fatal(err)
	}
	run := s.createJobWrapper(job, "coder", "s")
	for i := 0; i < 4; i++ {
		run()
	}
	mu.Lock()
	defer mu.Unlock()
	if runs != 2 {
		t.Fatalf("MaxRuns 2 must mean two runs, got %d", runs)
	}
	if _, err := store.GetJob("poll-1"); err == nil {
		t.Fatal("a finished job must leave the store")
	}
	if len(s.Jobs()) != 0 {
		t.Fatalf("a finished job must leave the scheduler: %+v", s.Jobs())
	}
}

// Every run is counted in the store, so a restart resumes the count rather
// than starting again at zero.
func TestEachRunIsCountedInTheStore(t *testing.T) {
	store := newMemStore()
	exec := func(context.Context, *config.CronJob, string, string) error { return nil }
	s, _ := NewScheduler(nil, exec, store, nil, false)
	job := &config.CronJob{ID: "poll-2", Schedule: "@every 1h", Message: "check", Enabled: true, MaxRuns: 10}
	_ = s.AddJob(job)
	run := s.createJobWrapper(job, "coder", "s")
	run()
	run()
	got, _ := store.GetJob("poll-2")
	if got == nil || got.RunCount != 2 {
		t.Fatalf("store must hold run_count 2, got %+v", got)
	}
}

// A job past its expiry does not run again, however many runs it has left.
func TestAnExpiredJobDoesNotRun(t *testing.T) {
	store := newMemStore()
	var runs int
	exec := func(context.Context, *config.CronJob, string, string) error { runs++; return nil }
	s, _ := NewScheduler(nil, exec, store, nil, false)
	job := &config.CronJob{ID: "poll-3", Schedule: "@every 1h", Message: "check", Enabled: true, MaxRuns: 10,
		ExpiresAt: time.Now().Add(-time.Minute)}
	_ = s.AddJob(job)
	s.createJobWrapper(job, "coder", "s")()
	if runs != 0 {
		t.Fatalf("an expired job ran %d times", runs)
	}
	if len(s.Jobs()) != 0 {
		t.Fatal("an expired job must be removed")
	}
}

// A tick that arrives while the previous run is still going is skipped: a
// slow check never piles up beside itself.
func TestARunningJobIsNotStartedAgain(t *testing.T) {
	store := newMemStore()
	hold := make(chan struct{})
	var started int
	var mu sync.Mutex
	exec := func(context.Context, *config.CronJob, string, string) error {
		mu.Lock()
		started++
		mu.Unlock()
		<-hold
		return nil
	}
	s, _ := NewScheduler(nil, exec, store, nil, false)
	job := &config.CronJob{ID: "poll-4", Schedule: "@every 1h", Message: "slow", Enabled: true, MaxRuns: 10}
	_ = s.AddJob(job)
	run := s.createJobWrapper(job, "coder", "s")
	go run()
	time.Sleep(50 * time.Millisecond)
	run() // the overlapping tick: must return at once, without a second start
	mu.Lock()
	n := started
	mu.Unlock()
	if n != 1 {
		t.Fatalf("an overlapping tick started a second run: %d", n)
	}
	close(hold)
}

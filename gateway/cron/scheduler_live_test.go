package cron

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"memdoor/gateway/config"
)

// A job added while the engine is running, with an @every schedule, fires —
// the agent's cron tool adds exactly such jobs, long after Start().
func TestAJobAddedAfterStartFires(t *testing.T) {
	store := newMemStore()
	var runs atomic.Int32
	exec := func(context.Context, *config.CronJob, string, string) error { runs.Add(1); return nil }
	cfg := config.DefaultConfig()
	if cfg.Cron == nil {
		cfg.Cron = &config.CronConfig{}
	}
	cfg.Cron.Enabled = true
	s, err := NewScheduler(cfg, exec, store, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	job := &config.CronJob{ID: "poll-live", Schedule: "@every 1s", Message: "check", Enabled: true, MaxRuns: 2}
	if err := s.AddJob(job); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for runs.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if got := runs.Load(); got != 2 {
		t.Fatalf("a live-added @every job must fire and stop at its bound: ran %d times in 5s", got)
	}
}

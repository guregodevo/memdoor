package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"memdoor/pkg/domain"
)

// A job an agent scheduled carries where to work, where to answer and its
// bounds; the store must keep every one of them, or a restart hands the poll
// a fresh budget and a scratch directory.
func TestCronJobKeepsItsBoundsAndPlace(t *testing.T) {
	f, err := NewSQLiteFactory(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	repo := f.CronJobs()
	ctx := context.Background()

	exp := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	job := &domain.CronJob{ID: "poll-1", WorkspaceID: "w", AgentID: "coder", Schedule: "@every 30s",
		Message: "is CI green?", Enabled: true, Workdir: "/tmp/proj", SessionKey: "workspace:w:channel:c",
		MaxRuns: 5, RunCount: 2, ExpiresAt: exp}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, "poll-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Workdir != "/tmp/proj" || got.SessionKey != "workspace:w:channel:c" || got.MaxRuns != 5 || got.RunCount != 2 || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("round trip lost a field: %+v", got)
	}

	got.RunCount = 3
	if err := repo.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	list, err := repo.List(ctx, "w")
	if err != nil || len(list) != 1 || list[0].RunCount != 3 {
		t.Fatalf("update must persist the run count: err=%v list=%+v", err, list)
	}

	// A job a person configured has none of it and reads back as before.
	plain := &domain.CronJob{ID: "nightly", WorkspaceID: "w", Schedule: "0 9 * * *", Message: "report", Enabled: true}
	if err := repo.Create(ctx, plain); err != nil {
		t.Fatal(err)
	}
	p, _ := repo.GetByID(ctx, "nightly")
	if p.MaxRuns != 0 || !p.ExpiresAt.IsZero() || p.Workdir != "" {
		t.Fatalf("a plain job must stay plain: %+v", p)
	}
}

// A database from before the columns existed must open and take the new jobs.
func TestOldCronTableGetsTheNewColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE cron_jobs (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL DEFAULT 'default', agent_id TEXT NOT NULL DEFAULT '',
		schedule TEXT NOT NULL, message TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1,
		created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO cron_jobs VALUES ('legacy','w','','0 9 * * *','report',1,1,1)`); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	f, err := NewSQLiteFactory(path)
	if err != nil {
		t.Fatalf("an old database must open: %v", err)
	}
	defer f.Close()
	repo := f.CronJobs()
	ctx := context.Background()
	if legacy, err := repo.GetByID(ctx, "legacy"); err != nil || legacy.Message != "report" {
		t.Fatalf("the old row must read: err=%v job=%+v", err, legacy)
	}
	if err := repo.Create(ctx, &domain.CronJob{ID: "poll-2", WorkspaceID: "w", Schedule: "@every 1m", Message: "x",
		Enabled: true, Workdir: "/p", MaxRuns: 3}); err != nil {
		t.Fatalf("a bounded job must insert after migration: %v", err)
	}
}

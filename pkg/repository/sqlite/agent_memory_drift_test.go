package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"memdoor/pkg/domain"
)

// A DRIFTED SEARCH INDEX MUST NOT REACH A USER.
//
// The FTS table is external-content: it holds rowids that must still exist
// in agent_memories. A migration that rebuilds the table renumbers those
// rowids, and a write while the sync triggers are dropped is invisible to
// it — either leaves entries pointing at rows that are gone, and the query
// fails with "missing row N from content table". Live 2026-09-05 that
// surfaced to a person as "Tool memory failed". The rows are the truth and
// the index is derived, so search repairs it and asks again.
func TestSearchRepairsADriftedIndex(t *testing.T) {
	f, err := NewSQLiteFactory(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	repo := f.AgentMemories()
	ctx := context.Background()
	db, ok := f.DB().(*sql.DB)
	if !ok {
		t.Fatal("the factory no longer exposes a *sql.DB")
	}
	// The search index needs FTS5, which the shipped binary is built with
	// (-DSQLITE_ENABLE_FTS5) and a plain `go test` is not.
	if _, err := db.Exec(`INSERT INTO agent_memories_fts(agent_memories_fts) VALUES('rebuild')`); err != nil {
		t.Skip("this build has no FTS5: CGO_CFLAGS=-DSQLITE_ENABLE_FTS5 go test ./pkg/repository/sqlite/")
	}

	for i, c := range []string{"the release notes are in docs-site", "sara prefers small commits"} {
		mem := &domain.AgentMemory{
			ID: fmt.Sprintf("mem-%d", i), AgentID: "planner", Content: c, Tags: "[]",
			CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(),
		}
		if err := repo.Store(ctx, mem); err != nil {
			t.Fatal(err)
		}
	}
	if got, _, err := repo.Search(ctx, "planner", "release", 5); err != nil || len(got) != 1 {
		t.Fatalf("baseline search: %d results, %v", len(got), err)
	}

	// Drift it exactly as a migration does: delete a row behind the index's
	// back, so the index still points at a rowid that is gone.
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS agent_memories_ad`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM agent_memories WHERE content LIKE 'sara%'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER IF NOT EXISTS agent_memories_ad AFTER DELETE ON agent_memories BEGIN
		DELETE FROM agent_memories_fts WHERE rowid = old.rowid;
	END`); err != nil {
		t.Fatal(err)
	}

	// The drifted index must not surface as a failure: search repairs and
	// answers, and the deleted memory is gone from the results.
	got, _, err := repo.Search(ctx, "planner", "sara OR release", 5)
	if err != nil {
		t.Fatalf("a drifted index reached the caller: %v", err)
	}
	if len(got) != 1 || got[0].Content != "the release notes are in docs-site" {
		t.Fatalf("after repair: %d results %+v", len(got), got)
	}
}

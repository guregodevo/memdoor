package migrations

import (
	"io"
	"log/slog"
	"testing"
)

// THE WIKI'S TABLES GO, WHEREVER THEY STILL ARE. A database from before
// 2026-09-27 carries them; one created since never did. The migration drops
// them from the first and must be a no-op on the second.
func TestMigration018DropsTheWikiTables(t *testing.T) {
	db := freshDB(t)
	// An old database: the tables exist, with a row in one of them.
	for _, stmt := range []string{
		`CREATE TABLE wikis (id TEXT PRIMARY KEY, workspace_id TEXT, slug TEXT, name TEXT)`,
		`CREATE TABLE wiki_pages (workspace_id TEXT, slug TEXT, body TEXT)`,
		`CREATE TABLE wiki_findings (id INTEGER PRIMARY KEY, message TEXT)`,
		`CREATE TABLE wiki_claims (workspace_id TEXT, claim_id TEXT)`,
		`CREATE TABLE wiki_page_refs (workspace_id TEXT, src_slug TEXT)`,
		`CREATE TABLE wiki_members (wiki_id TEXT, user_id TEXT)`,
		`CREATE TABLE wiki_settings (wiki_id TEXT, key TEXT)`,
		`INSERT INTO wiki_pages VALUES ('ws', 'page', 'body')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed an old database: %v", err)
		}
	}
	r := NewRunner(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := r.Run(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	for _, table := range []string{"wikis", "wiki_pages", "wiki_findings", "wiki_claims", "wiki_page_refs", "wiki_members", "wiki_settings"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s survived the migration", table)
		}
	}
	// A database without the tables — this one, now — migrates cleanly too.
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = '018_drop_wiki'`); err != nil {
		t.Fatal(err)
	}
	if err := r.Run(); err != nil {
		t.Fatalf("a database without the tables must migrate cleanly: %v", err)
	}
}

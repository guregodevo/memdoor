package migrations

import (
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	sqliterepo "memdoor/pkg/repository/sqlite"

	_ "memdoor/pkg/sqlitedriver"
)

// freshDB returns a *sql.DB backed by a temp-file SQLite with the
// base schema (pkg/repository/sqlite/initSchema) already applied —
// i.e. the same starting state the gateway boots against. Lets the
// migration tests run the full chain end-to-end.
//
// Skips the test when the test-time sqlite driver lacks FTS5: the
// pkg/repository/sqlite factory recreates agent_memories_* triggers
// unconditionally after a DROP COLUMN migration, and a later
// migrations 011 ALTER TABLE re-validates them. Without FTS5 those
// triggers reference a missing virtual table. Production builds (via
// `make build`) link a SQLite with FTS5, so this only bites
// `go test` invocations that skip the build tag. Avoid hiding a real
// regression by failing soft instead of forcing the tag everywhere.
func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	f, err := sqliterepo.NewSQLiteFactory(path)
	if err != nil {
		t.Fatalf("init sqlite factory: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	db, ok := f.DB().(*sql.DB)
	if !ok {
		t.Fatalf("factory.DB() not *sql.DB: %T", f.DB())
	}
	if _, err := db.Exec(`CREATE VIRTUAL TABLE __fts_probe USING fts5(x)`); err != nil {
		t.Skipf("sqlite driver lacks FTS5 — skipping migration chain test (build with -tags sqlite_fts5 to enable): %v", err)
	}
	_, _ = db.Exec(`DROP TABLE __fts_probe`)
	return db
}

// TestMigration014RewritesUUIDDefaultWorkspace exercises the end-to-end
// migration against an in-memory SQLite: run every migration 001..N,
// inject a "pre-fix `memdoor setup`" workspace (legacy UUID id +
// meaningful slug), and assert that migration 014 rewrites id=slug
// and cascades the new id to every FK column.
//
// Catches: SQL syntax errors in the migration, missing FK tables in
// the cascade list, ON CONFLICT clobbering, and the boring case of a
// healthy install accidentally getting touched.
func TestMigration014RewritesUUIDDefaultWorkspace(t *testing.T) {
	db := freshDB(t)

	// Run the full chain — same Runner the gateway boots with.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRunner(db, logger)
	if err := r.Run(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	const legacyUUID = "00000000-0000-0000-0000-000000000001"
	const newSlug = "greg-ws"

	// Inject the broken setup row: legacy UUID id, meaningful slug.
	if _, err := db.Exec(`
		INSERT INTO workspaces (id, name, slug, owner_id, language, plan, status, created_at, updated_at)
		VALUES (?, 'Demo', ?, '', 'en', 'free', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, legacyUUID, newSlug); err != nil {
		t.Fatalf("seed broken workspace: %v", err)
	}

	// Seed a row the cascade is supposed to update: the agent surface
	// the user complained was broken.
	if _, err := db.Exec(`
		INSERT INTO buddies (id, workspace_id, name, avatar_emoji, icon, description, personality, system_prompt, temperature, max_tokens, skills, tools, sandbox_scope, learning_enabled, is_active, created_by, created_at, updated_at)
		VALUES ('agent-x', ?, 'chief', '', '', '', '', '', 0.5, 4096, '[]', '[]', 'workspace', 1, 1, 'system', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, legacyUUID); err != nil {
		t.Fatalf("seed buddies: %v", err)
	}

	// Manually re-run migration 014 by clearing schema_migrations for
	// it and re-invoking the runner. This is the test's "what would
	// happen if the migration ran on this broken DB" path.
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = '014_uuid_default_workspace_to_slug'`); err != nil {
		t.Fatalf("clear migration tracking: %v", err)
	}
	if err := r.Run(); err != nil {
		t.Fatalf("re-run migrations: %v", err)
	}

	// Workspace: id should now equal slug.
	var gotID, gotSlug string
	if err := db.QueryRow(`SELECT id, slug FROM workspaces WHERE slug = ?`, newSlug).Scan(&gotID, &gotSlug); err != nil {
		t.Fatalf("read workspace after migration: %v", err)
	}
	if gotID != newSlug || gotSlug != newSlug {
		t.Errorf("workspace id/slug mismatch: got id=%q slug=%q, want both=%q", gotID, gotSlug, newSlug)
	}
	// And the legacy UUID id should be gone.
	var stale int
	_ = db.QueryRow(`SELECT COUNT(*) FROM workspaces WHERE id = ?`, legacyUUID).Scan(&stale)
	if stale != 0 {
		t.Errorf("legacy UUID workspace row not deleted: %d rows remain", stale)
	}

	// Buddies cascade.
	var buddyWS string
	if err := db.QueryRow(`SELECT workspace_id FROM buddies WHERE id = 'agent-x'`).Scan(&buddyWS); err != nil {
		t.Fatalf("read buddies after migration: %v", err)
	}
	if buddyWS != newSlug {
		t.Errorf("buddies.workspace_id not cascaded: got %q, want %q", buddyWS, newSlug)
	}
}

// TestMigration014SkipsHealthyInstall asserts the migration is a no-op
// when the workspace is already id-equals-slug. Protects every
// cyberlaw / hackernews / acme operator from accidental
// mutation when they upgrade past this migration.
func TestMigration014SkipsHealthyInstall(t *testing.T) {
	db := freshDB(t)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRunner(db, logger)
	if err := r.Run(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO workspaces (id, name, slug, owner_id, language, plan, status, created_at, updated_at)
		VALUES ('cyberlaw', 'Cyber-law', 'cyberlaw', '', 'en', 'free', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`); err != nil {
		t.Fatalf("seed healthy workspace: %v", err)
	}

	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = '014_uuid_default_workspace_to_slug'`); err != nil {
		t.Fatalf("clear migration tracking: %v", err)
	}
	if err := r.Run(); err != nil {
		t.Fatalf("re-run migrations: %v", err)
	}

	var gotID, gotSlug string
	if err := db.QueryRow(`SELECT id, slug FROM workspaces WHERE id = 'cyberlaw'`).Scan(&gotID, &gotSlug); err != nil {
		t.Fatalf("read workspace: %v", err)
	}
	if gotID != "cyberlaw" || gotSlug != "cyberlaw" {
		t.Errorf("healthy workspace mutated: id=%q slug=%q", gotID, gotSlug)
	}
}

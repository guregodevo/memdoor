package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"memdoor/pkg/domain"
	"memdoor/pkg/shared"

	"github.com/google/uuid"
)

// Agents are scoped by workspace (2026-10-20): the name is unique per
// workspace, the table is rebuilt from its stored DDL with every added
// column, ids kept; and every user, channel and agent of an older install
// points at the gateway's oldest workspace instead of the legacy default.
func TestAgentsAreScopedByWorkspaceAndOrphansJoinTheOldestWorkspace(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, name TEXT, slug TEXT, owner_id TEXT, created_at DATETIME)`,
		`INSERT INTO workspaces VALUES ('hackernews', 'Hacker News Best', 'hackernews', 'human:greg', '2026-06-07'), ('bob', 'bob', 'bob', 'human:bob', '2026-10-20')`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO users VALUES ('human:greg', '00000000-0000-0000-0000-000000000001'), ('human:bob', 'bob')`,
		`CREATE TABLE channels (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, name TEXT NOT NULL, UNIQUE (workspace_id, name))`,
		`INSERT INTO channels VALUES ('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-000000000001', 'general')`,
		`CREATE TABLE buddies (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, name TEXT UNIQUE NOT NULL, created_by TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`ALTER TABLE buddies ADD COLUMN admin_only INTEGER DEFAULT 0`,
		`INSERT INTO buddies (id, workspace_id, name, created_by, admin_only) VALUES ('22222222-2222-2222-2222-222222222222', 'hackernews', 'coder', 'system', 1)`,
		`INSERT INTO buddies (id, workspace_id, name, created_by) VALUES ('33333333-3333-3333-3333-333333333333', '', 'chief', 'system')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(stmt, err)
		}
	}
	if err := scopeBuddiesByWorkspace(db); err != nil {
		t.Fatal(err)
	}
	// Seeded before the first workspace existed AND again by the setup in
	// it (the fresh-install sequence of 2026-10-21): the orphan twin is
	// dropped; moving it would trip UNIQUE (workspace_id, name) and the
	// gateway would never boot again.
	for _, stmt := range []string{
		`INSERT INTO buddies (id, workspace_id, name, created_by) VALUES ('44444444-4444-4444-4444-444444444444', 'hackernews', 'runner', 'system')`,
		`INSERT INTO buddies (id, workspace_id, name, created_by) VALUES ('55555555-5555-5555-5555-555555555555', 'default', 'runner', 'system')`,
		`INSERT INTO channels VALUES ('66666666-6666-6666-6666-666666666666', 'hackernews', 'test')`,
		`INSERT INTO channels VALUES ('77777777-7777-7777-7777-777777777777', 'default', 'test')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(stmt, err)
		}
	}
	if err := repairOrphanWorkspaceIDs(db); err != nil {
		t.Fatal(err)
	}
	if err := repairOrphanWorkspaceIDs(db); err != nil {
		t.Fatalf("a second boot: %v", err)
	}
	for _, q := range []string{`SELECT id FROM buddies WHERE name = 'runner'`, `SELECT id FROM channels WHERE name = 'test'`} {
		rows, err := db.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for rows.Next() {
			var id string
			_ = rows.Scan(&id)
			ids = append(ids, id)
		}
		rows.Close()
		if len(ids) != 1 || !(ids[0] == "44444444-4444-4444-4444-444444444444" || ids[0] == "66666666-6666-6666-6666-666666666666") {
			t.Fatalf("%s: the workspace's own row stays, the orphan twin goes: %v", q, ids)
		}
	}
	if err := scopeBuddiesByWorkspace(db); err != nil {
		t.Fatalf("a second run is a no-op: %v", err)
	}
	var ws string
	for _, q := range []string{`SELECT workspace_id FROM users WHERE id = 'human:greg'`, `SELECT workspace_id FROM channels WHERE name = 'general'`, `SELECT workspace_id FROM buddies WHERE name = 'chief'`} {
		if err := db.QueryRow(q).Scan(&ws); err != nil || ws != "hackernews" {
			t.Fatalf("%s → %q %v (the oldest workspace)", q, ws, err)
		}
	}
	if err := db.QueryRow(`SELECT workspace_id FROM users WHERE id = 'human:bob'`).Scan(&ws); err != nil || ws != "bob" {
		t.Fatalf("a user in a real workspace is left alone: %q", ws)
	}
	var adminOnly int
	if err := db.QueryRow(`SELECT admin_only FROM buddies WHERE id = '22222222-2222-2222-2222-222222222222'`).Scan(&adminOnly); err != nil || adminOnly != 1 {
		t.Fatalf("the added column survives the rebuild: %d %v", adminOnly, err)
	}
	if _, err := db.Exec(`INSERT INTO buddies (id, workspace_id, name, created_by) VALUES (?, 'bob', 'coder', 'system')`, uuid.NewString()); err != nil {
		t.Fatalf("bob's own coder: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO buddies (id, workspace_id, name, created_by) VALUES (?, 'bob', 'coder', 'system')`, uuid.NewString()); err == nil {
		t.Fatal("twice the same name in one workspace")
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM buddies WHERE name = 'coder'`).Scan(&n)
	if n != 2 {
		t.Fatalf("a coder per workspace: %d", n)
	}
	_ = context.Background()
	_ = shared.AllPages()
	_ = domain.DefaultWorkspaceID
}

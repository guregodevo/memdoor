package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"memdoor/pkg/domain"
	"memdoor/pkg/shared"

	"github.com/google/uuid"
)

// Channels are scoped by workspace (2026-10-20): the name is unique per
// workspace, a workspace lists its own, and the rows from before (no
// workspace on them) belong to the legacy default workspace.
func TestChannelsAreScopedByWorkspaceAndOldRowsJoinTheLegacyDefault(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// The table as every gateway before 2026-10-20 had it.
	for _, stmt := range []string{
		`CREATE TABLE channels (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, name TEXT UNIQUE NOT NULL, description TEXT, is_private INTEGER DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, archived_at DATETIME)`,
		`INSERT INTO channels (id, workspace_id, name) VALUES ('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-000000000000', 'general')`,
		`INSERT INTO channels (id, workspace_id, name, description) VALUES ('22222222-2222-2222-2222-222222222222', '', 'greg-private', 'mine')`,
		`INSERT INTO channels (id, workspace_id, name) VALUES ('33333333-3333-3333-3333-333333333333', '', 'tui-1020')`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE channel_memberships (channel_id TEXT NOT NULL, actor_id TEXT NOT NULL, role TEXT, joined_at DATETIME)`,
		`INSERT INTO users (id, workspace_id) VALUES ('human:greg', 'hackernews')`,
		`INSERT INTO channel_memberships VALUES ('33333333-3333-3333-3333-333333333333', 'agent:coder', 'member', '2026-10-20 10:00:00')`,
		`INSERT INTO channel_memberships VALUES ('33333333-3333-3333-3333-333333333333', 'human:greg', 'admin', '2026-10-20 10:00:01')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := scopeChannelsByWorkspace(db); err != nil {
		t.Fatal(err)
	}
	if err := scopeChannelsByWorkspace(db); err != nil {
		t.Fatalf("a second run is a no-op: %v", err)
	}
	ctx := context.Background()
	repo := NewChannelRepository(db)

	legacy, err := repo.ListActive(ctx, domain.DefaultWorkspaceID, shared.AllPages())
	if err != nil || len(legacy) != 2 {
		t.Fatalf("the old rows with no human member belong to the legacy default workspace: %v %v", legacy, err)
	}
	gregs, err := repo.ListActive(ctx, "hackernews", shared.AllPages())
	if err != nil || len(gregs) != 1 || gregs[0].Name != "tui-1020" {
		t.Fatalf("an old row goes to its first human member's workspace: %v %v", gregs, err)
	}

	// Bob's workspace gets a general of its own; the name is free there.
	bob := &domain.Channel{ID: uuid.New(), Name: "general", WorkspaceScoped: domain.WorkspaceScoped{WorkspaceID: "bob"}}
	if err := repo.Create(ctx, bob); err != nil {
		t.Fatalf("a second general in another workspace: %v", err)
	}
	if err := repo.Create(ctx, &domain.Channel{ID: uuid.New(), Name: "general", WorkspaceScoped: domain.WorkspaceScoped{WorkspaceID: "bob"}}); err == nil {
		t.Fatal("twice the same name in one workspace")
	}

	bobs, _ := repo.ListActive(ctx, "bob", shared.AllPages())
	if len(bobs) != 1 || bobs[0].Name != "general" || bobs[0].WorkspaceID != "bob" {
		t.Fatalf("bob lists his own: %+v", bobs)
	}
	all, _ := repo.ListActive(ctx, "", shared.AllPages())
	if len(all) != 4 {
		t.Fatalf("the gateway's own caller lists all: %d", len(all))
	}
	if got, err := repo.GetByName(ctx, "bob", "general"); err != nil || got.ID != bob.ID {
		t.Fatalf("by name in bob's workspace: %+v %v", got, err)
	}
	if got, err := repo.GetByName(ctx, domain.DefaultWorkspaceID, "general"); err != nil || got.ID.String() != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("by name in the legacy workspace: %+v %v", got, err)
	}
	if _, err := repo.GetByName(ctx, "bob", "greg-private"); err == nil {
		t.Fatal("greg's channel is not found from bob's workspace")
	}
}

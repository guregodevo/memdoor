package migrations

import (
	"io"
	"log/slog"
	"testing"
)

// THE DEAD PROVIDER NAMES GO, WHEREVER THEY STILL ARE. A row carries one from
// before the rename (`spartacus`) or one from after it (`llamafit`). Everything
// else in the settings table is somebody's real configuration and must survive —
// including a row that merely puts the same word under a different key.
func TestMigration019DropsStaleProviderSettings(t *testing.T) {
	db := freshDB(t)
	for _, row := range []struct{ key, val string }{
		{"agent_provider:chief", "spartacus"},     // the pre-rename name
		{"agent_provider:researcher", "llamafit"}, // the post-rename name
		{"agent_provider:verifier", "anthropic"},  // a live provider: kept
		{"agent_model:chief", "llamafit"},         // a MODEL key, not a provider one
		{"heartbeat_interval", "llamafit"},        // unrelated key, same word
	} {
		if _, err := db.Exec(`INSERT INTO workspace_settings (workspace_id, key, value, updated_at) VALUES ('ws', ?, ?, CURRENT_TIMESTAMP)`,
			row.key, row.val); err != nil {
			t.Fatalf("seed %q: %v", row.key, err)
		}
	}

	r := NewRunner(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := r.Run(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	for _, key := range []string{"agent_provider:chief", "agent_provider:researcher"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM workspace_settings WHERE key = ?`, key).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s still names a provider that no longer exists", key)
		}
	}
	for _, row := range []struct{ key, want string }{
		{"agent_provider:verifier", "anthropic"},
		{"agent_model:chief", "llamafit"},
		{"heartbeat_interval", "llamafit"},
	} {
		var got string
		if err := db.QueryRow(`SELECT value FROM workspace_settings WHERE key = ?`, row.key).Scan(&got); err != nil {
			t.Fatalf("%s was deleted but names no dead provider: %v", row.key, err)
		}
		if got != row.want {
			t.Errorf("%s = %q, want %q", row.key, got, row.want)
		}
	}

	// A database that never had the rows — this one, now — migrates cleanly too.
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = '019_drop_stale_provider_settings'`); err != nil {
		t.Fatal(err)
	}
	if err := r.Run(); err != nil {
		t.Fatalf("a database without the rows must migrate cleanly: %v", err)
	}
}

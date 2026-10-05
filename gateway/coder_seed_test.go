package gateway

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "memdoor/pkg/sqlitedriver"

	"memdoor/gateway/logs"
)

// A coder row seeded in an earlier era keeps nothing of that era: boot
// rewrites its description and personality with the prompt and tools. Live
// 2026-09-27, a row from an earlier era still told every turn it "follows team
// conventions from the handbook". The mutation check: drop personality from the
// re-sync UPDATE and the old text survives.
func TestSeedCoderRewritesAnOldRow(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE buddies (id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, avatar_emoji TEXT, icon TEXT, description TEXT, personality TEXT, system_prompt TEXT, temperature REAL, max_tokens INTEGER, skills TEXT, tools TEXT, sandbox_scope TEXT, learning_enabled INTEGER, admin_only INTEGER, is_active INTEGER, execution_type TEXT, remote_config TEXT, created_by TEXT, created_at TIMESTAMP, updated_at TIMESTAMP, UNIQUE(workspace_id, name))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO buddies (id, workspace_id, name, description, personality) VALUES ('old', 'ws', 'coder', 'grounded in the team handbook', 'follows team conventions from the handbook')`); err != nil {
		t.Fatal(err)
	}
	(&Server{log: logs.New("test")}).seedCoderAgent(context.Background(), db, "ws")

	var desc, pers string
	if err := db.QueryRow(`SELECT description, personality FROM buddies WHERE name = 'coder'`).Scan(&desc, &pers); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(desc+pers), "handbook") {
		t.Fatalf("the old row kept its text: %q / %q", desc, pers)
	}
	if pers != coderPersonality || desc != coderDescription {
		t.Fatalf("got %q / %q", desc, pers)
	}
}

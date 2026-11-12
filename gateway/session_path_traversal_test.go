package gateway

import (
	"memdoor/internal/testenv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A conversation key's parts become directory names: a caller's channel_id of
// "../../<another workspace>/channels/<id>" reached another workspace's
// transcript, and could make directories anywhere, until 2026-10-30.
func TestASessionKeyCannotLeaveItsDirectory(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	sp, err := NewSessionPersistence(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"workspace:mine:channel:../../theirs/channels/c1",
		"workspace:..:channel:c1",
		"workspace:mine:channel:c1:agent:../../../../tmp",
		"workspace:mine:channel:a\\b",
	} {
		if p, err := sp.resolveSessionStorePath(key); err == nil {
			t.Errorf("%q resolved to %s", key, p)
		}
	}
	// A key of no known shape falls back to an agent's file, inside ~/.memdoor.
	if p, err := sp.resolveSessionStorePath("../../etc"); err == nil && !strings.HasPrefix(p, filepath.Join(home, ".memdoor")+string(filepath.Separator)) {
		t.Errorf("../../etc resolved outside ~/.memdoor: %s", p)
	}
	if _, err := os.Stat(filepath.Join(home, ".memdoor", "workspaces", "theirs")); err == nil {
		t.Fatal("a refused key still made a directory")
	}
	p, err := sp.resolveSessionStorePath("workspace:mine:channel:c1")
	if err != nil || !strings.HasPrefix(p, filepath.Join(home, ".memdoor", "workspaces", "mine", "channels", "c1")) {
		t.Fatalf("a plain key: %s %v", p, err)
	}
}

package gateway

import (
	"path/filepath"
	"strings"
	"testing"
)

// Each workflow task's turn has its own transcript file: the key used to fall
// through to the agent's shared file, and every task loaded every task
// before it (live 2026-10-03: 105k input tokens for a one-line task).
func TestEachWorkflowTaskHasItsOwnTranscript(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	sp, err := NewSessionPersistence(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	a, err := sp.resolveSessionStorePath("workflow:peer-comps:2026-10-03T195319:memo")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := sp.resolveSessionStorePath("workflow:peer-comps:2026-10-03T195319:publish")
	if a == b || !strings.Contains(a, filepath.Join(".memdoor", "workflow-sessions")) || strings.Contains(a, filepath.Join("agents", "main")) {
		t.Fatalf("own file per task: %s %s", a, b)
	}
}

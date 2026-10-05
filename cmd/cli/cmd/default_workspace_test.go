package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRecents(t *testing.T, home string, slugs ...string) {
	t.Helper()
	dir := filepath.Join(home, ".memdoor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := ""
	for _, s := range slugs {
		body += s + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, recentWorkspacesFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A QUESTION WITH ONE POSSIBLE ANSWER IS NOT A QUESTION (Greg, 2026-09-27,
// walking onboarding: "there is a default workspace"). Somebody with a single
// workspace was refused in any directory they had not pinned, and handed four
// ways to name the only workspace they have.
func TestTheOnlyWorkspaceIsTheDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envWorkspaceVar, "")
	writeRecents(t, home, "shipyard")
	if slug, ok := soleWorkspace(); !ok || slug != "shipyard" {
		t.Fatalf("the only workspace should be the default: %q ok=%v", slug, ok)
	}
}

// With several, the ambiguity is real. The error must then be worth reading:
// the slugs the person actually has, not a recital of four ways to set one.
func TestSeveralWorkspacesNameThemselves(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envWorkspaceVar, "")
	writeRecents(t, home, "shipyard", "hackernews")
	if _, ok := soleWorkspace(); ok {
		t.Fatal("two workspaces must not resolve to a default")
	}
	msg := noWorkspaceErrorMessage("agent").Error()
	for _, want := range []string{"shipyard", "hackernews", "workspace use"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the error should mention %q: %s", want, msg)
		}
	}
}

// And on a machine where nothing has happened yet, the answer is setup — not a
// lecture about flags, env vars and config keys.
func TestFreshMachineIsToldToSetUp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envWorkspaceVar, "")
	if _, ok := soleWorkspace(); ok {
		t.Fatal("an empty machine has no default workspace")
	}
	msg := noWorkspaceErrorMessage("agent").Error()
	if !strings.Contains(msg, "memdoor setup") {
		t.Errorf("a fresh machine is told to set up: %s", msg)
	}
	if strings.Contains(msg, "config: set workspace_id") {
		t.Errorf("the four-way recital is gone: %s", msg)
	}
}

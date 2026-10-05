package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandSlashSkillResolvesAndCarriesArgs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".memdoor", "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "my-review.md"), []byte("review each file carefully"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, ok := expandSlashSkill("/skill:my-review focus on error handling", "")
	if !ok {
		t.Fatal("known skill must expand")
	}
	for _, want := range []string{"review each file carefully", "## User input\nfocus on error handling", "Follow this workflow now."} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestExpandSlashSkillProjectLocalWins(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	proj := t.TempDir()
	pdir := filepath.Join(proj, ".agents", "skills")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "flow.md"), []byte("project version"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, ok := expandSlashSkill("/skill:flow", proj)
	if !ok || !strings.Contains(out, "project version") {
		t.Fatalf("project-local skill must expand, got ok=%v:\n%s", ok, out)
	}
}

func TestExpandSlashSkillPassthroughs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Set up a real skill so the bare-name case proves the namespace is required.
	dir := filepath.Join(home, ".memdoor", "skills")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "real.md"), []byte("content"), 0o644)
	for _, msg := range []string{
		"/skill:definitely-not-a-skill-xyz", // unknown name under the prefix
		"/real do it",                       // bare name: must NOT expand without skill: prefix
		"/tmp/file.txt is broken",           // path-like
		"fix the / in the URL",              // slash not at start
		"/skill:",                           // empty name
	} {
		if _, ok := expandSlashSkill(msg, ""); ok {
			t.Fatalf("%q must pass through unexpanded", msg)
		}
	}
}

func TestExpandSlashSkillToleratesMentionPrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".memdoor", "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rcpt.md"), []byte("make the receipt"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, ok := expandSlashSkill("@coder /skill:rcpt now please", "")
	if !ok || !strings.Contains(out, "make the receipt") || !strings.Contains(out, "now please") {
		t.Fatalf("mention-prefixed slash must expand with args, ok=%v:\n%s", ok, out)
	}
	if _, ok := expandSlashSkill("@coder fix the bug", ""); ok {
		t.Fatal("mention without slash must pass through")
	}
}

package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Project-local .agents/skills (harness-injected cwd) must win over the
// managed tier and resolve with no rebuild — the disk-first contract.
func TestSkillResolvesProjectLocalDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	proj := t.TempDir()
	skillDir := filepath.Join(proj, ".agents", "skills")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "my-flow.md"), []byte("do the thing"), 0o644); err != nil {
		t.Fatal(err)
	}
	in, _ := json.Marshal(map[string]string{"command": "my-flow", "cwd": proj})
	out, err := Skill(in)
	if err != nil {
		t.Fatalf("Skill: %v", err)
	}
	if !strings.Contains(out, "do the thing") {
		t.Fatalf("project-local skill not resolved, got:\n%s", out)
	}
}

// The managed tier (~/.memdoor/skills, the seed target) resolves without any
// injected cwd — a plain CLI call reaches user-edited skills.
func TestSkillResolvesManagedDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".memdoor", "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "edited.md"), []byte("edited on disk"), 0o644); err != nil {
		t.Fatal(err)
	}
	in, _ := json.Marshal(map[string]string{"command": "edited"})
	out, err := Skill(in)
	if err != nil {
		t.Fatalf("Skill: %v", err)
	}
	if !strings.Contains(out, "edited on disk") {
		t.Fatalf("managed skill not resolved, got:\n%s", out)
	}
}

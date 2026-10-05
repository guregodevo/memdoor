package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectInstructionsAreReadFromTheProjectRoot(t *testing.T) {
	dir := t.TempDir()
	spb := &SystemPromptBuilder{}
	if s := spb.buildProjectInstructionsSection(); s != "" {
		t.Fatal("no project dir, no section")
	}
	spb.SetProjectDir(dir)
	if s := spb.buildProjectInstructionsSection(); s != "" {
		t.Fatal("no file, no section")
	}
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("Run make test before committing."), 0o644)
	if s := spb.buildProjectInstructionsSection(); s != "" {
		t.Fatalf("CLAUDE.md is not read (AGENTS.md replaced it): %q", s)
	}
	os.MkdirAll(filepath.Join(dir, ".memdoor"), 0o755)
	os.WriteFile(filepath.Join(dir, ".memdoor", "AGENTS.md"), []byte("Kept: run the race tests."), 0o644)
	if s := spb.buildProjectInstructionsSection(); !strings.Contains(s, "race tests") {
		t.Fatalf("the rules Memdoor keeps are read: %q", s)
	}
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("Use go test -race."), 0o644)
	if s := spb.buildProjectInstructionsSection(); !strings.Contains(s, "(AGENTS.md)") || !strings.Contains(s, "go test -race") || strings.Contains(s, "make test") {
		t.Fatalf("the project's AGENTS.md first: %q", s)
	}
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(strings.Repeat("rule\n", 10000)), 0o644)
	if s := spb.buildProjectInstructionsSection(); len(s) > ProjectInstructionsMaxChars+400 || !strings.Contains(s, "clipped") {
		t.Fatalf("a long file is clipped: %d chars", len(s))
	}
}

// The coder runs in PromptModeNone (its seed is its prompt); the project's
// instructions still reach it.
func TestProjectInstructionsSurviveNoneMode(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("Run make verify-zebra before committing."), 0o644)
	spb := NewSystemPromptBuilder(nil, nil, dir, "m")
	spb.SetAgentName("coder")
	spb.SetPromptMode(PromptModeNone)
	spb.SetProjectDir(dir)
	out, err := spb.Build()
	if err != nil || !strings.Contains(out, "verify-zebra") {
		t.Fatalf("none mode drops the project's instructions: %v %q", err, out)
	}
}

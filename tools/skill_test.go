package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillLoadsByName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	skills := filepath.Join(home, ".memdoor", "skills")
	os.MkdirAll(skills, 0o755)
	os.WriteFile(filepath.Join(skills, "demo.md"), []byte("# Demo\n\nDo the thing.\n"), 0o644)

	in, _ := json.Marshal(map[string]string{"command": "demo"})
	out, err := Skill(json.RawMessage(in))
	if err != nil {
		t.Fatalf("Skill: %v", err)
	}
	if !strings.Contains(out, "Do the thing") {
		t.Errorf("skill content not returned:\n%s", out)
	}
	// Tolerates a trailing .md and can't traverse out.
	in2, _ := json.Marshal(map[string]string{"command": "demo.md"})
	if _, err := Skill(json.RawMessage(in2)); err != nil {
		t.Errorf("trailing .md should resolve: %v", err)
	}
}

func TestSkillUnknownListsAvailable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".memdoor", "skills")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "alpha.md"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "beta.md"), []byte("b"), 0o644)

	in, _ := json.Marshal(map[string]string{"command": "nope"})
	_, err := Skill(json.RawMessage(in))
	if err == nil || !strings.Contains(err.Error(), "alpha") || !strings.Contains(err.Error(), "beta") {
		t.Errorf("unknown skill should list available (alpha, beta): %v", err)
	}
}

func TestSkillRequiresName(t *testing.T) {
	in, _ := json.Marshal(map[string]string{"command": ""})
	if _, err := Skill(json.RawMessage(in)); err == nil {
		t.Error("empty command should error")
	}
}

// Skills must resolve from ANY working directory: the built-in library is embedded
// in the binary; ./skills on disk is only an override. Simulate a deployed gateway
// by chdir'ing to an empty temp dir and loading a built-in skill.
func TestSkillResolvesFromAnyCwd(t *testing.T) {
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	in, _ := json.Marshal(map[string]string{"command": "review"})
	out, err := Skill(json.RawMessage(in))
	if err != nil {
		t.Fatalf("embedded skill must load from any cwd: %v", err)
	}
	if !strings.Contains(out, "jread") {
		t.Fatalf("review skill content missing:\n%.200s", out)
	}
	// The not-found error must still list the embedded library.
	in2, _ := json.Marshal(map[string]string{"command": "nope-nothing"})
	_, err = Skill(json.RawMessage(in2))
	if err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("available-skills listing should include embedded names, got: %v", err)
	}
}

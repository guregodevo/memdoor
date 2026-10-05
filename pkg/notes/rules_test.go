package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A rule kept for later lands in the project's instructions file, under one
// heading, once — whatever else the file holds.
func TestRuleBookKeepsARuleOnceUnderItsHeading(t *testing.T) {
	book := NewMarkdownRuleBook()
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	if err := book.Add(path, "Run tests with `go test -race ./...`."); err != nil {
		t.Fatal(err)
	}
	if err := book.Add(path, "Never add dependencies."); err != nil {
		t.Fatal(err)
	}
	if err := book.Add(path, "Never add dependencies."); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	want := RulesHeading + "\n\n- Run tests with `go test -race ./...`.\n- Never add dependencies.\n"
	if string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}

	// An existing file keeps its text; the section goes where it is.
	other := filepath.Join(dir, "CLAUDE.md")
	body := "# Project\n\nUse tabs.\n\n" + RulesHeading + "\n\n- Old rule.\n\n## Layout\n\ncmd/ holds the CLI.\n"
	if err := os.WriteFile(other, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := book.Add(other, "New rule."); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(other)
	want = "# Project\n\nUse tabs.\n\n" + RulesHeading + "\n\n- Old rule.\n- New rule.\n\n## Layout\n\ncmd/ holds the CLI.\n"
	if string(got) != want {
		t.Fatalf("the rule belongs in its section:\n%s", got)
	}
	if err := book.Add("", "x"); err == nil || !strings.Contains(err.Error(), "instructions file") {
		t.Fatal("no file, no rule")
	}
}

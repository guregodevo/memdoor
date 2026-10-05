package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A judged read shows its verdict and the places it kept, relative to the
// working directory, and the label shows the pattern — not keep: "10" and the
// last lines of the last hunk (2026-09-27, the landing demo recording).
func TestJudgedReadShowsTheVerdictAndThePlaces(t *testing.T) {
	cwd, _ := os.Getwd()
	grep := "jgrep: kept 2 of 52 candidate hunks in 28 files (ranked by relevance to the task).\n\n" +
		"== " + filepath.Join(cwd, "scripts/deploy.sh") + ":666-682  (p=0.93)\n    666  x\n\n" +
		"== " + filepath.Join(cwd, "scripts/deploy.sh") + ":427-450  (p=0.79)\n    427  y\n"
	v := viewFor("jgrep")
	if got := v.Label(`{"pattern":"install\\.sh","task":"where is it served","keep":10}`, 100); got != `jgrep(install\.sh)` {
		t.Errorf("label must show the pattern, got %q", got)
	}
	body := v.Body(ToolRender{Output: grep, Width: 100})
	for _, want := range []string{"kept 2 of 52 candidate hunks", "scripts/deploy.sh:666-682  p=0.93", "scripts/deploy.sh:427-450  p=0.79"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	if strings.Contains(body, cwd) || strings.Contains(body, "666  x") {
		t.Errorf("paths are relative and hunk text stays behind ctrl+o:\n%s", body)
	}

	read := "kept 1 of 3 sections of " + filepath.Join(cwd, "docs/MUST.md") + " (114 lines):\n\n(p=0.80) " +
		filepath.Join(cwd, "docs/MUST.md") + ":1-38\n    1  # MUST\n"
	body = viewFor("jread").Body(ToolRender{Output: read, Width: 100})
	if !strings.Contains(body, "kept 1 of 3 sections of docs/MUST.md (114 lines)") || !strings.Contains(body, "docs/MUST.md:1-38  p=0.80") {
		t.Errorf("jread summary wrong:\n%s", body)
	}
	if viewFor("jgrep").Body(ToolRender{Output: grep, Expand: true}) != "" {
		t.Error("ctrl+o must fall back to the full result")
	}
}

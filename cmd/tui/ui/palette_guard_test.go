package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The palette is the one place colour is decided (AGENTS.md, ADR-0018), so a
// raw lipgloss.Color("NNN") anywhere else is the drift that put a tool's prose
// in a 2.3:1 grey and a selected row at 1.55:1.
func TestNoColourOutsideThePalette(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	// Both shapes: the call, and a bare 256-colour number handed to a variable
	// that becomes one (`colour := "240"` put a system note at 2.3:1).
	raw := regexp.MustCompile(`lipgloss\.Color\("[0-9]+"\)|col(our|or)\s*:?=\s*"[0-9]+"`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if !raw.MatchString(line) {
				continue
			}
			// The palette block itself is where the numbers live.
			if f == "model_view.go" && strings.Contains(line, "= \"") {
				continue
			}
			t.Errorf("%s:%d names a colour instead of a palette token: %s", f, i+1, strings.TrimSpace(line))
		}
	}
}

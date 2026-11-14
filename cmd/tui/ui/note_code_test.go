package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// A team run's diff in its note is code (the report of 2026-11-14): the diff
// of a markdown file keeps its backticks and bold marks, and a long + line
// wraps under its own text instead of falling back to the left edge.
func TestANoteShowsItsDiffAsWritten(t *testing.T) {
	m := Model{width: 80}
	added := "+In `gateway/route_handler.go`, the new `viewSessionRoute` helper builds the temporary session from a copy of the **metadata**, rather than a blank map."
	note := "✓ team-solve done\n\nThe diff:\n    --- /dev/null\n    " + added + "\n    +\tindented with a tab"
	got := ansi.Strip(m.renderMessage(Message{Role: "system", Content: note}))
	lines := strings.Split(got, "\n")

	if joined := strings.Join(lines, " "); !strings.Contains(joined, "`gateway/route_handler.go`,") || !strings.Contains(joined, "**metadata**,") {
		t.Fatalf("the diff lost its marks:\n%s", got)
	}
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "      +In `gateway") {
			start = i
		}
	}
	if start < 0 || start+1 >= len(lines) {
		t.Fatalf("no wrapped + line:\n%s", got)
	}
	for _, l := range lines[start+1 : start+3] {
		if !strings.HasPrefix(l, "       ") || strings.HasPrefix(l, "        ") || strings.TrimSpace(l) == "" {
			t.Fatalf("a continuation must start under the text, one past the +: %q\n%s", l, got)
		}
	}
	for _, l := range lines {
		if ansi.StringWidth(l) > m.width {
			t.Fatalf("a line runs past the window (%d > %d): %q", ansi.StringWidth(l), m.width, l)
		}
	}
	if !strings.Contains(got, "      +    indented with a tab") {
		t.Fatalf("a tab must be four spaces:\n%s", got)
	}
}

// The prose of a note still gets the markdown pass; only its indented lines
// are code.
func TestANoteKeepsMarkdownOutsideItsCode(t *testing.T) {
	m := Model{width: 80}
	got := ansi.Strip(m.renderMessage(Message{Role: "system", Content: "**Team mode**: on it\n    **kept**"}))
	if strings.Contains(strings.Split(got, "\n")[0], "**") {
		t.Fatalf("the prose must lose its marks: %q", got)
	}
	if !strings.Contains(got, "**kept**") {
		t.Fatalf("the code must keep them: %q", got)
	}
}

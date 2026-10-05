package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Commands and code are shown as they ran: markdown ate the asterisks of
// `--include="*.tsx" --include="*.ts"` (2026-09-28) and of `(m *Model) f(x *T)`.
func TestToolOutputKeepsAsterisks(t *testing.T) {
	m := Model{width: 200}
	for tool, text := range map[string]string{
		"bash":      `grep -rln "relay" web/src --include="*.tsx" --include="*.ts"`,
		"read_file": "func (m *Model) f(x *T) {}",
		"jgrep":     "a.go:12  func (s *routeShadow) exploreDue() (n *int)",
	} {
		if got := ansi.Strip(m.formatToolOutput(tool, text)); got != text {
			t.Errorf("%s: got %q, want %q", tool, got, text)
		}
	}
	if got := ansi.Strip(m.formatToolOutput("todo_write", "**done**")); strings.Contains(got, "**") {
		t.Errorf("prose tools still get markdown: %q", got)
	}
}

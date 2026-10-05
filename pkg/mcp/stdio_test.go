package mcp

import (
	"strings"
	"testing"
)

// A launcher that prints a banner and exits should not fill the add box with
// box-drawing characters: the reason is the last thing it said in words.
func TestStderrTailKeepsWhatWasSaidNotTheFraming(t *testing.T) {
	tr := &stdioTransport{stderr: []string{
		"-----------------------------------------------------",
		"|   Everything Server Launcher                      |",
		"-----------------------------------------------------",
		"",
		"Error: unknown argument --extra",
	}}
	got := tr.stderrTail()
	if !strings.Contains(got, "unknown argument --extra") {
		t.Fatalf("the reason is missing: %q", got)
	}
	if strings.Contains(got, "---") || strings.Contains(got, "|   ") {
		t.Fatalf("framing survived: %q", got)
	}
}

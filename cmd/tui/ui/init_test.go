package ui

import (
	"strings"
	"testing"
)

// /init is a brief for the coder, not a local action: it goes out as the
// turn's text and asks for the project's AGENTS.md.
func TestInitIsABriefForTheCoder(t *testing.T) {
	m := NewModel("ws://localhost:0/ws", "demo", "chan", "", func(string, string, string, string) error { return nil })
	text, local := m.handleSlashCommand("/init")
	if local || !strings.Contains(text, "AGENTS.md") || !strings.Contains(text, "under 80 lines") {
		t.Fatalf("local=%v text=%q", local, text)
	}
	if !strings.Contains(strings.Join(loadSlashCommands("coder"), " "), "/init") {
		t.Fatal("/init is offered to the coder")
	}
}

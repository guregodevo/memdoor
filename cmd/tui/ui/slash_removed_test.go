package ui

import (
	"strings"
	"testing"
)

// /doctor, /new, /go (/run, /approve) are gone (2026-11-06): /doctor said
// what the footer and /context say, /new told the person to exit and start
// again when /fresh does it in place, and /go took the last local answer
// (the /doctor text) as a plan and sent it to the coder.
func TestRemovedSlashCommandsAreUnknown(t *testing.T) {
	help := slashHelpText()
	for _, gone := range []string{"/doctor", "/new ", "/new\n", "/go ", "/go\n", "/run ", "/approve"} {
		if strings.Contains(help, gone) {
			t.Errorf("/help still lists %q", strings.TrimSpace(gone))
		}
	}
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	for _, cmd := range []string{"/doctor", "/new", "/go"} {
		n := len(m.messages)
		m.handleSlashCommand(cmd)
		if len(m.messages) == n || !strings.Contains(m.messages[len(m.messages)-1].Content, "Unknown slash command") {
			t.Errorf("%s is not unknown", cmd)
		}
	}
}

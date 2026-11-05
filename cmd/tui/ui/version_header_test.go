package ui

import (
	"strings"
	"testing"
)

// The header names the running version next to live, so a terminal under 150
// columns still shows it (#90: the footer's last place was cut at 120 and
// gone at 100). Mutation check: render the header without m.status.Version
// and this fails.
func TestTheHeaderNamesTheVersion(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", func(a, w, tx, mode string) error { return nil })
	m.connected, m.width = true, 80
	m.status.Version = "v1.794.0"
	head := m.renderHeader()
	if !strings.Contains(head, "live v1.794.0") {
		t.Fatalf("header %q", head)
	}
	if strings.Contains(m.status.line(), "v1.794.0") {
		t.Fatal("the version is in the header, not again in the footer")
	}
	m.status.Version = ""
	if head := m.renderHeader(); !strings.Contains(head, "live · ") {
		t.Fatalf("without a version the header reads as before: %q", head)
	}
}

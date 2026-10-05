package ui

import (
	"strings"
	"testing"
)

// /context says what is eating the window and how far compaction is.
func TestContextShowsWhatFillsTheWindow(t *testing.T) {
	m, _, _ := freshModel(t, nil)
	nm, _ := m.Update(contextUpdateMsg{tokens: 30_000, limit: 114_688, percent: 26.2,
		parts: contextParts{system: 2_500, tools: 4_000, conversation: 23_500, toolOutput: 18_000, compactAt: 68_812,
			compactRule: "60% of the window, at most 200K", keepRecent: 12_000}})
	*m = nm.(Model)
	out := m.getContextStatus()
	for _, want := range []string{"System prompt", "Tool definitions", "of which tool output", "Compacts at", "(60% of the window, at most 200K)", "to go", "/compact", "keeping the last 12.0k", "thresholdTokens, keepRecentTokens"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	for _, never := range []string{"AI compaction", "Keeps ~20", "90%"} {
		if strings.Contains(out, never) {
			t.Fatalf("stale text %q in:\n%s", never, out)
		}
	}
}

// Before the gateway's first context event there is no window to quote. The
// old default said "0 / 200.0k" — the 200K the compaction ceiling happens to
// be, displayed as if it were the model's window.
func TestContextBeforeAnyReportSaysSo(t *testing.T) {
	m, _, _ := freshModel(t, nil)
	if m.contextLimit != 0 || m.contextTokens != 0 {
		t.Fatalf("a fresh model must carry no context numbers: %d/%d",
			m.contextTokens, m.contextLimit)
	}
	out := m.getContextStatus()
	for _, want := range []string{"No context report yet", "Run a turn"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "200") || strings.Contains(out, "0.0%") {
		t.Fatalf("invented a window before any event:\n%s", out)
	}
	// /doctor's context line follows the same rule.
	doc := m.getDoctorStatus()
	if !strings.Contains(doc, "no report yet") || strings.Contains(doc, "200") {
		t.Fatalf("doctor invented a window before any event:\n%s", doc)
	}
}

// A window the gateway reported is never replaced by the placeholder.
func TestContextShowsTheReportedWindow(t *testing.T) {
	m, _, _ := freshModel(t, nil)
	nm, _ := m.Update(contextUpdateMsg{tokens: 12_500, limit: 114_688, percent: 10.9})
	*m = nm.(Model)
	out := m.getContextStatus()
	if !strings.Contains(out, "12.5k") || !strings.Contains(out, "114.7k") {
		t.Fatalf("missing the reported numbers in:\n%s", out)
	}
}

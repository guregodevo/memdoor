package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/logs"
)

// setupGlobalLogs ensures logs.GetGlobalStorage() returns a real sqlite
// storage rooted under a tempdir. Tests in this file emit through the
// global EventLogger and inspect what landed via QueryEvents.
func setupGlobalLogs(t *testing.T) logs.Storage {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := logs.InitGlobalLogger(filepath.Join(dir, "logs"), false); err != nil {
		t.Fatalf("InitGlobalLogger: %v", err)
	}
	return logs.GetGlobalStorage()
}

func callReportBug(t *testing.T, in ReportBugInput) string {
	t.Helper()
	body, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ReportBugDefinition.FunctionWithContext(json.RawMessage(body), nil)
	if err != nil {
		t.Fatalf("FunctionWithContext: %v", err)
	}
	// Drain the buffered event to storage now, so the assertions don't
	// race the 1s background flush loop (the CI flake source).
	logs.FlushGlobal()
	return out
}

func TestReportBug_WritesWarnEventForMediumSeverity(t *testing.T) {
	store := setupGlobalLogs(t)
	out := callReportBug(t, ReportBugInput{
		Title:           "jgrep returned empty for a query that should match",
		Severity:        "medium",
		WhatITried:      "called jgrep('foo'); 0 results; called grep('foo'); 1 hit",
		WhyItLooksWrong: "the same text contains 'foo' — grep and jgrep should agree",
	})
	if !strings.Contains(out, `"received":true`) {
		t.Errorf("expected received:true in result, got %s", out)
	}

	// Wait briefly for the EventLogger's flush loop (1s interval) to
	// drain the event into sqlite. Poll up to 5s.
	events := waitForEvents(t, store, 5)
	if len(events) == 0 {
		t.Fatal("expected at least one event written, got 0")
	}
	got := events[0]
	if got.Level != logs.LevelWarn {
		t.Errorf("medium severity should map to WARN, got %s", got.Level)
	}
	if !strings.Contains(got.Message, "jgrep") {
		t.Errorf("title not in event message: %s", got.Message)
	}
	if got.Component != "agent-bug-report" {
		t.Errorf("expected catch-all component, got %s", got.Component)
	}
}

func TestReportBug_HighSeverityMapsToError(t *testing.T) {
	store := setupGlobalLogs(t)
	callReportBug(t, ReportBugInput{
		Title:           "logged writes silently dropped 50% of events",
		Severity:        "high",
		WhatITried:      "wrote 100 events; logs stats counts 50",
		WhyItLooksWrong: "the write said ok for every event; half are missing",
		Component:       "logs",
	})
	events := waitForEvents(t, store, 5)
	if len(events) == 0 {
		t.Fatal("expected event")
	}
	if events[0].Level != logs.LevelError {
		t.Errorf("high severity should map to ERROR, got %s", events[0].Level)
	}
	if events[0].Component != "logs" {
		t.Errorf("explicit component should win, got %s", events[0].Component)
	}
}

func TestReportBug_RejectsMissingTitle(t *testing.T) {
	setupGlobalLogs(t)
	body, _ := json.Marshal(ReportBugInput{
		Severity:        "low",
		WhatITried:      "x",
		WhyItLooksWrong: "y",
	})
	_, err := ReportBugDefinition.FunctionWithContext(json.RawMessage(body), nil)
	if err == nil {
		t.Error("expected error for missing title")
	}
}

func TestReportBug_RejectsMissingWhatITried(t *testing.T) {
	setupGlobalLogs(t)
	body, _ := json.Marshal(ReportBugInput{
		Title:           "x",
		Severity:        "low",
		WhyItLooksWrong: "y",
	})
	_, err := ReportBugDefinition.FunctionWithContext(json.RawMessage(body), nil)
	if err == nil {
		t.Error("expected error for missing what_i_tried")
	}
}

func TestReportBug_EvidenceLandsInEventData(t *testing.T) {
	store := setupGlobalLogs(t)
	evidence := map[string]any{
		"logs_excerpt": "grep: 0 matches for parseConfig\nconfig.go:12 func parseConfig(",
		"expected":     "one match in config.go",
		"observed":     "no matches",
	}
	callReportBug(t, ReportBugInput{
		Title:           "grep returned 0 matches for a symbol the file plainly defines",
		Severity:        "medium",
		WhatITried:      "grep parseConfig in the project; read_file config.go shows it at line 12",
		WhyItLooksWrong: "the definition is on disk, so the search should find it",
		Component:       "grep tool",
		Evidence:        evidence,
	})
	events := waitForEvents(t, store, 5)
	if len(events) == 0 {
		t.Fatal("expected event")
	}
	got := events[0]
	if got.Data["bug_evidence"] == nil {
		t.Errorf("evidence should land in event data; got: %+v", got.Data)
	}
}

func TestReportBug_OversizedEvidenceRejected(t *testing.T) {
	setupGlobalLogs(t)
	// Build evidence whose JSON serialization exceeds the 64 KB cap.
	big := strings.Repeat("x", 80*1024)
	body, _ := json.Marshal(ReportBugInput{
		Title:           "x",
		Severity:        "low",
		WhatITried:      "x",
		WhyItLooksWrong: "y",
		Evidence:        map[string]any{"logs_excerpt": big},
	})
	_, err := ReportBugDefinition.FunctionWithContext(json.RawMessage(body), nil)
	if err == nil {
		t.Fatal("expected error for oversized evidence")
	}
	if !strings.Contains(err.Error(), "evidence too large") {
		t.Errorf("expected size-cap error, got %v", err)
	}
}

func TestReportBug_UnknownSeverityDefaultsToWarn(t *testing.T) {
	store := setupGlobalLogs(t)
	out := callReportBug(t, ReportBugInput{
		Title:           "weird thing happened",
		Severity:        "yikes",
		WhatITried:      "x",
		WhyItLooksWrong: "y",
	})
	if !strings.Contains(out, `"severity":"unspecified"`) {
		t.Errorf("unknown severity should normalize to 'unspecified' in result, got %s", out)
	}
	events := waitForEvents(t, store, 5)
	if len(events) == 0 {
		t.Fatal("expected event")
	}
	if events[0].Level != logs.LevelWarn {
		t.Errorf("unknown severity should fall back to WARN, got %s", events[0].Level)
	}
}

// waitForEvents polls the storage until QueryEvents returns at least
// one event. Bounds the EventLogger's 1-second flush interval so
// tests don't flake.
func waitForEvents(t *testing.T, store logs.Storage, maxSeconds int) []*logs.Event {
	t.Helper()
	for i := 0; i < maxSeconds*10; i++ {
		q := logs.NewQueryBuilder().MessageRegex(".*").Limit(20).Build()
		res, err := store.QueryEvents(context.Background(), q)
		if err == nil && len(res.Events) > 0 {
			return res.Events
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

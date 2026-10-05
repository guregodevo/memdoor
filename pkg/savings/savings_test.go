package savings

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMonthSumsWhatWasMeasured(t *testing.T) {
	SetLedger(filepath.Join(t.TempDir(), "savings.jsonl"))
	t.Cleanup(func() { SetLedger("") })

	now := time.Now()
	lastMonth := now.AddDate(0, -1, 0)
	// This month.
	Record(Entry{At: now, Kind: KindJudgedRead, Tool: "jgrep", RawBytes: 40_000, KeptBytes: 4_000})
	Record(Entry{At: now, Kind: KindJudgedRead, Tool: "read_file", RawBytes: 60_000, KeptBytes: 20_000})
	Record(Entry{At: now, Kind: KindToolbox, Calls: 1, RawBytes: 20_000, KeptBytes: 14_000, Model: "z-ai/glm-5.3-flash"})
	Record(Entry{At: now, Kind: KindEarlyStop})
	// A month the summary must not touch.
	Record(Entry{At: lastMonth, Kind: KindJudgedRead, Tool: "jgrep", RawBytes: 1_000_000, KeptBytes: 0})
	// Nothing saved, nothing recorded: a judged read that kept everything must
	// not pad the receipt.
	Record(Entry{At: now, Kind: KindJudgedRead, Tool: "jread", RawBytes: 5_000, KeptBytes: 5_000})

	s, err := ReadMonth(now)
	if err != nil {
		t.Fatal(err)
	}
	if s.JudgedCalls != 2 {
		t.Errorf("two judged reads saved something, got %d", s.JudgedCalls)
	}
	if s.JudgedRaw != 100_000 || s.JudgedKept != 24_000 {
		t.Errorf("raw/kept = %d/%d, want 100000/24000", s.JudgedRaw, s.JudgedKept)
	}
	if s.ToolboxSaved != 6_000 || s.ToolboxCalls != 1 {
		t.Errorf("toolbox = %d over %d calls, want 6000 over 1", s.ToolboxSaved, s.ToolboxCalls)
	}
	if s.EarlyStops != 1 {
		t.Errorf("early stops = %d, want 1", s.EarlyStops)
	}
	if s.ByTool["jgrep"] != 36_000 || s.ByTool["read_file"] != 40_000 {
		t.Errorf("per-tool: %v", s.ByTool)
	}
	if s.ModelsSeen["z-ai/glm-5.3-flash"] != 1 {
		t.Errorf("the model that would have read it: %v", s.ModelsSeen)
	}
	// (100,000 − 24,000 + 6,000) / 4
	if got := s.TokensSaved(); got != 20_500 {
		t.Errorf("tokens saved = %d, want 20500", got)
	}
	// At four cents a million input tokens, a floor.
	if got := s.DollarsSaved(0.04); got < 0.00081 || got > 0.00083 {
		t.Errorf("dollars at $0.04/M = %f, want ≈0.00082", got)
	}

	// An empty month is not an error: the ledger simply has not started.
	SetLedger(filepath.Join(t.TempDir(), "none.jsonl"))
	if s, err := ReadMonth(now); err != nil || s.TokensSaved() != 0 {
		t.Errorf("an absent ledger must read as nothing saved: %v %v", s, err)
	}
}

func TestSavedNeverGoesNegative(t *testing.T) {
	if got := (Entry{RawBytes: 10, KeptBytes: 40}).Saved(); got != 0 {
		t.Errorf("a judged read that grew the text saves nothing, got %d", got)
	}
	if got := (Entry{RawBytes: 100, KeptBytes: 40}).Saved(); got != 60 {
		t.Errorf("saved = %d, want 60", got)
	}
}

package gateway

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The report has to survive the process, or it cannot warn anyone later.
func TestModelChecksRoundTripAndKeepTheNewest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	saveModelCheck(modelCheck{ID: "z-ai/glm-5.3", At: time.Now().Add(-time.Hour), ToolCall: true, Note: "old"})
	saveModelCheck(modelCheck{ID: "x-ai/grok-4.7", At: time.Now(), ToolCall: true, RightTool: true, ValidArgs: true, RoundTrip: true, Patch: true})
	saveModelCheck(modelCheck{ID: "z-ai/glm-5.3", At: time.Now(), ToolCall: true, RightTool: true, ValidArgs: true, RoundTrip: true, Note: ""})

	all := loadModelChecks()
	if len(all) != 2 {
		t.Fatalf("one report per model, got %d", len(all))
	}
	if all["z-ai/glm-5.3"].Note != "" || !all["z-ai/glm-5.3"].Runs() {
		t.Errorf("the newest report must win: %+v", all["z-ai/glm-5.3"])
	}
	if _, err := os.Stat(filepath.Join(home, ".memdoor", "model-checks.json")); err != nil {
		t.Errorf("the store must be a file someone can read: %v", err)
	}

	// An absent store is not an error: nothing has been probed yet.
	t.Setenv("HOME", t.TempDir())
	if got := loadModelChecks(); len(got) != 0 {
		t.Errorf("a fresh machine has probed nothing, got %d", len(got))
	}
}

// RE-PROBING ON A SCHEDULE IS ASKING FOR WHAT AGED OUT, and nothing else. The
// probe spends the person's own key, so a schedule is only honest if it costs
// nothing on the weeks when every report is still good; that makes the selection
// the whole feature.
func TestStaleChecksAreTheOnesWorthSpendingOn(t *testing.T) {
	now := time.Now()
	all := map[string]modelCheck{
		"fresh/today":      {ID: "fresh/today", At: now.Add(-2 * time.Hour)},
		"fresh/six-days":   {ID: "fresh/six-days", At: now.Add(-6 * 24 * time.Hour)},
		"stale/eight-days": {ID: "stale/eight-days", At: now.Add(-8 * 24 * time.Hour)},
		"stale/a-month":    {ID: "stale/a-month", At: now.Add(-30 * 24 * time.Hour)},
		"stale/undated":    {ID: "stale/undated"}, // written before reports carried a date
	}
	got := staleChecks(all, now)
	// Oldest first: a probe interrupted halfway has refreshed the reports that
	// were worth the least.
	want := []string{"stale/undated", "stale/a-month", "stale/eight-days"}
	if len(got) != len(want) {
		t.Fatalf("stale = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stale = %v, want %v", got, want)
		}
	}
	// A week of fresh reports must cost nothing at all.
	fresh := map[string]modelCheck{"a/b": {ID: "a/b", At: now.Add(-time.Minute)}}
	if ids := staleChecks(fresh, now); len(ids) != 0 {
		t.Errorf("nothing has aged out, yet it would probe %v", ids)
	}
	if ids := staleChecks(map[string]modelCheck{}, now); len(ids) != 0 {
		t.Errorf("nothing probed yet means nothing to re-probe, got %v", ids)
	}
	// The boundary is the promise the flag makes: a week.
	edge := map[string]modelCheck{"a/b": {ID: "a/b", At: now.Add(-staleAfter - time.Second)}}
	if ids := staleChecks(edge, now); len(ids) != 1 {
		t.Errorf("a report older than a week is stale, got %v", ids)
	}
}

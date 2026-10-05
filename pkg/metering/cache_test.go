package metering

import (
	"encoding/json"
	"testing"
)

func TestEntryCacheHitRate(t *testing.T) {
	e := Entry{InputTokens: 68000, CachedTokens: 63000}
	if got := e.CacheHitRate(); got < 0.925 || got > 0.927 {
		t.Errorf("CacheHitRate = %.4f, want ~0.926", got)
	}
	// A turn with no prompt must not divide by zero.
	if got := (Entry{}).CacheHitRate(); got != 0 {
		t.Errorf("empty entry = %v, want 0", got)
	}
}

// The ledger is append-only and already holds records written before this
// field existed. They must keep decoding, and must not claim a 0% hit rate —
// absent has to stay distinguishable from measured-zero.
func TestOldEntriesWithoutCachedTokensStillDecode(t *testing.T) {
	var e Entry
	old := `{"ts":"2026-08-16T10:00:00Z","engine":"openrouter.ai","input_tokens":500,"output_tokens":20,"duration_ms":900}`
	if err := json.Unmarshal([]byte(old), &e); err != nil {
		t.Fatalf("a pre-existing meter record must still decode: %v", err)
	}
	if e.InputTokens != 500 {
		t.Errorf("InputTokens = %d", e.InputTokens)
	}
	if e.CachedTokens != 0 {
		t.Errorf("an absent field is 0, got %d", e.CachedTokens)
	}
	// omitempty: a turn with no cache figure must not write a misleading 0.
	b, err := json.Marshal(Entry{InputTokens: 500})
	if err != nil {
		t.Fatal(err)
	}
	if contains(string(b), "cached_tokens") {
		t.Errorf("unreported cache must be omitted, not written as 0: %s", b)
	}
	b, _ = json.Marshal(Entry{InputTokens: 500, CachedTokens: 100})
	if !contains(string(b), `"cached_tokens":100`) {
		t.Errorf("a measured figure must be persisted: %s", b)
	}
}

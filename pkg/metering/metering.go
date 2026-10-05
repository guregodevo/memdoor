package metering

import (
	"bytes"
	"encoding/json"
	"memdoor/pkg/shared"
	"os"
	"path/filepath"
	"time"
)

// The append-only usage ledger: one entry per model request (tokens, cost
// in USD at the provider's list price), read by `memdoor meter` and /usage.

// Entry is one metered inference turn on a remote engine.
type Entry struct {
	TS          time.Time `json:"ts"`
	Engine      string    `json:"engine"`
	Model       string    `json:"model"`
	InputTokens int64     `json:"input_tokens"`
	// CachedTokens is how many of InputTokens the engine served from its
	// prefix cache. Zero means "not reported" as well as "nothing cached" —
	// engines that do not publish the detail block are indistinguishable from
	// a total miss, so read a rate only over entries where it is non-zero.
	CachedTokens int64 `json:"cached_tokens,omitempty"`
	OutputTokens int64 `json:"output_tokens"`
	DurationMS   int64 `json:"duration_ms"`
	// User is who asked (the authenticated actor, "human:<uuid>") and Agent
	// who answered — the two keys the flat plan's cost line is kept by.
	// Empty on entries written before 2026-09-04 and on background turns.
	// GenID is the provider's own id for the call ("gen-…"). It is what makes a
	// line here checkable against an OpenRouter invoice or generation record —
	// and it is how the app attribution was verified end to end (2026-09-27).
	GenID string `json:"gen_id,omitempty"`
	User  string `json:"user,omitempty"`
	Agent string `json:"agent,omitempty"`
	// Session is the turn's session id, so cost can be summed per session.
	Session string `json:"session,omitempty"`
	// CostUSD is what the vendor said the call cost, when it says (OpenRouter
	// returns it in usage.cost); 0 means not reported.
	CostUSD float64 `json:"cost_usd,omitempty"`
}

// CacheHitRate is the fraction of an entry's prompt served from cache.
// Returns 0 for an entry with no prompt tokens rather than dividing by zero.
func (e Entry) CacheHitRate() float64 {
	if e.InputTokens <= 0 {
		return 0
	}
	return float64(e.CachedTokens) / float64(e.InputTokens)
}

// Repository is the append-only storage port for usage entries — the DDD
// repository. Factory returns the interface (rule 4); the medium (JSONL
// today, sqlite later) is an implementation detail.
type Repository interface {
	Append(Entry) error
	All() ([]Entry, error)
}

// UserUsage is one user's metered consumption over a period: the cost line
// of the flat plan.
type UserUsage struct {
	User         string
	Turns        int
	Sessions     int     // runs of turns closer than SessionGap
	BrainMS      int64   // remote-brain time while this user's turns ran
	OutputTokens int64   // thinking and answers; finds runaways
	EstCost      float64 // sum of each entry's reported cost
}

// SessionGap is the idle time after which the next turn counts as a new
// session.
const SessionGap = 30 * time.Minute

// UsageByUser aggregates the remote entries since a time, per user.
func UsageByUser(entries []Entry, since time.Time) map[string]*UserUsage {
	per := map[string]*UserUsage{}
	last := map[string]time.Time{}
	for _, e := range entries {
		if e.TS.Before(since) {
			continue
		}
		user := e.User
		if user == "" {
			user = "(unattributed)"
		}
		u := per[user]
		if u == nil {
			u = &UserUsage{User: user}
			per[user] = u
		}
		u.Turns++
		u.BrainMS += e.DurationMS
		u.OutputTokens += e.OutputTokens
		if prev, ok := last[user]; !ok || e.TS.Sub(prev) > SessionGap {
			u.Sessions++
		}
		last[user] = e.TS
		u.EstCost += e.CostUSD
	}
	return per
}

// NewJSONLLedger opens the file-backed repository (one JSON object per
// line, 0600, never rewritten). Path "" resolves to ~/.memdoor/meter.jsonl.
func NewJSONLLedger(path string) (Repository, error) {
	if path == "" {
		path = shared.MemdoorHome("meter.jsonl")
	}
	return &jsonlLedger{path: path}, nil
}

type jsonlLedger struct{ path string }

func (l *jsonlLedger) Append(e Entry) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func (l *jsonlLedger) All() ([]Entry, error) {
	b, err := os.ReadFile(l.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	dec := json.NewDecoder(bytes.NewReader(b))
	for dec.More() {
		var e Entry
		if dec.Decode(&e) != nil {
			break // a torn trailing line must not hide history
		}
		out = append(out, e)
	}
	return out, nil
}

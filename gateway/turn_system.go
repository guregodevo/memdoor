package gateway

import (
	"context"
	"sync"
)

// turnSystem is a turn's system prompt: built on the turn's first call and
// sent as it is on every later one. A prompt rebuilt per call changed under
// the turn (the hour in the date section, an AGENTS.md the coder edited),
// and an engine with a prefix cache then read the whole conversation again:
// a hybrid model (Qwen3.5, Qwen3.8) can only go back to marks near the end
// of the last prompt, so a change anywhere in the system prompt costs all of it.
type turnSystem struct {
	mu    sync.Mutex
	built string
	ok    bool
	sent  string
}

type ctxKeyTurnSystem struct{}

// withTurnSystem gives the turn its own system prompt.
func withTurnSystem(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyTurnSystem{}, &turnSystem{})
}

func turnSystemOf(ctx context.Context) *turnSystem {
	ts, _ := ctx.Value(ctxKeyTurnSystem{}).(*turnSystem)
	return ts
}

// builtOnce is the builder's prompt the turn built first; build runs on the
// turn's first call only. A failed build is not kept: the next call tries again.
func (ts *turnSystem) builtOnce(build func() (string, error)) (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.ok {
		return ts.built, nil
	}
	s, err := build()
	if err != nil {
		return "", err
	}
	ts.built, ts.ok = s, true
	return s, nil
}

// changed records the prompt a call sends and reports whether it differs
// from the one the turn's previous call sent.
func (ts *turnSystem) changed(prompt string) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	was := ts.sent
	ts.sent = prompt
	return was != "" && was != prompt
}

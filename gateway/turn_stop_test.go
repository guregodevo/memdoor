package gateway

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"memdoor/pkg/decision"
)

type fakeStopSvc struct {
	p        float64
	unav     bool
	state    string
	question string
}

func (f *fakeStopSvc) Evaluate(_ context.Context, req decision.Request, _ decision.Options) decision.Result {
	f.state = req.State
	if f.unav {
		return decision.Unavailable(decision.ReasonNotConfigured, "test")
	}
	answers := map[string]decision.Answer{}
	for id, q := range req.Questions {
		f.question = q.Instructions
		answers[id] = decision.Answer{Kind: decision.KindBoolean, ProbabilityTrue: f.p}
	}
	return decision.Result{Status: decision.StatusOK, Answers: answers}
}

// The shadow score fires once; the decision model reads the last steps in
// words and decides; off or unavailable never stops a run.
func TestTurnStopIsTheDecisionModelsCall(t *testing.T) {
	s := shadowFor(t)
	fail := ToolExecutionInfo{Output: "--- FAIL: TestX", Error: "exit status 1"}
	for i := 0; i < 5; i++ {
		s.observe("bash", `{"command":"go test ./..."}`, fail)
	}
	if !s.justFired() || s.justFired() {
		t.Fatal("the firing is handed out exactly once")
	}
	window := s.windowText()
	if !strings.Contains(window, "bash {\"command\":\"go test ./...\"} → error: exit status 1") || !strings.HasPrefix(window, "1. ") {
		t.Fatalf("the window is the steps in words: %q", window)
	}
	svc := &fakeStopSvc{p: 0.91}
	v := &turnVerdict{svc: svc, read: func(string) string { return "" }}
	p, stop := v.stuck(context.Background(), "coder", "fix the test", window)
	if !stop || p != 0.91 || !strings.Contains(svc.state, "Request: fix the test") || !strings.Contains(svc.state, "go test") {
		t.Fatalf("stop=%v p=%v state=%q", stop, p, svc.state)
	}
	if _, stop := (&turnVerdict{svc: &fakeStopSvc{p: 0.3}, read: func(string) string { return "" }}).stuck(context.Background(), "coder", "r", window); stop {
		t.Fatal("below the threshold the run goes on")
	}
	if _, stop := (&turnVerdict{svc: &fakeStopSvc{unav: true}, read: func(string) string { return "" }}).stuck(context.Background(), "coder", "r", window); stop {
		t.Fatal("unavailable never stops a run")
	}
	if _, stop := (&turnVerdict{svc: svc, read: func(k string) string { return "off" }}).stuck(context.Background(), "coder", "r", window); stop {
		t.Fatal("off is off")
	}
	var none *turnVerdict
	if _, stop := none.stuck(context.Background(), "coder", "r", window); stop {
		t.Fatal("no decision model, no stop")
	}
}

// A turn that only reads never trips the score (live 2026-09-28: 129 calls,
// 0 edits, score 0). Every 30 reads with no edit the decision model is asked
// whether it should stop exploring; an edit ends the question for the turn.
func TestExploringWithoutAnEditIsAsked(t *testing.T) {
	s := shadowFor(t)
	read := func(i int) {
		s.observe("bash", fmt.Sprintf(`{"command":"sed -n %d,%dp a.go"}`, i, i+40), ToolExecutionInfo{Output: "x"})
	}
	due := 0
	for i := 0; i < 95; i++ {
		read(i)
		if n, edited, ok := s.exploreDue(); ok {
			if edited {
				t.Fatal("no edit was made")
			}
			due++
			if n != 30*due {
				t.Fatalf("asked at %d reads, want %d", n, 30*due)
			}
		}
	}
	if due != 3 || s.fired {
		t.Fatalf("asked %d times (want 3 at 30/60/90); the score fired=%v", due, s.fired)
	}
	svc := &fakeStopSvc{p: 0.85}
	v := &turnVerdict{svc: svc, read: func(string) string { return "" }}
	if p, stop := v.exploring(context.Background(), "coder", "continue the work in this PR", s.windowText(), 90, false); !stop || p != 0.85 ||
		!strings.Contains(svc.state, "90 reads, 0 edits") || !strings.Contains(svc.state, "sed -n") {
		t.Fatalf("stop=%v p=%v state=%q", stop, p, svc.state)
	}
	if _, stop := (&turnVerdict{svc: &fakeStopSvc{p: 0.45}, read: func(string) string { return "" }}).exploring(context.Background(), "coder", "where is X?", s.windowText(), 30, false); stop {
		t.Fatal("a question turn reading at 0.45 goes on")
	}
	if _, stop := (&turnVerdict{svc: svc, read: func(string) string { return "off" }}).exploring(context.Background(), "coder", "r", s.windowText(), 30, false); stop {
		t.Fatal("decision_turn_stop off is off")
	}
}

// Reads count from the last edit: a turn that edited once and then read on
// for 30 steps is asked too (live 2026-09-28: edited tunnel.go, then ~60
// reads of one goroutine dump, and no check fired) — with the question for
// a turn that already changed files, "is it going in circles".
func TestReadingSinceTheLastEditIsAsked(t *testing.T) {
	s := shadowFor(t)
	read := func(i int) {
		s.observe("bash", fmt.Sprintf(`{"command":"sed -n %d,%dp /tmp/t2.log"}`, i, i+20), ToolExecutionInfo{Output: "x"})
	}
	for i := 0; i < 20; i++ {
		read(i)
	}
	s.observe("apply_patch", "patch", ToolExecutionInfo{Output: "ok"})
	for i := 0; i < 29; i++ {
		read(100 + i)
		if _, _, ok := s.exploreDue(); ok {
			t.Fatalf("asked at %d reads since the edit; the 20 before it must not count", i+1)
		}
	}
	read(200)
	n, edited, ok := s.exploreDue()
	if !ok || n != 30 || !edited {
		t.Fatalf("30 reads since the edit must be asked: n=%d edited=%v ok=%v", n, edited, ok)
	}
	s.observe("apply_patch", "patch 2", ToolExecutionInfo{Output: "ok"})
	for i := 0; i < 29; i++ {
		read(300 + i)
		if _, _, ok := s.exploreDue(); ok {
			t.Fatal("a new edit starts the count again")
		}
	}

	svc := &fakeStopSvc{p: 0.91}
	v := &turnVerdict{svc: svc, read: func(string) string { return "" }}
	if p, stop := v.exploring(context.Background(), "coder", "continue the plan", s.windowText(), 48, true); !stop || p != 0.91 {
		t.Fatalf("stop=%v p=%v", stop, p)
	}
	if !strings.Contains(svc.state, "it changed files, then made 48 reads") {
		t.Fatalf("the state must say the turn changed files: %q", svc.state)
	}
	if !strings.Contains(svc.question, "going in circles") {
		t.Fatalf("after an edit the circles question is asked, got %q", svc.question)
	}
}

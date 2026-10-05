package gateway

import (
	"context"
	"testing"

	"memdoor/pkg/decision"
)

type effortSvc struct {
	choice    string
	available bool
	asked     int
}

func (s *effortSvc) Evaluate(_ context.Context, req decision.Request, _ decision.Options) decision.Result {
	s.asked++
	return decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{"effort": {Choice: s.choice}}}
}
func (s *effortSvc) Available(string) bool { return s.available }

// The person's Shift+Tab choice wins; on auto the decision model picks; with
// none (or no answer) the turn gets high, never the host's default.
func TestTurnEffortPrecedence(t *testing.T) {
	sess := &Session{ID: "s", Metadata: map[string]interface{}{}}
	svc := &effortSvc{choice: "low", available: true}
	ar := &AgentRuntime{effortJudge: &effortJudge{svc: svc}}
	if e, from := ar.turnEffort(context.Background(), sess, "coder", "rename Foo to Bar"); e != "low" || from != "decision model" {
		t.Fatalf("auto with a decision model: %s from %s", e, from)
	}
	if err := setEffort(sess, "high"); err != nil {
		t.Fatal(err)
	}
	svc.asked = 0
	if e, from := ar.turnEffort(context.Background(), sess, "coder", "x"); e != "high" || from != "chosen" || svc.asked != 0 {
		t.Fatalf("a chosen effort must win without asking: %s from %s, asked %d", e, from, svc.asked)
	}
	_ = setEffort(sess, "auto")
	svc.available = false
	if e, from := ar.turnEffort(context.Background(), sess, "coder", "x"); e != "high" || from != "default" {
		t.Fatalf("no decision model: %s from %s", e, from)
	}
	if err := setEffort(sess, "max"); err == nil {
		t.Fatal("an unknown effort must be refused")
	}
}

// The route says what the last turn ran at and why, for the footer.
func TestTheRouteCarriesTheEffortInUse(t *testing.T) {
	sess := &Session{ID: "s", Metadata: map[string]interface{}{}}
	sess.SetMetadata(sessionEffortUsed, "medium")
	sess.SetMetadata(sessionEffortFrom, "decision model")
	if v := viewRoute(nil, sess, "coder"); v.EffortUsed != "medium" || v.EffortFrom != "decision model" || v.Effort != "" {
		t.Fatalf("route %+v", v)
	}
}

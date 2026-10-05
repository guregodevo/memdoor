package gateway

import (
	"context"
	"testing"

	"memdoor/pkg/decision"
	"memdoor/pkg/message"
	"memdoor/pkg/shared"
)

type gateSvc struct {
	p     float64
	calls int
}

func (g *gateSvc) Evaluate(context.Context, decision.Request, decision.Options) decision.Result {
	g.calls++
	return decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{"reply": {Kind: decision.KindBoolean, ProbabilityTrue: g.p}}}
}

func TestMessageGateShadowAsksButDoesNotDecide(t *testing.T) {
	svc := &gateSvc{p: 0.95}
	mode := "shadow"
	g := &decisionMessageGate{svc: svc, read: func(k string) string {
		if k == settingDecisionGate {
			return mode
		}
		return ""
	}}
	msg := &message.Message{ID: 7, AuthorID: shared.ActorID("user:alice"), Content: message.MessageContent{Text: "can someone plan this release?"}}
	agent := shared.ActorID("agent:planner")
	if reply, decided := g.ShouldRespond(context.Background(), agent, msg, nil); reply || decided || svc.calls != 1 {
		t.Fatalf("shadow: reply=%v decided=%v calls=%d", reply, decided, svc.calls)
	}
	mode = "on"
	if reply, decided := g.ShouldRespond(context.Background(), agent, msg, nil); !reply || !decided {
		t.Fatalf("on: reply=%v decided=%v", reply, decided)
	}
	mode = "off"
	if _, decided := g.ShouldRespond(context.Background(), agent, msg, nil); decided || svc.calls != 2 {
		t.Fatalf("off must not ask: calls=%d", svc.calls)
	}
}

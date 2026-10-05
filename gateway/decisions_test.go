package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memdoor/pkg/authorization"
	"memdoor/pkg/decision"
	"memdoor/pkg/message"
	"memdoor/pkg/shared"
)

type fakeDecisionService struct {
	res  decision.Result
	last decision.Request
	opts decision.Options
}

func (f *fakeDecisionService) Evaluate(_ context.Context, req decision.Request, opts decision.Options) decision.Result {
	f.last, f.opts = req, opts
	return f.res
}

func okBoolean(id string, p float64) decision.Result {
	return decision.Result{Status: decision.StatusOK, Provider: "fake", Model: "m",
		Answers: map[string]decision.Answer{id: {Kind: decision.KindBoolean, ProbabilityTrue: p, Confidence: p}}}
}

func postDecision(t *testing.T, s *Server, body string, actor string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/decisions/evaluate", strings.NewReader(body))
	if actor != "" {
		req = req.WithContext(authorization.WithActorID(context.Background(), shared.ActorID(actor)))
	}
	rr := httptest.NewRecorder()
	s.handleDecisionsEvaluate(rr, req)
	return rr
}

func TestDecisionsEvaluateWireShape(t *testing.T) {
	fake := &fakeDecisionService{res: okBoolean("reply", 0.91)}
	s := &Server{decisions: fake}
	body := `{"state":"hello","questions":{
	  "reply":{"type":"boolean","instructions":"Reply?","criteria":{"true":"yes","false":"no"}},
	  "dept":{"type":"choice","instructions":"Team?","criteria":{"billing":"Payments","tech":"Bugs"}},
	  "mood":{"type":"score","instructions":"Mood?","criteria":["calm","annoyed","furious"]}
	},"options":{"agentId":"writer","purpose":"test","timeoutMs":500}}`
	rr := postDecision(t, s, body, "u1")
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out["status"] != "ok" {
		t.Fatalf("body: %s", rr.Body.String())
	}
	if fake.opts.AgentID != "writer" || fake.opts.Purpose != "test" || fake.opts.Timeout.Milliseconds() != 500 {
		t.Fatalf("options not passed: %+v", fake.opts)
	}
	if q := fake.last.Questions["dept"]; len(q.Options) != 2 || q.Options[0].Label != "billing" || q.Options[0].Description != "Payments" {
		t.Fatalf("choice criteria object: %+v", q)
	}
	if q := fake.last.Questions["mood"]; q.Kind != decision.KindScore || len(q.Options) != 3 {
		t.Fatalf("score criteria: %+v", q)
	}
	if q := fake.last.Questions["reply"]; q.TrueText != "yes" || q.FalseText != "no" {
		t.Fatalf("boolean criteria: %+v", q)
	}
	answers := out["result"].(map[string]any)["answers"].(map[string]any)
	if answers["reply"].(map[string]any)["probabilityTrue"] != 0.91 {
		t.Fatalf("answer: %+v", answers)
	}
}

func TestDecisionsEvaluateRefusals(t *testing.T) {
	s := &Server{decisions: &fakeDecisionService{res: okBoolean("q", 0.5)}}
	cases := map[string]struct {
		body, actor string
		want        int
	}{
		"anonymous":      {`{"state":"x","questions":{"q":{"type":"boolean","instructions":"?"}}}`, "", http.StatusUnauthorized},
		"bad json":       {`{`, "u1", http.StatusBadRequest},
		"no questions":   {`{"state":"x","questions":{}}`, "u1", http.StatusBadRequest},
		"unknown type":   {`{"state":"x","questions":{"q":{"type":"maybe","instructions":"?"}}}`, "u1", http.StatusBadRequest},
		"score too deep": {`{"state":"x","questions":{"q":{"type":"score","instructions":"?","criteria":["a","b","c","d","e","f","g","h","i","j","k"]}}}`, "u1", http.StatusBadRequest},
		"one option":     {`{"state":"x","questions":{"q":{"type":"choice","instructions":"?","criteria":["a"]}}}`, "u1", http.StatusBadRequest},
	}
	for name, c := range cases {
		if rr := postDecision(t, s, c.body, c.actor); rr.Code != c.want {
			t.Errorf("%s: %d want %d (%s)", name, rr.Code, c.want, strings.TrimSpace(rr.Body.String()))
		}
	}
	// Unavailable is a 200 with a status, never an HTTP error: the consumer decides.
	s = &Server{decisions: &fakeDecisionService{res: decision.Unavailable(decision.ReasonNotConfigured, "none")}}
	rr := postDecision(t, s, `{"state":"x","questions":{"q":{"type":"boolean","instructions":"?"}}}`, "u1")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"reason":"not-configured"`) {
		t.Fatalf("unavailable: %d %s", rr.Code, rr.Body.String())
	}
}

func TestMessageGateThresholdAndUnavailable(t *testing.T) {
	settings := map[string]string{settingDecisionGate: "on"}
	read := func(k string) string { return settings[k] }
	msg := &message.Message{ID: 42, AuthorID: shared.ActorID("user:alice"), ChannelID: "c1",
		Content: message.MessageContent{Text: "can someone summarize yesterday's call?"}}
	recent := []*message.Message{{ID: 41, AuthorID: shared.ActorID("user:bob"), ChannelID: "c1", Content: message.MessageContent{Text: "morning"}}}
	agent := shared.ActorID("agent:writer")

	fake := &fakeDecisionService{res: okBoolean("reply", 0.85)}
	g := &decisionMessageGate{svc: fake, read: read}
	if respond, ok := g.ShouldRespond(context.Background(), agent, msg, recent); !ok || !respond {
		t.Fatalf("0.85 >= default 0.8 should respond: %v %v", respond, ok)
	}
	if !strings.Contains(fake.last.State, "morning") || !strings.Contains(fake.last.State, "summarize") || fake.opts.AgentID != "writer" {
		t.Fatalf("state/opts: %q %+v", fake.last.State, fake.opts)
	}
	settings[settingDecisionThreshold] = "0.9"
	if respond, ok := g.ShouldRespond(context.Background(), agent, msg, recent); !ok || respond {
		t.Fatalf("0.85 < 0.9 must not respond: %v %v", respond, ok)
	}
	g = &decisionMessageGate{svc: &fakeDecisionService{res: decision.Unavailable(decision.ReasonDeadline, "slow")}, read: read}
	if respond, ok := g.ShouldRespond(context.Background(), agent, msg, recent); ok || respond {
		t.Fatal("unavailable must be undecided, never a turn")
	}
	settings[settingDecisionGate] = "off"
	g = &decisionMessageGate{svc: fake, read: read}
	if respond, ok := g.ShouldRespond(context.Background(), agent, msg, recent); ok || respond {
		t.Fatal("gate off must be undecided without calling the model")
	}
}

// JEV IS NOT PRO (Greg, 2026-10-03: "Jev at user key by default"): the key
// the person codes with registers the decision provider, marked as the coding
// key; an explicit decision key wins over it and is not marked.
func TestTheCodingKeyJudgesByDefault(t *testing.T) {
	for _, name := range []string{"OPEN_ROUTER_API_KEY", "OPENROUTER_API_KEY", "MEMDOOR_BYOK", envSystemOneKey, envTypeSafeKey, envSystemOneURL, envSystemOneModel} {
		t.Setenv(name, "")
	}
	if p, _ := systemOneFromEnv(); p != nil {
		t.Fatal("no key at all: no decision provider")
	}
	t.Setenv("OPEN_ROUTER_API_KEY", "sk-or-v1-a-key-for-writing-code")
	p, coding := systemOneFromEnv()
	if p == nil || !p.Ready() || !coding {
		t.Fatalf("the coding key must judge, marked as such: %v %v", p, coding)
	}
	// The old MEMDOOR_BYOK=0 switch is gone (2026-10-04): the key judges.
	t.Setenv("MEMDOOR_BYOK", "0")
	if p, _ := systemOneFromEnv(); p == nil {
		t.Fatal("the coding key judges whatever MEMDOOR_BYOK says")
	}
	t.Setenv("MEMDOOR_BYOK", "")
	t.Setenv(envSystemOneKey, "sk-or-v1-a-key-chosen-for-decisions")
	p, coding = systemOneFromEnv()
	if p == nil || !p.Ready() || coding {
		t.Fatalf("an explicit decision key wins and is not the coding key: %v %v", p, coding)
	}
}

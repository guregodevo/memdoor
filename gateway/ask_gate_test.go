package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"memdoor/pkg/decision"
)

type fakeAskSvc struct {
	p     float64
	unav  bool
	calls int
	state string
}

func (f *fakeAskSvc) Evaluate(_ context.Context, req decision.Request, _ decision.Options) decision.Result {
	f.calls++
	f.state = req.State
	if f.unav {
		return decision.Unavailable(decision.ReasonNotConfigured, "test")
	}
	return decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{
		"answerable": {Kind: decision.KindBoolean, ProbabilityTrue: f.p},
	}}
}

type memSession map[string]interface{}

func (m memSession) SetMetadata(k string, v interface{})           { m[k] = v }
func (m memSession) GetMetadataValue(k string) (interface{}, bool) { v, ok := m[k]; return v, ok }

// When verifiable, don't ask: a question the agent could answer by reading
// or running is refused with the instruction to find out; a real choice
// reaches the person; a refused question asked again gets through.
func TestAskGateRefusesTheVerifiable(t *testing.T) {
	ask, _ := json.Marshal(map[string]any{"question": "Does go test ./... pass on main?", "options": []string{"yes", "no"}})
	choice, _ := json.Marshal(map[string]any{"question": "Which eviction policy should the cache use?", "options": []string{"LRU", "FIFO"}})

	f := &fakeAskSvc{p: 0.95}
	v := &turnVerdict{svc: f, read: settings(nil)}
	s := memSession{}
	out := v.askGate(context.Background(), s, "fix the cache", ask)
	if !strings.HasPrefix(out, "Not asked: ") || !strings.Contains(out, "go test ./... pass") {
		t.Fatalf("verifiable question must be refused: %q", out)
	}
	if !strings.Contains(f.state, "Task: fix the cache") || !strings.Contains(f.state, "Options offered: yes | FIFO") && !strings.Contains(f.state, "Options offered: yes | no") {
		t.Fatalf("state: %s", f.state)
	}
	if again := v.askGate(context.Background(), s, "fix the cache", ask); again != "" {
		t.Fatalf("the same question asked again must reach the person, got %q", again)
	}

	f = &fakeAskSvc{p: 0.1}
	v = &turnVerdict{svc: f, read: settings(nil)}
	if out := v.askGate(context.Background(), memSession{}, "fix the cache", choice); out != "" {
		t.Fatalf("a real choice must reach the person, got %q", out)
	}
	for name, c := range map[string]struct {
		svc *fakeAskSvc
		set map[string]string
	}{
		"unavailable":  {&fakeAskSvc{unav: true}, nil},
		"off":          {&fakeAskSvc{p: 0.99}, map[string]string{settingAskGate: "off"}},
		"under raised": {&fakeAskSvc{p: 0.85}, map[string]string{settingAskGateThreshold: "0.9"}},
	} {
		v := &turnVerdict{svc: c.svc, read: settings(c.set)}
		if out := v.askGate(context.Background(), memSession{}, "fix the cache", ask); out != "" {
			t.Errorf("%s: must let the question through, got %q", name, out)
		}
	}
	var none *turnVerdict
	if none.askGate(context.Background(), memSession{}, "x", ask) != "" {
		t.Error("nil verdict lets it through")
	}
}

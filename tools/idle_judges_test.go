package tools

import (
	"context"
	"testing"

	"memdoor/pkg/decision"
)

type availSvc bool

func (a availSvc) Evaluate(context.Context, decision.Request, decision.Options) decision.Result {
	return decision.Unavailable(decision.ReasonNotConfigured, "test")
}
func (a availSvc) Available(string) bool { return bool(a) }

// jgrep and jread without a decision model are plain grep and read under
// longer descriptions: they are not offered then (2026-09-29).
func TestJudgedToolsOnlyWithADecisionModel(t *testing.T) {
	defs := []ToolDefinition{{Name: "bash"}, {Name: "jgrep"}, {Name: "jread"}, {Name: "read_file"}}
	names := func(ds []ToolDefinition) (out []string) {
		for _, d := range ds {
			out = append(out, d.Name)
		}
		return out
	}
	t.Cleanup(func() { SetDecisionService(nil) })
	for _, c := range []struct {
		svc  decision.Service
		want int
	}{{nil, 2}, {availSvc(false), 2}, {availSvc(true), 4}} {
		SetDecisionService(c.svc)
		if got := WithoutIdleJudges(defs); len(got) != c.want {
			t.Errorf("service %v: offered %v, want %d tools", c.svc, names(got), c.want)
		}
	}
}

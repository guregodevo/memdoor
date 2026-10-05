package gateway

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/decision"
)

// A turn's reasoning effort (providers/reasoning.go sends it). The person's
// Shift+Tab choice wins; left on auto, the decision model picks it for the
// request, once per turn, the way omp's auto-thinking does
// (auto-thinking/classifier.ts): how open-ended the problem is, not how much
// work, ties to the lower level. Without a decision model, or when it does
// not answer in time, high — never the host's default, which for GLM 5.3
// Flash is its maximum (16-46K tokens of thinking before a first tool call,
// 2026-09-29/30).

const (
	effortDefault  = "high"
	effortTimeout  = 4 * time.Second
	effortQuestion = "How open-ended is this request — how much it needs working out, not how much work it is? When unsure, choose the lower level."
)

type effortJudge struct{ svc decision.Service }

// SetEffortJudge installs the decision model's effort pick; nil leaves every
// auto turn at effortDefault.
func (ar *AgentRuntime) SetEffortJudge(j *effortJudge) { ar.effortJudge = j }

// turnEffort is the effort for this turn and where it came from.
func (ar *AgentRuntime) turnEffort(ctx context.Context, s *Session, agent, request string) (string, string) {
	if e := sessionString(s, sessionEffortKey); e != "" {
		return e, "chosen"
	}
	if e := ar.effortJudge.pick(ctx, agent, request); e != "" {
		return e, "decision model"
	}
	return effortDefault, "default"
}

func (j *effortJudge) pick(ctx context.Context, agent, request string) string {
	if j == nil || j.svc == nil || strings.TrimSpace(request) == "" {
		return ""
	}
	if a, ok := j.svc.(decision.Availability); ok && !a.Available(agent) {
		return ""
	}
	q, err := decision.Choice(effortQuestion, []decision.Option{
		{Label: "low", Description: "The answer or the change is clear from the request: a lookup, a small fix, a rename, running something."},
		{Label: "medium", Description: "Some working out: a bug to find, a change touching a few places, a choice between known approaches."},
		{Label: "high", Description: "Open-ended: design, an unclear failure, a problem whose approach has to be found."},
	})
	if err != nil {
		return ""
	}
	start := time.Now()
	res := j.svc.Evaluate(ctx, decision.Request{State: "Request: " + clipHead(request, turnVerdictMaxRequest), Questions: map[string]decision.Question{"effort": q}},
		decision.Options{AgentID: agent, Purpose: "effort", Timeout: effortTimeout})
	if !res.OK() {
		logs.New("Decisions").Info("Effort pick skipped", slog.String("agent", agent), slog.String("reason", string(res.Reason)))
		return ""
	}
	e := res.Answers["effort"].Choice
	switch e {
	case "low", "medium", "high":
	default:
		return ""
	}
	logs.New("Decisions").Info("Effort picked", slog.String("agent", agent), slog.String("effort", e),
		slog.Int64("ms", time.Since(start).Milliseconds()))
	return e
}

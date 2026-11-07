package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/guregodevo/mario/tasks"

	"memdoor/gateway/logs"
	"memdoor/pkg/decision"
)

// A DECISION TASK'S JUDGE (2026-11-07, #160). A `type: decision` task in a
// workflow asks how likely a condition holds of a state — the reviewer's
// findings and what the Check printed, say — and keeps the probability as
// its proof. mario declares the Decider; this is the gateway's, on the same
// decision service as the turn-done check (turn_done.go): the person's own
// OpenRouter key, else an explicit decision key, never the broker. Without
// a service, switched off, late or in error it answers an error, and the
// task's own `else` command decides: a workflow that names the type runs
// to the same result with no decision model at all.

const (
	settingTaskDecision          = "decision_task"           // on (default) / off
	settingTaskDecisionThreshold = "decision_task_threshold" // the probability a decision task passes at when its file names none; default 0.7
	taskDecisionDefaultThreshold = 0.7
	taskDecisionTimeout          = 2500 * time.Millisecond
	// taskDecisionMaxState is how much of the state the judge reads: the
	// decision provider's prompt cap (tool_routing.go fails open past
	// 12 000 characters). The head and the tail are kept, since a Check's
	// verdict is usually at the end of what it printed and the task at the
	// start of the state.
	taskDecisionMaxState = 12000
	taskDecisionPurpose  = "task-decision"
)

// newTaskDecider is the gateway's judge for decision tasks: mario's
// tasks.Decider (and tasks.Thresholder) on the workspace's decision service
// and its settings. The type behind it stays here.
func newTaskDecider(svc decision.Service, read func(key string) string) tasks.Decider {
	return &workflowDecider{svc: svc, read: read}
}

type workflowDecider struct {
	svc  decision.Service
	read func(key string) string
}

var _ tasks.Thresholder = (*workflowDecider)(nil)

func (d *workflowDecider) Decide(ctx context.Context, condition, state string) (float64, error) {
	if d == nil || d.svc == nil {
		return 0, errors.New("no decision model on this gateway")
	}
	if d.read != nil && strings.EqualFold(strings.TrimSpace(d.read(settingTaskDecision)), "off") {
		return 0, errors.New(settingTaskDecision + " is off")
	}
	q, err := decision.Boolean(strings.TrimSpace(condition), "", "")
	if err != nil {
		return 0, err
	}
	start := time.Now()
	res := d.svc.Evaluate(ctx, decision.Request{State: clipMiddle(state, taskDecisionMaxState), Questions: map[string]decision.Question{"holds": q}},
		decision.Options{Purpose: taskDecisionPurpose, Timeout: taskDecisionTimeout})
	log := logs.New("Decisions")
	if !res.OK() {
		log.Info("Decision task not judged", slog.String("reason", string(res.Reason)), slog.String("detail", res.Detail))
		return 0, fmt.Errorf("%s: %s", res.Reason, res.Detail)
	}
	p := res.Answers["holds"].ProbabilityTrue
	log.Info("Decision task judged", slog.Float64("p", p), slog.Int("state_chars", len(state)), slog.Int64("ms", time.Since(start).Milliseconds()))
	return p, nil
}

// DefaultThreshold is the workspace's, else 0.7.
func (d *workflowDecider) DefaultThreshold() float64 {
	if d == nil || d.read == nil {
		return taskDecisionDefaultThreshold
	}
	if t, err := strconv.ParseFloat(strings.TrimSpace(d.read(settingTaskDecisionThreshold)), 64); err == nil && t > 0 && t <= 1 {
		return t
	}
	return taskDecisionDefaultThreshold
}

// clipMiddle keeps the head and the tail of s within n bytes, marking the cut.
func clipMiddle(s string, n int) string {
	if len(s) <= n {
		return s
	}
	const mark = "\n[…]\n"
	head := n * 2 / 3
	tail := n - head - len(mark)
	if tail < 0 {
		tail = 0
	}
	return clipHead(s, head) + mark + clipTail(s, tail)
}

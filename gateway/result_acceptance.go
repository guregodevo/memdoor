package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"memdoor/gateway/logs"
	"memdoor/gateway/subagents"
	"memdoor/pkg/decision"
	"memdoor/tools"
)

// result_acceptance: is a spawned agent's result the task done, or not yet?
//
// A requester spawns the coder, the coder's turn ends, and whatever it said
// last is announced back as the result and acted on as done. Nothing checked
// whether it was: a plan, half the files, a question back or a bare "done"
// all arrived the same way. This asks the decision model one yes/no over the
// task and the result before the announcement, and when the answer is
// "unfinished" the child is run once more with its task, in its own session,
// so it continues rather than starts over. The second result is announced
// whatever the judge says, with the judgment noted, so the requester knows
// what it is holding.
//
// Wording measured 2026-09-26 on Jev over eight labeled results: a plan, a
// partial, a question back, an honest failure, a bare claim and a result for
// another task scored 0.93–0.99; the finished module with its test output
// 0.08, a code answer 0.18.
//
// The neighbours are duck-typed: the review names the few methods it needs
// from the child's session, the run registry and the queue, so the server
// hands in its own types and a test hands in fakes.

const (
	settingResultAcceptance          = "decision_result_acceptance"           // "off" disables
	settingResultAcceptanceThreshold = "decision_result_acceptance_threshold" // P(unfinished) that hands back; default 0.7
	resultAcceptanceDefaultThreshold = 0.7
	resultAcceptanceTimeout          = 2500 * time.Millisecond
	resultAcceptanceMaxTask          = 3000
	resultAcceptanceMaxResult        = 4000
	resultAcceptanceQuestion         = "Does the subagent's result leave the task unfinished (a plan, a partial, a question back, an error, or a bare claim with no evidence) rather than the work done or the question answered?"
	// acceptanceHandbackKey on the child's session records that it was handed
	// back once; a run is never handed back twice.
	acceptanceHandbackKey     = "acceptance_handback"
	acceptanceHandbackTimeout = 5 * time.Minute
)

// sessionMeta is what the review needs from the child's session.
type sessionMeta interface {
	SetMetadata(key string, value interface{})
	GetMetadataValue(key string) (interface{}, bool)
}

// childRuns is what it needs from the subagent registry to re-run a child.
type childRuns interface {
	ReactivateChildSession(childSessionKey string)
	Register(record *subagents.SubagentRunRecord) error
}

// subagentEnqueuer is what it needs from the queue.
type subagentEnqueuer interface {
	EnqueueSubagentJob(runID, sessionKey, agentID, message string, tools []string, systemPrompt, workdir string, timeout time.Duration) error
}

type resultAcceptance struct {
	svc     decision.Service
	read    func(key string) string
	runs    childRuns
	enqueue subagentEnqueuer
	session func(key string) (sessionMeta, error)
	resolve func(agentID string) (tools []string, systemPrompt string)
}

// resultReview is what the caller does with the result.
type resultReview struct {
	Judged     bool
	Unfinished bool    // the judge's answer at the threshold
	P          float64 // P(unfinished)
	HandedBack bool    // the child runs again; do not announce this result
	Note       string  // for the announcement, "" when there is nothing to say
}

// review judges a completed run's result and, the first time it is judged
// unfinished, hands the task back to the child. Never judges an error or
// timeout outcome (the failure is the announcement), an empty result, or a
// flow step (the flow driver owns those).
func (a *resultAcceptance) review(ctx context.Context, record *subagents.SubagentRunRecord, result string) resultReview {
	if a == nil || a.svc == nil || record == nil || strings.TrimSpace(result) == "" {
		return resultReview{}
	}
	if strings.EqualFold(strings.TrimSpace(a.read(settingResultAcceptance)), "off") {
		return resultReview{}
	}
	if record.Outcome != nil && record.Outcome.Status != subagents.OutcomeOK && record.Outcome.Status != "" {
		return resultReview{}
	}
	if record.Label == "flow-step" {
		return resultReview{}
	}
	log := logs.New("Decisions")
	q, err := decision.Boolean(resultAcceptanceQuestion, "", "")
	if err != nil {
		return resultReview{}
	}
	state := "Task given to the subagent: " + clipHead(record.Task, resultAcceptanceMaxTask) +
		"\n\nSubagent's result:\n" + clipTail(result, resultAcceptanceMaxResult)
	agent := agentFromChildKey(record.ChildSessionKey)
	start := time.Now()
	res := a.svc.Evaluate(ctx, decision.Request{State: state, Questions: map[string]decision.Question{"unfinished": q}},
		decision.Options{AgentID: agent, Purpose: "result-acceptance", Timeout: resultAcceptanceTimeout})
	if !res.OK() {
		log.Info("Result acceptance skipped", slog.String("run", record.RunID), slog.String("reason", string(res.Reason)))
		return resultReview{}
	}
	p := res.Answers["unfinished"].ProbabilityTrue
	rv := resultReview{Judged: true, P: p, Unfinished: p >= a.threshold()}

	var sess sessionMeta
	if a.session != nil {
		sess, _ = a.session(record.ChildSessionKey)
	}
	handedBackBefore := false
	if sess != nil {
		_, handedBackBefore = sess.GetMetadataValue(acceptanceHandbackKey)
	}
	log.Info("Result acceptance", slog.String("run", record.RunID), slog.String("agent", agent),
		slog.Float64("p_unfinished", p), slog.Bool("unfinished", rv.Unfinished),
		slog.Bool("handed_back_before", handedBackBefore), slog.Int64("ms", time.Since(start).Milliseconds()))

	switch {
	case rv.Unfinished && !handedBackBefore && sess != nil:
		if err := a.handBack(record, agent, sess, p); err != nil {
			log.Warn("Result judged unfinished; hand-back not possible, announcing as is",
				slog.String("run", record.RunID), slog.String("error", err.Error()))
			rv.Note = fmt.Sprintf("Result check: judged UNFINISHED (P=%.2f). Do not report it as done: re-spawn naming the missing part, or finish it yourself.", p)
			return rv
		}
		rv.HandedBack = true
		return rv
	case rv.Unfinished:
		rv.Note = fmt.Sprintf("Result check: handed back once and still judged UNFINISHED (P=%.2f). Do not report it as done: re-spawn naming the missing part, or finish it yourself.", p)
	case handedBackBefore:
		rv.Note = fmt.Sprintf("Result check: handed back once, now judged done (P(unfinished)=%.2f).", p)
	}
	return rv
}

// handBack runs the child once more, in its own session, with its task.
// Mirrors the flow driver's re-dispatch (server_flow.go): the prior run is
// cleared so the fresh one owns the session's lifecycle.
func (a *resultAcceptance) handBack(record *subagents.SubagentRunRecord, agent string, sess sessionMeta, p float64) error {
	if a.runs == nil || a.enqueue == nil || a.resolve == nil {
		return fmt.Errorf("hand-back not wired")
	}
	if agent == "" {
		return fmt.Errorf("no agent in child session key %q", record.ChildSessionKey)
	}
	palette, prompt := a.resolve(agent)
	if len(palette) == 0 {
		return fmt.Errorf("agent %q palette could not be resolved", agent)
	}
	workdir := ""
	if wd, ok := sess.GetMetadataValue(tools.RequesterWorkdirKey); ok {
		workdir, _ = wd.(string)
	}
	sess.SetMetadata(acceptanceHandbackKey, time.Now().Format(time.RFC3339))

	a.runs.ReactivateChildSession(record.ChildSessionKey)
	rerun := &subagents.SubagentRunRecord{
		RunID:               uuid.New().String(),
		ChildSessionKey:     record.ChildSessionKey,
		RequesterSessionKey: record.RequesterSessionKey,
		RequesterDisplayKey: record.RequesterDisplayKey,
		Task:                record.Task,
		Cleanup:             record.Cleanup,
		Label:               record.Label,
		ParentMessageID:     record.ParentMessageID,
		CreatedAt:           time.Now(),
		ArchiveAtMs:         time.Now().Add(24 * time.Hour).UnixMilli(),
	}
	if err := a.runs.Register(rerun); err != nil {
		return fmt.Errorf("register re-run: %w", err)
	}
	msg := handbackMessage(record.Task, p)
	if err := a.enqueue.EnqueueSubagentJob(rerun.RunID, record.ChildSessionKey, agent, msg, palette, prompt, workdir, acceptanceHandbackTimeout); err != nil {
		return fmt.Errorf("enqueue re-run: %w", err)
	}
	logs.New("Decisions").Info("Result handed back to the child", slog.String("run", record.RunID),
		slog.String("rerun", rerun.RunID), slog.String("agent", agent), slog.Float64("p_unfinished", p))
	return nil
}

func handbackMessage(task string, p float64) string {
	return fmt.Sprintf("Your result was checked against the task and judged unfinished (P=%.2f): it reads as a plan, a partial, a question, or a claim without evidence.\n\n"+
		"The task again:\n%s\n\n"+
		"Finish it now: do the remaining work with your tools, then report what is done with evidence (test output, file paths, the answer itself). "+
		"If some part cannot be done, say exactly which part and why.", p, task)
}

func (a *resultAcceptance) threshold() float64 {
	if v, err := strconv.ParseFloat(strings.TrimSpace(a.read(settingResultAcceptanceThreshold)), 64); err == nil && v >= 0 && v <= 1 {
		return v
	}
	return resultAcceptanceDefaultThreshold
}

// agentFromChildKey reads the agent out of "agent:<agent>:subagent:<run>".
func agentFromChildKey(key string) string {
	parts := strings.Split(key, ":")
	if len(parts) >= 4 && parts[0] == "agent" && parts[2] == "subagent" {
		return parts[1]
	}
	return ""
}

// verdictOf is the review's answer for the announcement's closing directive.
func verdictOf(rv resultReview) string {
	switch {
	case !rv.Judged:
		return ""
	case rv.Unfinished:
		return subagents.VerdictUnfinished
	default:
		return subagents.VerdictDone
	}
}

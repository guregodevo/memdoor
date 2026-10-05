package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/decision"
)

// turn_verdict: does a reply that ends the turn only ANNOUNCE work?
//
// announcesUnmadeAction reads the reply's tail for a list of phrases. A new
// wording slips past it: live 2026-09-26 an agent ended on a sentence that
// announced the next step without taking it; no phrase matched, the turn was
// reported done, and the work was never made. When the list says no, one yes/no to the
// decision model asks the question the list approximates. It is consulted
// only where the phrase list already would be — a reply with no tool call —
// and unavailable means the list's answer stands, as before.
//
// Wording measured 2026-09-26 on Jev over six replies: the two announcing
// unfinished work scored 0.82 and 0.83, the four finished ones (a delivered
// short, a code answer, a fix reported with its test output, a diagnosis
// ending on advice for next time) 0.02–0.03.
//
// Reworded 2026-10-04: a finished review ending on its "one next action" for
// the person scored 0.72 live and was retried as an announcement (the retry cut
// the answer from the window). The question now says whose work: re-measured
// on Jev, that review 0.40 → 0.11, a reply offering the person a next step
// 0.09 → 0.04, three genuine announcements 0.96–0.97 → 0.93–0.95, a finished
// answer 0.02 either way.

const (
	settingTurnVerdict    = "decision_turn_verdict"
	turnVerdictThreshold  = 0.7
	turnVerdictTimeout    = 2500 * time.Millisecond
	turnVerdictMaxRequest = 2000
	turnVerdictMaxReply   = 1500
	turnVerdictQuestion   = "Does the agent's final message end by announcing work the agent itself is about to do now, and stop there without doing it — rather than reporting finished work, answering, or suggesting or offering a next step for the person to decide?"

	// The second question, asked in the same call: does the reply address the
	// request at all? A finished-sounding reply about something else passed
	// every check. Measured 2026-09-26 on eight labeled replies: three
	// off-request ones 0.89–0.98; three answers, a clarifying question and an
	// honest "it does not exist" 0.06–0.29.
	offRequestQuestion  = "Does the agent's final message fail to address the request (it talks about something else, or leaves the request unanswered without saying why)?"
	offRequestThreshold = 0.8

	// The stop question: asked once per turn, when the shadow score
	// (route_shadow.go) says the last steps made no progress. A yes ends the
	// turn with a wrap-up round. The decision model, not a call count, is
	// what ends a run (Greg, 2026-09-26: "with decision model we have a
	// control on how to end"; "instead of hard coded limits").
	settingTurnStop   = "decision_turn_stop"
	turnStopThreshold = 0.8

	turnStopTimeout  = 3000 * time.Millisecond
	turnStopQuestion = "Looking at the agent's last steps, is this run stuck — repeating the same actions or failing the same way without progress toward the request — so that it should stop and report rather than keep going?"
	// The explore question: asked every 30 reads of a turn that has changed
	// nothing (route_shadow.go exploreDue). Measured 2026-09-28 on Jev: the
	// live "continue the work in this PR" turn at 129 calls, 0 edits scored
	// 0.83–0.85 over three asks; a 31-read race review that must not edit
	// 0.15; a 30-read "where is X" question 0.41–0.46. The stop threshold
	// separates them. A wording that asked "has it read enough to start"
	// scored the stuck turn 0.64.
	exploreStopQuestion = "Is the agent still only reading and searching, many steps in, when the request asks it to make a change — so it should stop exploring and report what it found and the change it would make?"
	// The same check for a turn that already changed files: reading since
	// the last change is normal work (the callers of a rename, the rest of a
	// review), going in circles is not. Measured 2026-09-28 on Jev: the live
	// turn that edited tunnel.go then re-read one goroutine dump for ~50
	// steps 0.91 (twice); a review reading handler after handler 0.12-0.13;
	// a rename reading its callers 0.16-0.17. "Only reading many steps in"
	// scored all three 0.77-0.85 — after an edit it cannot tell them apart.
	circlesStopQuestion = "Look at the agent's last steps. Is it going in circles — re-reading the same file or the same output, or inspecting one artifact from different angles — instead of moving through different parts of the work?"
)

type turnVerdict struct {
	svc  decision.Service
	read func(key string) string
}

// announcesUndone reports whether the decision model judges the reply to end
// on an announcement. False when off, unavailable, or below the threshold.
func (v *turnVerdict) announcesUndone(ctx context.Context, agent, request, reply string) bool {
	if v == nil || v.svc == nil || strings.TrimSpace(reply) == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(v.read(settingTurnVerdict)), "off") {
		return false
	}
	q, err := decision.Boolean(turnVerdictQuestion, "", "")
	if err != nil {
		return false
	}
	q2, err := decision.Boolean(offRequestQuestion, "", "")
	if err != nil {
		return false
	}
	state := "Request: " + clipHead(request, turnVerdictMaxRequest) +
		"\n\nAgent's final message (no tool call followed it):\n" + clipTail(reply, turnVerdictMaxReply)
	start := time.Now()
	res := v.svc.Evaluate(ctx, decision.Request{State: state, Questions: map[string]decision.Question{"announces": q, "off_request": q2}},
		decision.Options{AgentID: agent, Purpose: "turn-verdict", Timeout: turnVerdictTimeout})
	log := logs.New("Decisions")
	if !res.OK() {
		log.Info("Turn verdict skipped", slog.String("agent", agent), slog.String("reason", string(res.Reason)))
		return false
	}
	p := res.Answers["announces"].ProbabilityTrue
	off := res.Answers["off_request"].ProbabilityTrue
	retry := p >= turnVerdictThreshold || off >= offRequestThreshold
	log.Info("Turn verdict", slog.String("agent", agent), slog.Float64("p_announces", p),
		slog.Float64("p_off_request", off), slog.Bool("retry", retry), slog.Int64("ms", time.Since(start).Milliseconds()))
	return retry
}

func clipHead(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func clipTail(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return "…" + string(r[len(r)-n:])
	}
	return s
}

// stuck asks the decision model whether the last steps show a run that
// should stop. Returns P(stuck) and whether it clears the threshold. False
// when off or unavailable: the run goes on, as before the question existed.
func (v *turnVerdict) stuck(ctx context.Context, agent, request, window string) (float64, bool) {
	if v == nil || v.svc == nil || strings.TrimSpace(window) == "" {
		return 0, false
	}
	if strings.EqualFold(strings.TrimSpace(v.read(settingTurnStop)), "off") {
		return 0, false
	}
	q, err := decision.Boolean(turnStopQuestion, "", "")
	if err != nil {
		return 0, false
	}
	state := "Request: " + clipHead(request, turnVerdictMaxRequest) + "\n\nAgent's last steps, oldest first:\n" + window
	start := time.Now()
	res := v.svc.Evaluate(ctx, decision.Request{State: state, Questions: map[string]decision.Question{"stuck": q}},
		decision.Options{AgentID: agent, Purpose: "turn-stop", Timeout: turnStopTimeout})
	log := logs.New("Decisions")
	if !res.OK() {
		log.Info("Turn stop skipped", slog.String("agent", agent), slog.String("reason", string(res.Reason)))
		return 0, false
	}
	p := res.Answers["stuck"].ProbabilityTrue
	stop := p >= turnStopThreshold
	log.Info("Turn stop verdict", slog.String("agent", agent), slog.Float64("p_stuck", p), slog.Bool("stop", stop),
		slog.Int64("ms", time.Since(start).Milliseconds()))
	return p, stop
}

// exploring asks whether a turn that has read reads times and changed nothing
// should stop and report. Same setting, threshold and fallback as stuck.
func (v *turnVerdict) exploring(ctx context.Context, agent, request, window string, reads int, edited bool) (float64, bool) {
	if v == nil || v.svc == nil || strings.TrimSpace(window) == "" {
		return 0, false
	}
	if strings.EqualFold(strings.TrimSpace(v.read(settingTurnStop)), "off") {
		return 0, false
	}
	question, so := exploreStopQuestion, fmt.Sprintf("%d reads, 0 edits", reads)
	if edited {
		question, so = circlesStopQuestion, fmt.Sprintf("it changed files, then made %d reads and searches without another change", reads)
	}
	q, err := decision.Boolean(question, "", "")
	if err != nil {
		return 0, false
	}
	state := fmt.Sprintf("Request: %s\n\nThis turn so far: %s.\nAgent's last steps, oldest first:\n%s",
		clipHead(request, turnVerdictMaxRequest), so, window)
	start := time.Now()
	res := v.svc.Evaluate(ctx, decision.Request{State: state, Questions: map[string]decision.Question{"exploring": q}},
		decision.Options{AgentID: agent, Purpose: "turn-stop", Timeout: turnStopTimeout})
	log := logs.New("Decisions")
	if !res.OK() {
		log.Info("Explore stop skipped", slog.String("agent", agent), slog.String("reason", string(res.Reason)))
		return 0, false
	}
	p := res.Answers["exploring"].ProbabilityTrue
	stop := p >= turnStopThreshold
	log.Info("Explore stop verdict", slog.String("agent", agent), slog.Int("reads", reads), slog.Float64("p", p),
		slog.Bool("stop", stop), slog.Int64("ms", time.Since(start).Milliseconds()))
	return p, stop
}

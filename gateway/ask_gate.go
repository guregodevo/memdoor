package gateway

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/pkg/decision"
	"memdoor/tools"
)

// ask_gate: WHEN VERIFIABLE, DON'T ASK (Greg, 2026-10-03: "anything
// verifiable could save question babysitting", "when verifiable", "dont
// ask", "everything gated by Jev").
//
// A question to the person is the one interruption a turn has, and a model
// reaches for it to settle things the repository or a command would settle:
// which file holds X, whether a test passes, what a function returns. Before
// ask_user_question reaches the window, one yes/no asks the decision model
// whether the agent could answer it alone — by reading, querying, running or
// validating. A yes refuses the call with the instruction to go and find
// out; the same question asked again is let through, so a model that
// insists is not walled off. Off, unavailable or under the threshold: the
// question reaches the person, as before.

const (
	settingAskGate          = "decision_ask_gate"
	settingAskGateThreshold = "decision_ask_gate_threshold"
	askGateDefaultThreshold = 0.8
	askGateTimeout          = 2500 * time.Millisecond
	askGateQuestion         = "Could the agent answer this question itself — by reading the code or files, querying, running a command or test, " +
		"or checking a result — rather than asking the person? (A choice of design or preference, or a missing requirement only the person knows, cannot.)"
	askGateRefusedKey = "ask_gate_refused:"
)

// askGate judges an ask_user_question call. Returns the refusal the model
// reads, or "" to let the question through.
func (v *turnVerdict) askGate(ctx context.Context, session interface {
	SetMetadata(string, interface{})
	GetMetadataValue(string) (interface{}, bool)
}, task string, input json.RawMessage) string {
	if v == nil || v.svc == nil || session == nil {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(v.read(settingAskGate)), "off") {
		return ""
	}
	var in tools.AskUserQuestionInput
	if json.Unmarshal(input, &in) != nil || strings.TrimSpace(in.Question) == "" {
		return ""
	}
	key := askGateRefusedKey + questionHash(in.Question)
	if _, refusedBefore := session.GetMetadataValue(key); refusedBefore {
		return "" // asked again after a refusal: the person hears it
	}
	threshold := askGateDefaultThreshold
	if t, ok := parseProbability(v.read(settingAskGateThreshold)); ok {
		threshold = t
	}
	q, err := decision.Boolean(askGateQuestion, "", "")
	if err != nil {
		return ""
	}
	state := "Task: " + clipHead(task, turnVerdictMaxRequest) +
		"\n\nQuestion the agent wants to ask the person: " + clipHead(in.Question, 600)
	if len(in.Options) > 0 {
		state += "\nOptions offered: " + strings.Join(in.Options, " | ")
	}
	start := time.Now()
	res := v.svc.Evaluate(ctx, decision.Request{State: state, Questions: map[string]decision.Question{"answerable": q}},
		decision.Options{Purpose: "ask-gate", Timeout: askGateTimeout})
	log := logs.New("Decisions")
	if !res.OK() {
		log.Info("Ask gate skipped", slog.String("reason", string(res.Reason)))
		return ""
	}
	p := res.Answers["answerable"].ProbabilityTrue
	refuse := p >= threshold
	log.Info("Ask gate", slog.Float64("p_answerable", p), slog.Bool("refused", refuse),
		slog.String("question", clipHead(in.Question, 120)), slog.Int64("ms", time.Since(start).Milliseconds()))
	if !refuse {
		return ""
	}
	session.SetMetadata(key, time.Now().UTC().Format(time.RFC3339))
	return fmt.Sprintf("Not asked: this is something you can find out yourself — read the code, query, run a command or test, check the result — "+
		"then act on what you find. Ask the person only for a choice or a requirement that cannot be verified without them. (Question held back: %s)",
		clipHead(in.Question, 200))
}

// decisionNote puts a harness decision in the window as a system line (the
// provider-notice event the window already renders), so the person sees the
// harness act instead of wondering why a turn went on or a question never
// came (Greg, 2026-10-03: "add notes when a decision is made").
func (ar *AgentRuntime) decisionNote(runID, sessionID, text string) {
	if ar == nil || ar.events == nil {
		return
	}
	ar.events.EmitEvent(runID, infra.EventStreamLifecycle, sessionID, map[string]interface{}{
		"event":   "provider_notice",
		"message": "⚖ " + text,
	})
}

func questionHash(q string) string {
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(q))))
	return hex.EncodeToString(sum[:8])
}

func parseProbability(s string) (float64, bool) {
	var t float64
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%g", &t); err != nil || t <= 0 || t > 1 {
		return 0, false
	}
	return t, true
}

// askedQuestion is the question text of an ask_user_question input, short.
func askedQuestion(input json.RawMessage) string {
	var in tools.AskUserQuestionInput
	_ = json.Unmarshal(input, &in)
	return clipHead(strings.TrimSpace(in.Question), 120)
}

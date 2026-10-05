package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"memdoor/pkg/decision"
)

// decision_evaluate — the agent asks the workspace's decision model typed
// questions about some content and gets probabilities back, instead of
// reasoning the classification out in prose. OpenClaw provides the same tool
// when an agent has an effective decisionModel; here it answers when the
// gateway has a decision model — the person's own OpenRouter key, a decision
// key, or a seat (docs/features/DECIDE.md). The result may be "unavailable": that is not a
// no, it means decide some other way.

const DecisionEvaluateName = "decision_evaluate"

type DecisionQuestionInput struct {
	ID           string   `json:"id" jsonschema_description:"Your key for this question; the answer comes back under it."`
	Kind         string   `json:"kind" jsonschema_description:"choice | score | boolean"`
	Instructions string   `json:"instructions" jsonschema_description:"The question, one sentence, about the state."`
	Options      []string `json:"options,omitempty" jsonschema_description:"choice: 2..26 labels to pick from, in display order."`
	Levels       []string `json:"levels,omitempty" jsonschema_description:"score: 2..10 rubric levels from low to high, each a short anchor phrase."`
}

type DecisionEvaluateInput struct {
	State     string                  `json:"state" jsonschema_description:"The content to decide about: a message, a transcript excerpt, a diff, a tool call. Plain text."`
	Questions []DecisionQuestionInput `json:"questions" jsonschema_description:"One or more typed questions about the state."`
	Purpose   string                  `json:"purpose,omitempty" jsonschema_description:"Why you are asking, a few words, for the log."`
}

var DecisionEvaluateInputSchema = GenerateSchema[DecisionEvaluateInput]()

// DecisionEvaluateDefinition is a standard tool; the service comes from
// SetDecisionService.
var DecisionEvaluateDefinition = ToolDefinition{
	Name: DecisionEvaluateName,
	Description: "Ask the workspace's decision model typed questions about a piece of content and get a probability " +
		"for every option, in milliseconds, with no text to parse. Use it for bounded judgments you would otherwise " +
		"reason out: which category, which of these candidates, yes or no, how severe on a rubric. Kinds: choice " +
		"(pick one label, distribution over labels), score (position on ordered levels, fractional), boolean " +
		"(probability of true). Treat a probability near 0.5 as \"unsure\" and a status of \"unavailable\" as " +
		"\"decide another way\", never as a no.",
	InputSchema: DecisionEvaluateInputSchema,
	Function:    DecisionEvaluate,
}

var (
	decisionSvcMu sync.RWMutex
	decisionSvc   decision.Service
)

// SetDecisionService installs the workspace decision model for the tools that
// use it (decision_evaluate, jgrep). Called once at gateway start, the same
// way SetGatewayBaseURL is; nil leaves them answering "unavailable" / plain grep.
func SetDecisionService(svc decision.Service) {
	decisionSvcMu.Lock()
	defer decisionSvcMu.Unlock()
	decisionSvc = svc
}

// judgedTools only do their job with a decision model; without one they are
// plain grep and read under longer descriptions (Greg, 2026-09-29: "it's using
// tool like jgrep with it off", "it wont work").
var judgedTools = map[string]bool{"jgrep": true, "jread": true}

// WithoutIdleJudges leaves the judged tools out of what a turn submits when
// no decision model is available: no OpenRouter key, no seat, or a company's
// vendor key without a decision key.
func WithoutIdleJudges(defs []ToolDefinition) []ToolDefinition {
	if a, ok := decisionService().(decision.Availability); ok && a.Available("") {
		return defs
	}
	out := make([]ToolDefinition, 0, len(defs))
	for _, d := range defs {
		if !judgedTools[d.Name] {
			out = append(out, d)
		}
	}
	return out
}

func decisionService() decision.Service {
	decisionSvcMu.RLock()
	defer decisionSvcMu.RUnlock()
	return decisionSvc
}

// DecisionEvaluate is the tool function.
func DecisionEvaluate(input json.RawMessage) (string, error) {
	var in DecisionEvaluateInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	req, err := in.toRequest()
	if err != nil {
		return "", err
	}
	svc := decisionService()
	if svc == nil {
		out, _ := json.Marshal(RenderDecisionResult(decision.Unavailable(decision.ReasonNotConfigured, "no decision model configured")))
		return string(out), nil
	}
	res := svc.Evaluate(context.Background(), req, decision.Options{Purpose: in.Purpose})
	out, err := json.Marshal(RenderDecisionResult(res))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (in DecisionEvaluateInput) toRequest() (decision.Request, error) {
	req := decision.Request{State: in.State, Questions: map[string]decision.Question{}}
	for i, qi := range in.Questions {
		id := strings.TrimSpace(qi.ID)
		if id == "" {
			id = fmt.Sprintf("q%d", i+1)
		}
		kind, err := decision.ParseKind(qi.Kind)
		if err != nil {
			return req, fmt.Errorf("question %q: %w", id, err)
		}
		var q decision.Question
		switch kind {
		case decision.KindBoolean:
			q, err = decision.Boolean(qi.Instructions, "", "")
		case decision.KindScore:
			q, err = decision.Score(qi.Instructions, qi.Levels)
		default:
			q, err = decision.Choice(qi.Instructions, decision.Labels(qi.Options...))
		}
		if err != nil {
			return req, fmt.Errorf("question %q: %w", id, err)
		}
		req.Questions[id] = q
	}
	return req, nil
}

// RenderDecisionResult is the one JSON shape a Result takes on every wire
// (tool result, HTTP response, CLI): OpenClaw's status/result/reason layout.
func RenderDecisionResult(res decision.Result) map[string]any {
	if !res.OK() {
		return map[string]any{"status": string(res.Status), "reason": string(res.Reason), "detail": res.Detail}
	}
	answers := make(map[string]any, len(res.Answers))
	for id, a := range res.Answers {
		m := map[string]any{"type": string(a.Kind), "confidence": a.Confidence}
		switch a.Kind {
		case decision.KindBoolean:
			m["probabilityTrue"] = a.ProbabilityTrue
		case decision.KindScore:
			m["score"] = a.Score
			m["probabilities"] = a.ScoreProbabilities
		default:
			m["choice"] = a.Choice
			m["probabilities"] = a.Probabilities
		}
		answers[id] = m
	}
	return map[string]any{
		"status": string(res.Status),
		"result": map[string]any{"answers": answers, "provider": res.Provider, "model": res.Model},
	}
}

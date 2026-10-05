package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/providers"
	"memdoor/pkg/authorization"
	"memdoor/pkg/decision"
	"memdoor/pkg/decision/systemone"
	"memdoor/pkg/message"
	"memdoor/pkg/shared"
	"memdoor/tools"
)

// Decision models, the OpenClaw way: providers sit behind one service,
// consumers call one Evaluate and handle "unavailable" themselves. Nothing
// here ever turns a missing decision model into a chat completion. There is
// no switch (Greg, 2026-09-29: "remove the toggle on and off"). JEV IS NOT
// PRO (Greg, 2026-10-03: "Jev at user key by default", "jev decision model is
// not pro"): a decision key, or the person's own OpenRouter key, judges —
// never a seat through the broker (no seat supplies a key, 2026-10-04) — and
// otherwise nothing is judged (autoDecisionModel, decision_model.go).
//
//	decision_message_gate   on — let a configured model decide which agent
//	                        answers a channel message nobody @mentioned
//	decision_gate_threshold P(respond) an agent needs to take the turn (0.8)
//
// The System One provider (hosted Jev or a local Kev server) is configured
// from the environment: MEMDOOR_SYSTEMONE_URL (default api.typesafe.ai),
// MEMDOOR_SYSTEMONE_API_KEY or TYPESAFE_API_KEY, MEMDOOR_SYSTEMONE_MODEL.
const (
	settingDecisionGate      = "decision_message_gate"
	settingDecisionThreshold = "decision_gate_threshold"
	defaultGateThreshold     = 0.8

	envSystemOneURL   = "MEMDOOR_SYSTEMONE_URL"
	envSystemOneKey   = "MEMDOOR_SYSTEMONE_API_KEY"
	envTypeSafeKey    = "TYPESAFE_API_KEY"
	envSystemOneModel = "MEMDOOR_SYSTEMONE_MODEL"

	openRouterDecisionsURL = "https://openrouter.ai/api/alpha/decisions"
	openRouterJevModel     = "~typesafe/jev-latest"
)

// dlog is resolved per call: the global logger does not exist at package init.
func dlog() *logs.EventLogger { return logs.New("Decisions") }

// newWorkspaceSettingReader is THE reader for workspace_settings rows (tool
// guards, decision model, gate): one query, one place.
func newWorkspaceSettingReader(db *sql.DB) func(key string) string {
	return func(key string) string {
		var v string
		_ = db.QueryRow(`SELECT value FROM workspace_settings WHERE workspace_id = '' AND key = ?`, key).Scan(&v)
		return v
	}
}

// newDecisionService assembles the providers this gateway can route to:
// System One, when a decision key or the person's OpenRouter key configures
// it. Never the broker: no seat supplies a key (2026-10-04).
func newDecisionService(read func(key string) string) (decision.Service, error) {
	var provs []decision.Provider
	direct, codingKey := systemOneFromEnv()
	if direct != nil {
		provs = append(provs, direct)
	}
	resolve := func(string) string { return autoDecisionModel(direct, codingKey) }
	return decision.NewService(resolve, provs...)
}

// systemOneFromEnv builds the System One provider: from a DECISION key
// (MEMDOOR_SYSTEMONE_API_KEY, or TYPESAFE_API_KEY for api.typesafe.ai), else
// from the person's own OpenRouter key, which serves Jev on OpenRouter.
// codingKey says it is the latter: on a company's vendor key that one must not
// be used, since nothing but the vendor may be contacted (autoDecisionModel).
//
// THE CODING KEY JUDGES BY DEFAULT (Greg, 2026-10-03: "Jev at user key by
// default", "jev decision model is not pro"). This reverses 2026-09-27, when
// the decision model was what a seat bought: Pi 1.0 hands any user Jev on
// their own key, so selling access to it is selling nothing.
func systemOneFromEnv() (p decision.Provider, codingKey bool) {
	key, base, model := "", os.Getenv(envSystemOneURL), os.Getenv(envSystemOneModel)
	// A DECISION PROVIDER FIRST, connected or from the environment
	// (`memdoor connect typesafe`, providers/registry.go): Jev on its own
	// key, whatever provider answers the chat (Greg, 2026-10-03: "how pi do
	// without OpenRouter?" → decisions need no OpenRouter account).
	if dp, ok := providers.DecisionProvider(); ok {
		key = dp.Key
		if base == "" && !strings.HasPrefix(key, "sk-or-") {
			base = dp.Base
		}
	}
	if key == "" && base == "" {
		if key = providers.ByokKey(); key != "" {
			codingKey = true
		}
	}
	// A decision key pointed at OpenRouter still needs the endpoint and model
	// that serve Jev there; it just has to be a key someone chose for this.
	if key != "" && strings.HasPrefix(key, "sk-or-") {
		if base == "" {
			base = openRouterDecisionsURL
		}
		if model == "" {
			model = openRouterJevModel
		}
	}
	if key == "" && base == "" {
		return nil, false
	}
	c, err := systemone.New(systemone.Config{BaseURL: base, APIKey: key, Model: model})
	if err != nil {
		dlog().Warn("System One provider not registered", slog.String("error", err.Error()))
		return nil, false
	}
	dlog().Info("System One decision provider registered", slog.String("endpoint", c.Endpoint()), slog.String("model", model), slog.Bool("own_openrouter_key", codingKey))
	return c, codingKey
}

// wireDecisions builds the service, installs it for the HTTP route and the
// decision tools (jgrep, decision_evaluate), and attaches the message gate.
func (s *Server) wireDecisions(db *sql.DB, messageService *message.Service) {
	read := newWorkspaceSettingReader(db)
	svc, err := newDecisionService(read)
	if err != nil {
		dlog().Error("Decision service not wired", slog.String("error", err.Error()))
		return
	}
	s.decisions = svc
	// jgrep and decision_evaluate are standard tools; they read the service
	// installed here and fall back (plain grep / "unavailable") without a model.
	tools.SetDecisionService(svc)
	s.agent.SetToolRouter(&toolRouter{svc: svc, read: read})
	s.agent.SetTurnVerdict(&turnVerdict{svc: svc, read: read})
	s.agent.SetEffortJudge(&effortJudge{svc: svc})
	// The result review's neighbours, each behind the methods it needs; a nil
	// neighbour means the review judges but cannot hand back.
	acc := &resultAcceptance{svc: svc, read: read, resolve: s.resolveAgentConfig}
	if s.subagentRegistry != nil {
		acc.runs = s.subagentRegistry
	}
	if s.queueManager != nil {
		acc.enqueue = s.queueManager
	}
	if s.sessions != nil {
		sessions := s.sessions
		acc.session = func(key string) (sessionMeta, error) {
			sess, err := sessions.GetSession(key)
			if err != nil {
				return nil, err
			}
			if sess == nil {
				return nil, fmt.Errorf("session %q not found", key)
			}
			return sess, nil
		}
	}
	s.acceptance = acc
	dlog().Info("Decision service installed for jgrep and decision_evaluate")
	// The gate is always attached and consults decision_message_gate per
	// message, so an admin can switch it on or off without a restart.
	messageService.SetMessageGate(&decisionMessageGate{svc: svc, read: read})
	dlog().Info("Message gate attached", slog.Bool("enabled", isOn(read(settingDecisionGate))))
}

func isOn(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "on", "1", "true", "yes":
		return true
	}
	return false
}

// ---- message gate ----------------------------------------------------------

// decisionMessageGate asks the decision model one boolean per candidate
// agent: should this agent answer the latest message. The threshold is read
// per call so an admin can tune it without a restart.
type decisionMessageGate struct {
	svc  decision.Service
	read func(key string) string
}

func (g *decisionMessageGate) threshold() float64 {
	if v, err := strconv.ParseFloat(strings.TrimSpace(g.read(settingDecisionThreshold)), 64); err == nil && v > 0 && v < 1 {
		return v
	}
	return defaultGateThreshold
}

// ShouldRespond answers whether agentID should take a turn on msg. With
// decision_message_gate=shadow the question is asked and the verdict logged
// ("Message gate decision" shadow=true, would_reply) but nothing is decided:
// the gate collects the evidence for switching it on without changing who
// answers.
func (g *decisionMessageGate) ShouldRespond(ctx context.Context, agentID shared.ActorID, msg *message.Message, recent []*message.Message) (bool, bool) {
	mode := strings.ToLower(strings.TrimSpace(g.read(settingDecisionGate)))
	shadow := mode == "shadow"
	if !shadow && !isOn(mode) {
		return false, false
	}
	q, err := decision.Boolean(
		fmt.Sprintf("Should the agent %q reply to the latest message?", agentID.ID()),
		"The latest message asks for something this agent can do, or continues an exchange with it.",
		"The latest message is for another participant, is small talk between people, or needs no reply.",
	)
	if err != nil {
		return false, false
	}
	req := decision.Request{State: gateState(msg, recent), Questions: map[string]decision.Question{"reply": q}}
	res := g.svc.Evaluate(ctx, req, decision.Options{AgentID: agentID.ID(), Purpose: "message-gate", Timeout: 10 * time.Second})
	if !res.OK() {
		dlog().Debug("Message gate unavailable", slog.String("reason", string(res.Reason)), slog.String("detail", res.Detail))
		return false, false
	}
	p := res.Answers["reply"].ProbabilityTrue
	dlog().Info("Message gate decision",
		slog.String("agent_id", agentID.String()), slog.Float64("p_reply", p), slog.Float64("threshold", g.threshold()),
		slog.Bool("shadow", shadow), slog.Bool("would_reply", p >= g.threshold()),
		slog.String("message", clipHead(msg.Content.Text, 200)),
		slog.String("provider", res.Provider), slog.String("model", res.Model))
	if shadow {
		return false, false
	}
	return p >= g.threshold(), true
}

func gateState(msg *message.Message, recent []*message.Message) string {
	var b strings.Builder
	b.WriteString("Channel transcript, oldest first:\n")
	for _, m := range recent {
		if m.ID == msg.ID {
			continue
		}
		fmt.Fprintf(&b, "[%s]: %s\n", m.AuthorID.ID(), strings.TrimSpace(m.Content.Text))
	}
	fmt.Fprintf(&b, "\nLatest message:\n[%s]: %s\n", msg.AuthorID.ID(), strings.TrimSpace(msg.Content.Text))
	return b.String()
}

// ---- HTTP -------------------------------------------------------------------

// evaluateWire is POST /api/decisions/evaluate, in OpenClaw's shape: a state,
// a map of questions (type, instructions, criteria), optional options.
// Criteria: choice takes a {label: description} object or a list of labels;
// score takes an ordered list of level descriptions; boolean takes an optional
// {"true": ..., "false": ...} object.
type evaluateWire struct {
	State     string                  `json:"state"`
	Questions map[string]questionWire `json:"questions"`
	Options   struct {
		AgentID   string `json:"agentId"`
		Purpose   string `json:"purpose"`
		TimeoutMs int    `json:"timeoutMs"`
	} `json:"options"`
}

type questionWire struct {
	Type         string          `json:"type"`
	Instructions string          `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria"`
}

func (w questionWire) toQuestion() (decision.Question, error) {
	kind, err := decision.ParseKind(w.Type)
	if err != nil {
		return decision.Question{}, err
	}
	switch kind {
	case decision.KindBoolean:
		var crit map[string]string
		if len(w.Criteria) > 0 {
			if err := json.Unmarshal(w.Criteria, &crit); err != nil {
				return decision.Question{}, fmt.Errorf("boolean criteria must be an object: %w", err)
			}
		}
		return decision.Boolean(w.Instructions, crit["true"], crit["false"])
	case decision.KindScore:
		var levels []string
		if err := json.Unmarshal(w.Criteria, &levels); err != nil {
			return decision.Question{}, fmt.Errorf("score criteria must be a list of levels: %w", err)
		}
		return decision.Score(w.Instructions, levels)
	}
	var labels []string
	if err := json.Unmarshal(w.Criteria, &labels); err == nil {
		return decision.Choice(w.Instructions, decision.Labels(labels...))
	}
	var described map[string]string
	if err := json.Unmarshal(w.Criteria, &described); err != nil {
		return decision.Question{}, fmt.Errorf("choice criteria must be a list of labels or a {label: description} object: %w", err)
	}
	keys := make([]string, 0, len(described))
	for k := range described {
		keys = append(keys, k)
	}
	sort.Strings(keys) // JSON objects carry no order; make the display order deterministic
	opts := make([]decision.Option, len(keys))
	for i, k := range keys {
		opts[i] = decision.Option{Label: k, Description: described[k]}
	}
	return decision.Choice(w.Instructions, opts)
}

// handleDecisionsEvaluate handles POST /api/decisions/evaluate. Any
// authenticated actor may ask; the answer is whatever the workspace's decision
// model says, or an unavailable status with its reason.
func (s *Server) handleDecisionsEvaluate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "POST only"}`, http.StatusMethodNotAllowed)
		return
	}
	if authorization.GetActorID(r.Context()) == "" {
		http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
		return
	}
	var in evaluateWire
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": %q}`, "invalid JSON: "+err.Error()), http.StatusBadRequest)
		return
	}
	req := decision.Request{State: in.State, Questions: map[string]decision.Question{}}
	for id, qw := range in.Questions {
		q, err := qw.toQuestion()
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": %q}`, fmt.Sprintf("question %q: %v", id, err)), http.StatusBadRequest)
			return
		}
		req.Questions[id] = q
	}
	if err := req.Validate(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), http.StatusBadRequest)
		return
	}
	if s.decisions == nil {
		_ = json.NewEncoder(w).Encode(tools.RenderDecisionResult(decision.Unavailable(decision.ReasonNotConfigured, "decision service not wired")))
		return
	}
	res := s.decisions.Evaluate(r.Context(), req, decision.Options{
		AgentID: in.Options.AgentID, Purpose: in.Options.Purpose, Timeout: time.Duration(in.Options.TimeoutMs) * time.Millisecond,
	})
	_ = json.NewEncoder(w).Encode(tools.RenderDecisionResult(res))
}

package providers

import (
	"context"
	"fmt"
	sharedctx "memdoor/pkg/shared/context"
	"strings"
	"sync/atomic"

	"memdoor/pkg/llm"
)

// RemoteEngine is the model chosen at start — a company's vendor key or the
// person's own OpenRouter key (model_engine.go). It names the provider and the agents'
// models; every request goes through that provider's client (factory.go).
type RemoteEngine struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"` // base URL; /v1/chat/completions appended if absent
	APIKey   string `json:"api_key,omitempty"`
	Model    string `json:"model"`
	// CtxLen is the window a turn is measured against when the model's own
	// is not known; sizing it from a smaller default compacted too early
	// (live: turn aborted "context window exceeded").
	CtxLen int `json:"ctx_len,omitempty"`
	// AgentModels: agents served by a model other than Model (e.g. coder →
	// its ladder's first rung).
	AgentModels map[string]string `json:"agent_models,omitempty"`
	// AgentLadders: each agent's ladder of models, tier 0 first, for the
	// agents with more than one rung. A session's tier picks the rung.
	AgentLadders map[string][]string `json:"agent_ladders,omitempty"`
	// Byok: Endpoint is OpenRouter itself and APIKey is the PERSON'S key
	// (byok.go): the ladder, the rung and the provider policy are decided
	// here, and nothing goes through memdoor.ai.
	Byok bool `json:"byok,omitempty"`
	// Vendor: Endpoint is an approved LLM vendor (or a company's AI gateway
	// in front of one) and APIKey the key the company gave the developer
	// (vendor.go): "anthropic", "openai" or "google". Nothing but that
	// endpoint is contacted — no OpenRouter, no catalogue.
	Vendor string `json:"vendor,omitempty"`
}

// ModelFromContext is the model pinned by id for the turn, "" when none.
func ModelFromContext(ctx context.Context) string {
	m, _ := ctx.Value(sharedctx.ModelKey).(string)
	return m
}

// ModelFor is the model that serves agent on the first rung.
func (re *RemoteEngine) ModelFor(agent string) string { return re.ModelForTier(agent, 0) }

// ModelForTier is the agent's model at a rung of its ladder: the rung when
// the agent has a ladder (a tier past the top is the top), else the agent's
// one model, else the engine's.
func (re *RemoteEngine) ModelForTier(agent string, tier int) string {
	name := strings.ToLower(agent)
	if ladder := re.AgentLadders[name]; tier > 0 && len(ladder) > 0 {
		if tier >= len(ladder) {
			tier = len(ladder) - 1
		}
		return ladder[tier]
	}
	if m := re.AgentModels[name]; m != "" {
		return m
	}
	return re.Model
}

// AnsweringModel is the model that answers agent's turn: the one the person
// pinned by id, else the agent's rung. On the person's own key a rung always
// resolves (byokDefaultModel).
// The client (factory.go), the window a turn is measured against and the
// request log all read it here, so they cannot name different models.
func (re *RemoteEngine) AnsweringModel(ctx context.Context, agent string) string {
	if m := ModelFromContext(ctx); m != "" {
		return m
	}
	if m := re.ModelForTier(agent, TierFromContext(ctx)); m != "" || !re.Byok {
		return m
	}
	return byokDefaultModel
}

// activeEngine is the engine chosen at start, held in memory: it is decided
// from the environment every time the gateway starts (model_engine.go), so a
// file only kept a copy of the key on disk and let a removed key outlive it.
var activeEngine atomic.Pointer[RemoteEngine]

// ActiveRemoteEngine returns a copy of the engine chosen at start, or nil
// when none was (a provider added with memdoor connect still answers:
// factory.go).
func ActiveRemoteEngine() *RemoteEngine {
	re := activeEngine.Load()
	if re == nil {
		return nil
	}
	c := *re
	return &c
}

// SetRemoteEngine activates an engine.
func SetRemoteEngine(re RemoteEngine) error {
	switch {
	case re.Model == "":
		return fmt.Errorf("remote engine needs a model")
	case re.Endpoint == "":
		return fmt.Errorf("remote engine needs an endpoint")
	}
	activeEngine.Store(&re)
	return nil
}

// ClearRemoteEngine forgets the active engine.
func ClearRemoteEngine() error {
	activeEngine.Store(nil)
	return nil
}

// ProseBreakerBytes is where a tag-free reply stops being "thinking" and
// starts being a runaway. Calibrated from live turns on a 27B: legitimate
// pre-call reasoning measured up to ~28KB; the runaways ran to the 16k-token
// cap at 64KB+. 48KB clears every healthy turn observed by a wide margin
// while still cutting a runaway four-plus minutes before the cap would.
const ProseBreakerBytes = 48 << 10

// ProseRunawayGuard is the rule the gateway plants for codebase agents: stop
// once the reply is past the threshold with no tool call started anywhere in
// it. A reply that HAS started a call is working — never cut, whatever its
// size (a big apply_patch is legitimate) — and neither is anything short.
func ProseRunawayGuard(byteCount int, sawToolCall bool) bool {
	return ProseRunawayGuardAt(ProseBreakerBytes)(byteCount, sawToolCall)
}

// ProseRunawayGuardAt is the prose breaker at a chosen threshold: a reply
// that has started a call is working and is never cut.
func ProseRunawayGuardAt(limit int) llm.StreamGuard {
	return func(byteCount int, sawToolCall bool) bool {
		return byteCount > limit && !sawToolCall
	}
}

// ctxString reads a context value as text ("" when absent); actor ids are
// a named string type, agent ids a plain string.
func ctxString(ctx context.Context, key sharedctx.ContextKey) string {
	switch v := ctx.Value(key).(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

// TierFromContext is the rung the turn asked for (sharedctx.TierKey), 0
// when none.
func TierFromContext(ctx context.Context) int {
	if t, ok := ctx.Value(sharedctx.TierKey).(int); ok && t > 0 {
		return t
	}
	return 0
}

// ServedModel is the model that will answer this turn: the one the person
// pinned, else the rung of the agent's ladder the turn is on. Used where a
// caller needs to NAME the model without building a client — the savings
// receipt prices what it saved at that model's price (pkg/savings).
func ServedModel(ctx context.Context, agent string) string {
	if m := ModelFromContext(ctx); m != "" {
		return m
	}
	re := ActiveRemoteEngine()
	if re == nil {
		return ""
	}
	return re.ModelForTier(agent, TierFromContext(ctx))
}

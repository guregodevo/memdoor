package providers

import (
	"context"
	"fmt"
)

// ClientFactory hands out the client for a turn. Every request goes through
// a provider (registry.go) — no exception (Greg, 2026-10-04: "all request for
// llm goes through provider"): a model pinned by id takes the provider that
// lists it, else the active engine's provider answers on the agent's rung,
// else the first connected provider.
type ClientFactory struct{}

func NewClientFactory() *ClientFactory {
	return &ClientFactory{}
}

// GetClientFor is the client for one agent's turn.
func (f *ClientFactory) GetClientFor(ctx context.Context, agent string) (LLMClient, error) {
	if m := ModelFromContext(ctx); m != "" {
		if p, model, ok := FindModel(ctx, m); ok {
			return p.Client(ctx, model.ID), nil
		}
	}
	if re := ActiveRemoteEngine(); re != nil {
		if p, ok := FindProvider(providerOfEngine(re)); ok && p.Connected() {
			return p.Client(ctx, re.AnsweringModel(ctx, agent)), nil
		}
	}
	if c, ok := connectedClient(ctx); ok {
		return c, nil
	}
	return nil, fmt.Errorf("no model is configured: set a provider key (OPEN_ROUTER_API_KEY, ANTHROPIC_API_KEY, …) or run memdoor connect")
}

// ModelName is the label a request carries before a provider names its
// model: the engine replaces it, and it keys the fallback window in
// gateway/context/limits.go.
const ModelName = "default"

// connectedClient answers a turn when no engine is active but a provider is
// connected (memdoor connect keeps it in ~/.memdoor/providers.json and sets
// no engine): the first connected chat provider on its first model. Without it a
// connect-only setup read "ready" and every turn failed (review, 2026-10-04).
func connectedClient(ctx context.Context) (LLMClient, bool) {
	if p, model, ok := connectedModel(ctx); ok {
		return p.Client(ctx, model), true
	}
	return nil, false
}

// connectedModel is the first connected chat provider and the first model it
// lists: what answers when no engine was chosen at start.
func connectedModel(ctx context.Context) (Provider, string, bool) {
	for _, p := range Providers() {
		if !p.Connected() || p.API == APIDecisions {
			continue
		}
		if list, err := ModelsOf(ctx, p); err == nil && len(list) > 0 {
			return p, list[0].ID, true
		}
	}
	return Provider{}, "", false
}

// ConnectedModel is the model connectedModel names, "" when nothing is
// connected: a turn with no start engine is measured against ITS window and
// output cap, never the 32K placeholder.
func ConnectedModel(ctx context.Context) string {
	_, m, _ := connectedModel(ctx)
	return m
}

package gateway

import (
	"context"
	"testing"

	"memdoor/gateway/providers"
)

func TestRouteViewAndPin(t *testing.T) {
	re := &providers.RemoteEngine{Model: "x", AgentLadders: map[string][]string{"coder": {"a", "b", "c"}}, AgentModels: map[string]string{"chief": "omni"}}
	sm := NewSessionManager()
	s, _ := sm.GetOrCreateSession("workspace:w:channel:c", "main")

	v := viewRoute(re, s, "coder")
	if v.Rung != 1 || v.Model != "a" || v.Pinned || v.Reason != "first rung · the default" || len(v.Rungs) != 3 {
		t.Fatalf("default: %+v", v)
	}
	if v := viewRoute(re, nil, "coder"); v.Model != "a" || v.Rung != 1 {
		t.Fatalf("no session yet is the first rung: %+v", v)
	}
	if v := viewRoute(re, nil, "chief"); len(v.Rungs) != 1 || v.Model != "omni" {
		t.Fatalf("an agent without a ladder has one rung: %+v", v)
	}
	if err := pinRoute(s, 3, 3); err != nil {
		t.Fatal(err)
	}
	v = viewRoute(re, s, "coder")
	if v.Rung != 3 || v.Model != "c" || !v.Pinned || v.Reason != reasonPinned || sessionTier(s) != 2 {
		t.Fatalf("pinned to the top: %+v", v)
	}
	if err := pinRoute(s, 4, 3); err == nil {
		t.Fatal("past the top is refused")
	}
	if err := pinRoute(s, 0, 3); err != nil {
		t.Fatal(err)
	}
	if v = viewRoute(re, s, "coder"); v.Rung != 1 || v.Pinned || v.Reason != "first rung · the default" {
		t.Fatalf("auto releases: %+v", v)
	}
	// A session raised by something else keeps a reason of its own.
	s.SetMetadata(sessionTierKey, 1)
	s.SetMetadata(sessionReasonKey, "escalated: 5 failing test runs")
	if v = viewRoute(re, s, "coder"); v.Model != "b" || v.Reason != "escalated: 5 failing test runs" {
		t.Fatalf("reason kept: %+v", v)
	}
}

// A message-path turn runs on a fresh Session; the adapter carries the
// registered session's rung onto it.
func TestAdapterCarriesTheRouteOntoTheTurnSession(t *testing.T) {
	sm := NewSessionManager()
	reg, _ := sm.GetOrCreateSession("workspace:w:channel:c", "main")
	_ = pinRoute(reg, 3, 4)
	a := &AgentExecutorAdapter{routes: sm}
	turn := &Session{ID: "workspace:w:channel:c", Metadata: map[string]interface{}{"channel_id": "c"}}
	a.carryRoute(turn)
	if sessionTier(turn) != 2 || !sessionPinned(turn) || sessionReason(turn) != reasonPinned {
		t.Fatalf("carried: tier=%d pinned=%v reason=%q", sessionTier(turn), sessionPinned(turn), sessionReason(turn))
	}
	other := &Session{ID: "workspace:w:channel:other", Metadata: map[string]interface{}{}}
	a.carryRoute(other)
	if sessionTier(other) != 0 {
		t.Fatal("another conversation is untouched")
	}
}

// Once the gateway holds a rung for a session — even the first, after a
// release — its word is exact.
func TestReleasedSessionIsStillRouted(t *testing.T) {
	sm := NewSessionManager()
	s, _ := sm.GetOrCreateSession("workspace:w:channel:c", "main")
	if sessionRouted(s) {
		t.Fatal("a fresh session has no rung of its own")
	}
	_ = pinRoute(s, 2, 4)
	_ = pinRoute(s, 0, 4)
	if !sessionRouted(s) || sessionPinned(s) || sessionTier(s) != 0 {
		t.Fatalf("released: routed=%v pinned=%v tier=%d", sessionRouted(s), sessionPinned(s), sessionTier(s))
	}
}

// A model pinned by id sits off the ladder: the view names it, the ladder
// stays visible, a rung pin or auto clears it.
func TestPinAModelById(t *testing.T) {
	re := &providers.RemoteEngine{Model: "x", AgentLadders: map[string][]string{"coder": {"a", "b", "c"}}}
	sm := NewSessionManager()
	s, _ := sm.GetOrCreateSession("workspace:w:channel:c", "main")
	if err := pinModel(s, "not a model!", "", ""); err == nil {
		t.Fatal("an id is letters, digits and a few marks")
	}
	if err := pinModel(s, "claude-haiku-4-5-20251001", "", ""); err != nil {
		t.Fatalf("a provider's own id, without a vendor/ prefix, pins: %v", err)
	}
	if err := pinModel(s, "z-ai/glm-5.3", "fastest", ""); err == nil {
		t.Fatal("the host preference is one of a few words")
	}
	if err := pinModel(s, "z-ai/glm-5.3", "throughput", "Baidu, morph"); err != nil {
		t.Fatal(err)
	}
	v := viewRoute(re, s, "coder")
	if !v.Custom || v.Model != "z-ai/glm-5.3" || v.Rung != 0 || !v.Pinned || len(v.Rungs) != 3 || sessionModel(s) != "z-ai/glm-5.3" || v.Sort != "throughput" || v.Order != "baidu,morph" {
		t.Fatalf("pinned by id with a host preference: %+v", v)
	}
	if err := pinRoute(s, 2, 3); err != nil {
		t.Fatal(err)
	}
	if v = viewRoute(re, s, "coder"); v.Custom || v.Model != "b" || sessionModel(s) != "" {
		t.Fatalf("a rung pin replaces the id pin: %+v", v)
	}
}

// The turn runs on a copy of the session; it reports the effort it runs at
// through the adapter's callback, straight to the registered session
// /api/route (the footer) reads — "effort auto" stayed on screen after a turn
// at high while it sat on the copy (live, 2026-09-30).
func TestTheTurnReportsItsEffortToTheRoute(t *testing.T) {
	sm := NewSessionManager()
	reg, _ := sm.GetOrCreateSession("workspace:w:channel:c", "main")
	a := &AgentExecutorAdapter{routes: sm}
	turn := &Session{ID: "workspace:w:channel:c", Metadata: map[string]interface{}{}}
	a.carryRoute(turn)
	ctx := withRouteSink(context.Background(), a.routeSink(turn))
	reportRoute(ctx, sessionEffortUsed, "high")
	reportRoute(ctx, sessionEffortFrom, "default")
	if v := viewRoute(nil, reg, "coder"); v.EffortUsed != "high" || v.EffortFrom != "default" {
		t.Fatalf("route after the report: %+v", v)
	}
	reportRoute(context.Background(), sessionEffortUsed, "low") // no sink: nothing, no panic
}

// The route view names the provider that serves the model: the active
// engine's, or the one that lists a pinned id (the footer shows it).
func TestRouteViewNamesTheProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, v := range []string{"AI_GATEWAY_BASE_URL", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "XAI_API_KEY", "BASETEN_API_KEY", "GROQ_API_KEY", "DEEPSEEK_API_KEY", "GROK_API_KEY", "MEMDOOR_BYOK"} {
		t.Setenv(v, "")
	}
	t.Setenv("OPEN_ROUTER_API_KEY", "sk-or-test")
	re, _ := providers.ByokEngine()
	t.Cleanup(func() { _ = providers.ClearRemoteEngine() })
	if err := providers.SetRemoteEngine(re); err != nil {
		t.Fatal(err)
	}
	old := providers.OpenRouterModels
	providers.OpenRouterModels = func(string) ([]providers.Model, error) { return []providers.Model{{ID: "z-ai/glm-5.3"}}, nil }
	defer func() { providers.OpenRouterModels = old }()
	providers.ForgetModels("openrouter")
	sm := NewSessionManager()
	s, _ := sm.GetOrCreateSession("workspace:w:channel:c", "main")
	if v := viewRoute(&re, s, "coder"); v.Provider != "openrouter" || len(v.RungProviders) != 3 || v.RungProviders[1] != "openrouter" {
		t.Fatalf("the active engine's provider, stated per rung: %+v", v)
	}
	if err := pinModel(s, "z-ai/glm-5.3", "", ""); err != nil {
		t.Fatal(err)
	}
	if v := viewRoute(&re, s, "coder"); v.Provider != "openrouter" || v.Model != "z-ai/glm-5.3" || !v.Custom {
		t.Fatalf("a pin names its provider: %+v", v)
	}
}

package gateway

import (
	"testing"
	"time"

	ctxmgmt "memdoor/gateway/context"
	"memdoor/gateway/providers"
)

// THE BELT MUST FIT THE BRAIN THAT IS ACTUALLY WEARING IT.
//
// context/limits.go prefers the active engine's serving window over the
// placeholder, through a probe that NOTHING registered — so every turn was
// measured against the 32K placeholder while the engine served 65K. Live 2026-09-16: "Context window exceeded: 32524 tokens >
// 28672 effective limit" killed a clipping run at half the window we pay
// for.
func TestTheContextLimitFollowsTheServingBrain(t *testing.T) {
	// A temp HOME first: setting and clearing an engine PERSISTS, and this
	// test must never touch the machine's real brain configuration.
	t.Setenv("HOME", t.TempDir())
	restore := providers.ActiveRemoteEngine()
	t.Cleanup(func() {
		if restore != nil {
			_ = providers.SetRemoteEngine(*restore)
		} else {
			_ = providers.ClearRemoteEngine()
		}
	})

	// Building a runtime is what registers the probe.
	if _, err := NewAgentRuntime("", false); err != nil {
		t.Skipf("runtime unavailable in this environment: %v", err)
	}

	// No engine and no model named: the placeholder answers.
	if err := providers.ClearRemoteEngine(); err != nil {
		t.Fatal(err)
	}
	local, err := ctxmgmt.GetModelLimits("default")
	if err != nil {
		t.Fatal(err)
	}
	if local.ContextWindow != 32_768 {
		t.Fatalf("with no engine, the placeholder must answer, got %d", local.ContextWindow)
	}

	// With a 65K brain serving, the limits must follow it.
	if err := providers.SetRemoteEngine(providers.RemoteEngine{
		Name: "test-brain", Endpoint: "http://127.0.0.1:1", Model: "m", CtxLen: 65_536,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := ctxmgmt.GetModelLimits("default")
	if err != nil {
		t.Fatal(err)
	}
	if got.ContextWindow != 65_536 {
		t.Fatalf("the window did not follow the brain: %d", got.ContextWindow)
	}
	if got.EffectiveLimit() <= local.EffectiveLimit() {
		t.Fatalf("the effective limit did not grow: %d vs local %d",
			got.EffectiveLimit(), local.EffectiveLimit())
	}
}

// On the person's own key the window is the answering model's own, from the
// catalogue; a model it does not list keeps the engine's (2026-09-29: every
// model, 128K to 1.3M, was measured against one fixed 262K).
func TestTheWindowIsTheAnsweringModels(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPEN_ROUTER_API_KEY", "sk-test")
	if _, err := NewAgentRuntime("", false); err != nil {
		t.Skipf("runtime unavailable in this environment: %v", err)
	}
	re, _ := providers.ByokEngine()
	if err := providers.SetRemoteEngine(re); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providers.ClearRemoteEngine() })
	theByokCatalog.mu.Lock()
	saved, savedAt := theByokCatalog.list, theByokCatalog.at
	theByokCatalog.list = []byokModel{{ID: "tiny/32k", Context: 32_768}, {ID: "small/128k", Context: 131_072}, {ID: "big/1m", Context: 1_048_576}}
	theByokCatalog.at = time.Now()
	theByokCatalog.mu.Unlock()
	t.Cleanup(func() {
		theByokCatalog.mu.Lock()
		theByokCatalog.list, theByokCatalog.at = saved, savedAt
		theByokCatalog.mu.Unlock()
	})

	for model, want := range map[string]int{"small/128k": 131_072, "big/1m": 1_048_576, "not/listed": re.CtxLen, "default": re.CtxLen} {
		got, err := ctxmgmt.GetModelLimits(model)
		if err != nil {
			t.Fatal(err)
		}
		if got.ContextWindow != want {
			t.Errorf("%s: window %d, want %d", model, got.ContextWindow, want)
		}
		// A prompt at the effective limit plus the reply it asks for must fit
		// the window, or the provider refuses the request.
		if out := maxOutputTokens(model); got.EffectiveLimit()+int(out) > got.ContextWindow {
			t.Errorf("%s: %d prompt + %d reply > %d window", model, got.EffectiveLimit(), out, got.ContextWindow)
		}
	}
	// Live 2026-09-29: a 32K model was asked for 16,384 tokens of reply.
	if out := maxOutputTokens("tiny/32k"); out != 8_192 {
		t.Errorf("a 32K model's reply cap is a quarter of it, got %d", out)
	}
	if esc := escalatedMaxOutputTokens("tiny/32k"); esc != 16_384 {
		t.Errorf("a 32K model's escalated cap is half of it, got %d", esc)
	}
	// Live 2026-09-29: a 1.31M model was retried at 655,360 tokens.
	if esc := escalatedMaxOutputTokens("big/1m"); esc != 2*maxOutputTokens("big/1m") {
		t.Errorf("a huge model's escalated cap is twice its normal cap, got %d (normal %d)", esc, maxOutputTokens("big/1m"))
	}
}

// The escalated retry of a cut-off reply asks for half the window; with a
// big prompt on a small model that overflowed the window, and the provider
// refused. The cap is fitted to the room the prompt leaves.
func TestAnEscalatedReplyFitsTheRoomLeft(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPEN_ROUTER_API_KEY", "sk-test")
	if _, err := NewAgentRuntime("", false); err != nil {
		t.Skipf("runtime unavailable in this environment: %v", err)
	}
	re, _ := providers.ByokEngine()
	if err := providers.SetRemoteEngine(re); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providers.ClearRemoteEngine() })
	theByokCatalog.mu.Lock()
	saved, savedAt := theByokCatalog.list, theByokCatalog.at
	theByokCatalog.list = []byokModel{{ID: "tiny/32k", Context: 32_768}}
	theByokCatalog.at = time.Now()
	theByokCatalog.mu.Unlock()
	t.Cleanup(func() {
		theByokCatalog.mu.Lock()
		theByokCatalog.list, theByokCatalog.at = saved, savedAt
		theByokCatalog.mu.Unlock()
	})

	esc := escalatedMaxOutputTokens("tiny/32k") // 16,384: half the window
	check := &ctxmgmt.CheckResult{TotalTokens: 20_000}
	if got := fitReplyToWindow(esc, "tiny/32k", check); got != 32_768-20_000 {
		t.Fatalf("a 20K prompt leaves 12,768 for the reply, got %d", got)
	}
	if got := fitReplyToWindow(8_192, "tiny/32k", check); got != 8_192 {
		t.Fatalf("a cap that fits is left alone, got %d", got)
	}
	if got := fitReplyToWindow(esc, "tiny/32k", nil); got != esc {
		t.Fatalf("no check, no change: %d", got)
	}
}

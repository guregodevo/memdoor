package gateway

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"memdoor/gateway/providers"
)

// keptKeyGateway is a HOME whose only OpenRouter key is the one `memdoor
// connect` kept (a sign-in, a key found on the machine), with the tier
// question and the catalogue answered by the test.
func keptKeyGateway(t *testing.T, free, known bool, catalog []providers.Model, catalogErr error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	for _, v := range []string{"OPEN_ROUTER_API_KEY", "OPENROUTER_API_KEY", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY",
		"XAI_API_KEY", "GROK_API_KEY", "BASETEN_API_KEY", "GROQ_API_KEY", "DEEPSEEK_API_KEY", "AI_GATEWAY_BASE_URL", "AI_GATEWAY_TOKEN",
		"MEMDOOR_MODEL", "MEMDOOR_VENDOR"} {
		t.Setenv(v, "")
	}
	keepOpenRouterKey(t, "sk-or-v1-kept")
	savedTier, savedCatalog := openRouterFreeTier, openRouterCatalog
	openRouterFreeTier = func(context.Context, string) (bool, bool) { return free, known }
	openRouterCatalog = func(string) ([]providers.Model, error) { return catalog, catalogErr }
	t.Cleanup(func() {
		openRouterFreeTier, openRouterCatalog = savedTier, savedCatalog
		_ = providers.ClearRemoteEngine()
	})
	_ = providers.ClearRemoteEngine()
}

func keepOpenRouterKey(t *testing.T, key string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".memdoor")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{"providers":[]}`
	if key != "" {
		body = `{"providers":[{"id":"openrouter","name":"OpenRouter","api":"openrouter","base":"https://openrouter.ai/api/v1","key":"` + key + `"}]}`
	}
	if err := os.WriteFile(filepath.Join(dir, "providers.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

var freeCatalog = []providers.Model{
	{ID: "z-ai/glm-5.3-flash", Context: 1_000_000, Tools: true},
	{ID: "q/coder:free", Context: 262_144, Tools: true},
}

// A kept key gets the OpenRouter engine and its paid ladder, as an exported
// one does (before 2026-10-31 it answered on OpenRouter's first-listed model).
func TestAKeptKeyChoosesTheOpenRouterEngine(t *testing.T) {
	keptKeyGateway(t, false, true, freeCatalog, nil)
	(&Server{}).selectModelEngine()
	re := providers.ActiveRemoteEngine()
	if re == nil || !re.Byok || re.APIKey != "sk-or-v1-kept" || re.FreeTier {
		t.Fatalf("want the paid OpenRouter engine on the kept key, got %+v", re)
	}
	if m := re.ModelForTier("coder", 0); m == "" || m == "q/coder:free" {
		t.Fatalf("the paid ladder's first rung, got %q", m)
	}
}

// A free account answers on a free model with tools, from the catalogue.
func TestAFreeAccountRunsOnAFreeModel(t *testing.T) {
	keptKeyGateway(t, true, true, freeCatalog, nil)
	(&Server{}).selectModelEngine()
	re := providers.ActiveRemoteEngine()
	if re == nil || !re.FreeTier || re.ModelForTier("coder", 0) != "q/coder:free" || re.AnsweringModel(context.Background(), "coder") != "q/coder:free" {
		t.Fatalf("want every turn on q/coder:free, got %+v", re)
	}
}

// Negative paths: a tier nobody could read, or a free account whose
// catalogue cannot be read or holds no free model with tools, keep the paid
// ladder: never a model that may train on the code by default.
func TestTheFreeModelNeedsAClearAnswer(t *testing.T) {
	cases := map[string]struct {
		free, known bool
		catalog     []providers.Model
		err         error
	}{
		"tier unknown":         {free: true, known: false, catalog: freeCatalog},
		"catalogue unreadable": {free: true, known: true, err: errors.New("offline")},
		"no free model":        {free: true, known: true, catalog: []providers.Model{{ID: "z-ai/glm-5.3-flash", Tools: true}, {ID: "x/notools:free"}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			keptKeyGateway(t, c.free, c.known, c.catalog, c.err)
			(&Server{}).selectModelEngine()
			re := providers.ActiveRemoteEngine()
			if re == nil || !re.Byok || re.FreeTier || re.ModelForTier("coder", 0) == "" {
				t.Fatalf("want the paid ladder, got %+v", re)
			}
		})
	}
}

// The key removed, its engine goes; credit added, the next choice leaves the
// free model.
func TestTheEngineFollowsTheKeptKey(t *testing.T) {
	keptKeyGateway(t, true, true, freeCatalog, nil)
	s := &Server{}
	s.selectModelEngine()
	if re := providers.ActiveRemoteEngine(); re == nil || !re.FreeTier {
		t.Fatalf("free engine first: %+v", re)
	}
	openRouterFreeTier = func(context.Context, string) (bool, bool) { return false, true }
	s.selectModelEngine()
	if re := providers.ActiveRemoteEngine(); re == nil || re.FreeTier {
		t.Fatalf("credit added: the paid ladder, got %+v", re)
	}
	keepOpenRouterKey(t, "")
	s.selectModelEngine()
	if re := providers.ActiveRemoteEngine(); re != nil {
		t.Fatalf("the key removed: no engine, got %+v", re)
	}
}

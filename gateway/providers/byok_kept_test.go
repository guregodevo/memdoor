package providers

import (
	"os"
	"path/filepath"
	"testing"

	"memdoor/pkg/shared"
)

// noModelEnv clears every variable a key or an engine is read from, so the
// shell running the test cannot decide it.
func noModelEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	for _, v := range []string{"OPEN_ROUTER_API_KEY", "OPENROUTER_API_KEY", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY",
		"XAI_API_KEY", "GROK_API_KEY", "BASETEN_API_KEY", "GROQ_API_KEY", "DEEPSEEK_API_KEY", "AI_GATEWAY_BASE_URL", "AI_GATEWAY_TOKEN",
		"MEMDOOR_MODEL", "MEMDOOR_VENDOR", "KEPT_BY_NAME"} {
		t.Setenv(v, "")
	}
}

func writeProviders(t *testing.T, body string) {
	t.Helper()
	path := shared.MemdoorHome(providersFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The OpenRouter key is the environment's, else the one `memdoor connect`
// kept; anything else is no key (the mutation check: drop the keptKey
// fallback in byokKey and the kept case finds "").
func TestByokKeyEnvironmentThenKept(t *testing.T) {
	noModelEnv(t)
	if k, s := ByokKey(), ByokKeySource(); k != "" || s != "" {
		t.Fatalf("no file, no env: got %q %q", k, s)
	}

	writeProviders(t, `{"providers":[{"id":"openrouter","api":"openrouter","base":"https://openrouter.ai/api/v1","key":"sk-or-v1-kept"}]}`)
	if k, s := ByokKey(), ByokKeySource(); k != "sk-or-v1-kept" || s != "providers.json" {
		t.Fatalf("kept: got %q %q", k, s)
	}
	if ByokKeyFromEnv() {
		t.Fatal("a kept key is not the environment's")
	}
	if re, ok := ByokEngine(); !ok || !re.Byok || re.APIKey != "sk-or-v1-kept" || re.ModelForTier("coder", 0) == "" {
		t.Fatalf("a kept key gets the OpenRouter engine and its ladder: %+v %v", re, ok)
	}

	t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-env")
	if k, s := ByokKey(), ByokKeySource(); k != "sk-or-v1-env" || s != "env OPENROUTER_API_KEY" || !ByokKeyFromEnv() {
		t.Fatalf("the environment wins: got %q %q", k, s)
	}
}

// Negative paths: a kept name that is not set, a keyless or another
// provider's entry, a broken file, an empty key: no key, never a panic.
func TestByokKeyNegativePaths(t *testing.T) {
	cases := map[string]string{
		"a name not set":        `{"providers":[{"id":"openrouter","key":"KEPT_BY_NAME"}]}`,
		"keyless":               `{"providers":[{"id":"openrouter","keyless":true,"key":"sk-or-v1-x"}]}`,
		"another provider only": `{"providers":[{"id":"anthropic","key":"sk-ant-x"}]}`,
		"broken json":           `{"providers":[{"id":"openrouter","key":`,
		"empty key":             `{"providers":[{"id":"openrouter","key":"  "}]}`,
		"empty file":            ``,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			noModelEnv(t)
			writeProviders(t, body)
			if k := ByokKey(); k != "" {
				t.Fatalf("want no key, got %q", k)
			}
			if _, ok := ByokEngine(); ok {
				t.Fatal("no key, no engine")
			}
		})
	}
	t.Run("a kept name that is set", func(t *testing.T) {
		noModelEnv(t)
		t.Setenv("KEPT_BY_NAME", "sk-or-v1-by-name")
		writeProviders(t, `{"providers":[{"id":"openrouter","key":"KEPT_BY_NAME"}]}`)
		if k, s := ByokKey(), ByokKeySource(); k != "sk-or-v1-by-name" || s != "env KEPT_BY_NAME" || ByokKeyFromEnv() {
			t.Fatalf("got %q %q", k, s)
		}
	})
}

// The registry's OpenRouter entry says where its key came from.
func TestOpenRouterProviderNamesItsKeySource(t *testing.T) {
	noModelEnv(t)
	writeProviders(t, `{"providers":[{"id":"openrouter","api":"openrouter","base":"https://openrouter.ai/api/v1","key":"sk-or-v1-kept"}]}`)
	p, ok := FindProvider(OpenRouterID)
	if !ok || !p.Connected() || p.Key != "sk-or-v1-kept" {
		t.Fatalf("the kept key connects OpenRouter: %+v", p)
	}
	if p.KeySource == "env "+OpenRouterKeyVar {
		t.Fatalf("a kept key is not labelled as the environment's: %q", p.KeySource)
	}
}

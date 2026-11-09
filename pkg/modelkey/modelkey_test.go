package modelkey

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Fake keys, built so no scanner takes them for real ones.
var (
	fakeOpenRouter = "sk-or-v1-" + strings.Repeat("a1", 32)
	fakeAnthropic  = "sk-ant-api03-" + strings.Repeat("Ab", 40)
	fakeOAuth      = "sk-ant-oat01-" + strings.Repeat("Ab", 40)
	fakeOpenAI     = "sk-proj-" + strings.Repeat("Ab", 40)
	fakeGemini     = "AIza" + strings.Repeat("x", 35)
	fakeGroq       = "gsk_" + strings.Repeat("Ab", 26)
	fakeXAI        = "xai-" + strings.Repeat("Ab", 40)
	fakeDeepSeek   = "sk-" + strings.Repeat("0f", 16)
	fakeOldOpenAI  = "sk-" + strings.Repeat("Ab", 24)
	fakeLLMGateway = "llmgtwy_" + strings.Repeat("Lg", 20)
)

func TestProvidersOf(t *testing.T) {
	cases := map[string][]Provider{
		fakeOpenRouter:       {ProviderOpenRouter},
		fakeAnthropic:        {ProviderAnthropic},
		fakeOpenAI:           {ProviderOpenAI},
		fakeGemini:           {ProviderGemini},
		fakeGroq:             {ProviderGroq},
		fakeXAI:              {ProviderXAI},
		fakeLLMGateway:       {ProviderLLMGateway},
		fakeDeepSeek:         {ProviderDeepSeek, ProviderOpenAI},
		fakeOldOpenAI:        {ProviderOpenAI, ProviderDeepSeek},
		" " + fakeGroq + " ": {ProviderGroq},
		// Not a model key: a Claude subscription's OAuth token, a name, a
		// path, a short sk-, nothing.
		fakeOAuth:            nil,
		"OPENROUTER_API_KEY": nil,
		"/usr/local/bin":     nil,
		"sk-short":           nil,
		"":                   nil,
	}
	for value, want := range cases {
		if got := ProvidersOf(value); !reflect.DeepEqual(got, want) {
			t.Errorf("ProvidersOf(%.20q…) = %v, want %v", value, got, want)
		}
	}
}

func TestFindByValueNotName(t *testing.T) {
	dir := t.TempDir()
	env := []string{
		"PATH=/usr/bin",
		"MY_OR_KEY=" + fakeOpenRouter, // a name Memdoor never reads
		"OPEN_ROUTER_API_KEY=" + fakeOpenRouter,
		"CLAUDE_CODE_OAUTH_TOKEN=" + fakeOAuth,
	}
	dotenv := "# keys\nexport ANTHROPIC_API_KEY=\"" + fakeAnthropic + "\"\nGEMINI='" + fakeGemini + "' # mine\nNOT A=LINE\nOPENROUTER_API_KEY=" + fakeOpenRouter + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(dotenv), 0o600); err != nil {
		t.Fatal(err)
	}
	got := Find(env, dir)
	var where []string
	for _, k := range got {
		where = append(where, string(k.Providers[0])+" "+k.Where())
	}
	want := []string{"openrouter $MY_OR_KEY", "anthropic .env: ANTHROPIC_API_KEY", "gemini .env: GEMINI"}
	if !reflect.DeepEqual(where, want) {
		t.Fatalf("found %v, want %v (each value once, the environment first)", where, want)
	}
	if !got[0].InEnvironment() || got[1].InEnvironment() {
		t.Error("InEnvironment: true for the shell's, false for a file's")
	}
	if got[1].Value != fakeAnthropic || got[2].Value != fakeGemini {
		t.Error("quotes and a trailing comment are not part of the value")
	}
}

func TestFindNoDir(t *testing.T) {
	if got := Find([]string{"A=b"}, ""); got != nil {
		t.Fatalf("nothing to find, got %v", got)
	}
}

// Every sk-ant- kind but a subscription's OAuth token and an admin key is
// Anthropic's, and Google's newer AQ. keys are Gemini's (live 2026-10-31: an
// sk-ant-usr- and an AQ. key in .envrc were not found; the mutation check:
// the old ^sk-ant-api[0-9]{2}- shape misses the first).
func TestTheNewerKeyShapes(t *testing.T) {
	body := strings.Repeat("aB3_x-", 8)
	for value, want := range map[string]Provider{
		"sk-ant-usr-" + body:             ProviderAnthropic,
		"sk-ant-api03-" + body:           ProviderAnthropic,
		"AQ." + body:                     ProviderGemini,
		"AIza" + strings.Repeat("a", 35): ProviderGemini,
	} {
		if ps := ProvidersOf(value); len(ps) == 0 || ps[0] != want {
			t.Errorf("%.14s…: %v, want %s", value, ps, want)
		}
	}
	for _, value := range []string{"sk-ant-oat01-" + body, "sk-ant-admin01-" + body, "AQ.short", "sk-ant-usr-short"} {
		if ps := ProvidersOf(value); ps != nil {
			t.Errorf("%.16s… is no model key, got %v", value, ps)
		}
	}
}

// A variable's name breaks a tie between providers one shape fits, and finds
// a key with no shape of its own under that provider's own name; under any
// other name, or holding a placeholder, it finds nothing (the mutation check:
// ignore the name and DEEPSEEK_API_KEY reads "DeepSeek or OpenAI" unnamed).
func TestTheNameBreaksATieAndFindsAShapelessKey(t *testing.T) {
	hex := "sk-" + strings.Repeat("0a", 16)
	baseten := "abcd1234." + strings.Repeat("Q", 32)
	found := Find([]string{
		"DEEPSEEK_API_KEY=" + hex,
		"OPENAI_KEY=" + strings.Replace(hex, "0a", "1b", 1),
		"MY_KEY=" + strings.Replace(hex, "0a", "2c", 1),
		"BASETEN_API_KEY=" + baseten,
		"EUROUTER_API_KEY=eur_" + strings.Repeat("e", 20) + "." + strings.Repeat("F", 20),
		"TYPESAFE_API_KEY=your-key-here",
		"SOME_OTHER_KEY=" + strings.Replace(baseten, "Q", "R", 1),
	}, "")
	got := map[string]Found{}
	for _, k := range found {
		got[k.Name] = k
	}
	if k := got["DEEPSEEK_API_KEY"]; !k.Named || k.Providers[0] != ProviderDeepSeek || len(k.Providers) != 2 {
		t.Errorf("DEEPSEEK_API_KEY: %+v", k)
	}
	if k := got["OPENAI_KEY"]; !k.Named || k.Providers[0] != ProviderOpenAI {
		t.Errorf("OPENAI_KEY: %+v", k)
	}
	if k := got["MY_KEY"]; k.Named || k.Providers[0] != ProviderDeepSeek {
		t.Errorf("a name that says nothing leaves the tie: %+v", k)
	}
	if k := got["BASETEN_API_KEY"]; !k.Named || len(k.Providers) != 1 || k.Providers[0] != ProviderBaseten {
		t.Errorf("BASETEN_API_KEY: %+v", k)
	}
	if k := got["EUROUTER_API_KEY"]; !k.Named || len(k.Providers) != 1 || k.Providers[0] != ProviderEURouter {
		t.Errorf("EUROUTER_API_KEY: %+v", k)
	}
	for _, name := range []string{"TYPESAFE_API_KEY", "SOME_OTHER_KEY"} {
		if k, ok := got[name]; ok {
			t.Errorf("%s is no key here: %+v", name, k)
		}
	}
}

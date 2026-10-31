package gateway

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/gateway/providers"
)

// planGateway is keptKeyGateway (a kept OpenRouter key, a paid account) with
// a ChatGPT plan that is signed in or not, as the test says.
func planGateway(t *testing.T, signedIn *bool) {
	t.Helper()
	keptKeyGateway(t, false, true, freeCatalog, nil)
	saved := chatGPTPlanEngine
	chatGPTPlanEngine = func(context.Context) (providers.RemoteEngine, bool) {
		if !*signedIn {
			return providers.RemoteEngine{}, false
		}
		return providers.RemoteEngine{Name: providers.VendorEngineName, Vendor: providers.VendorChatGPT, Endpoint: "https://api.openai.com/v1", APIKey: "plan-token", Model: "gpt-6.1-sol", CtxLen: 272_000}, true
	}
	t.Cleanup(func() { chatGPTPlanEngine = saved })
}

// Signed in with ChatGPT, the plan answers before the OpenRouter key (Greg,
// 2026-10-31: "if you login to chatgpt you should get chatgpt model by
// default"); signed out, the key answers again (the mutation check: drop the
// plan's step and the key answers while the plan is signed in).
func TestASignedInPlanAnswersBeforeTheOpenRouterKey(t *testing.T) {
	signedIn := true
	planGateway(t, &signedIn)
	(&Server{}).selectModelEngine()
	if re := providers.ActiveRemoteEngine(); re == nil || !re.IsChatGPTPlan() || re.Model != "gpt-6.1-sol" {
		t.Fatalf("the plan answers: %+v", re)
	}
	signedIn = false
	(&Server{}).selectModelEngine()
	if re := providers.ActiveRemoteEngine(); re == nil || !re.Byok {
		t.Fatalf("signed out, the OpenRouter key answers again: %+v", re)
	}
}

// Signed out with no OpenRouter key either, nothing keeps answering on the
// plan (the mutation check: clear only a Byok engine and the plan's stays).
func TestASignedOutPlanLeavesNoEngine(t *testing.T) {
	signedIn := true
	planGateway(t, &signedIn)
	keepOpenRouterKey(t, "")
	(&Server{}).selectModelEngine()
	if re := providers.ActiveRemoteEngine(); re == nil || !re.IsChatGPTPlan() {
		t.Fatalf("the plan answers alone: %+v", re)
	}
	signedIn = false
	(&Server{}).selectModelEngine()
	if re := providers.ActiveRemoteEngine(); re != nil {
		t.Fatalf("no plan and no key: no engine, got %+v", re)
	}
}

// A company's vendor key still comes first: that is policy, not a
// preference.
func TestAVendorKeyAnswersBeforeThePlan(t *testing.T) {
	signedIn := true
	planGateway(t, &signedIn)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-company")
	(&Server{}).selectModelEngine()
	if re := providers.ActiveRemoteEngine(); re == nil || re.Vendor != providers.VendorAnthropic {
		t.Fatalf("the company's key answers: %+v", re)
	}
}

// A pin kept on the OpenAI key for an id the plan lists moves to the plan;
// one the plan does not list, a bare one and a plan's own stay (the mutation
// check: move every openai: pin and gpt-4.1-mini moves too).
func TestPinsOnTheKeyMoveToThePlan(t *testing.T) {
	dir := t.TempDir()
	saved := persistentPinPath
	persistentPinPath = func() string { return filepath.Join(dir, "model.json") }
	t.Cleanup(func() { persistentPinPath = saved })
	for actor, model := range map[string]string{
		"human:a": "openai:gpt-6-astra",
		"human:b": "openai:gpt-4.1-mini",
		"human:c": "gpt-6-astra",
		"human:d": "chatgpt:gpt-6-sol",
	} {
		if err := writePersistentPin(actor, &persistentPin{Model: model, Sort: "price"}); err != nil {
			t.Fatal(err)
		}
	}
	moved := pinsToPlan([]providers.Model{{ID: "gpt-6-astra"}, {ID: "gpt-6-sol"}})
	if len(moved) != 1 || moved[0] != "openai:gpt-6-astra → chatgpt:gpt-6-astra" {
		t.Fatalf("moved %q", moved)
	}
	want := map[string]string{"human:a": "chatgpt:gpt-6-astra", "human:b": "openai:gpt-4.1-mini", "human:c": "gpt-6-astra", "human:d": "chatgpt:gpt-6-sol"}
	for actor, model := range want {
		p := readPersistentPin(actor)
		if p == nil || p.Model != model {
			t.Fatalf("%s: %+v, want %s", actor, p, model)
		}
	}
	if p := readPersistentPin("human:a"); p.Sort != "price" {
		t.Fatalf("a moved pin keeps its host preference: %+v", p)
	}
	if moved := pinsToPlan(nil); len(moved) != 0 || !strings.HasPrefix(readPersistentPin("human:b").Model, "openai:") {
		t.Fatalf("an empty plan moves nothing: %q", moved)
	}
}

package providers

import (
	"context"
	sharedctx "memdoor/pkg/shared/context"
	"net/http"
	"strings"
	"testing"
)

// A company's key (Greg, 2026-10-02: the policy allows an approved vendor on a
// key the developer creates — Anthropic, Google, OpenAI — or the company AI
// gateway in front of one; the default provider must be off).
func TestVendorEngineFromTheEnvironment(t *testing.T) {
	for _, v := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "XAI_API_KEY", "BASETEN_API_KEY", "GROQ_API_KEY", "DEEPSEEK_API_KEY", "GROK_API_KEY", "MEMDOOR_MODEL", "MEMDOOR_VENDOR", "MEMDOOR_MODEL_CONTEXT", "ANTHROPIC_BASE_URL", "OPENAI_BASE_URL", "GEMINI_BASE_URL"} {
		t.Setenv(v, "")
	}
	if _, ok, reason := VendorEngine(); ok || reason != "" {
		t.Fatalf("no key, no engine: ok=%v reason=%q", ok, reason)
	}

	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-company")
	re, ok, _ := VendorEngine()
	if !ok || re.Vendor != VendorAnthropic || re.Model != anthropicDefaultModel || re.Endpoint != anthropicDefaultBase || re.CtxLen != anthropicDefaultContext || re.Name != VendorEngineName {
		t.Fatalf("anthropic engine: %+v", re)
	}
	if re.ModelFor("coder") != anthropicDefaultModel || re.AnsweringModel(context.Background(), "coder") != anthropicDefaultModel {
		t.Fatalf("the coder answers on the key's model, got %q", re.ModelFor("coder"))
	}
	if _, isAnthropic := newVendorClient(context.Background(), &re, re.Model).(*anthropicClient); !isAnthropic {
		t.Fatal("an Anthropic key gets Anthropic's own client")
	}

	// The company AI gateway in front of the vendor, and a pinned model.
	t.Setenv("ANTHROPIC_BASE_URL", "https://ai-gateway.corp.example/anthropic/")
	t.Setenv("MEMDOOR_MODEL", "claude-opus-5-5")
	t.Setenv("MEMDOOR_MODEL_CONTEXT", "500000")
	re, _, _ = VendorEngine()
	if re.Endpoint != "https://ai-gateway.corp.example/anthropic" || re.Model != "claude-opus-5-5" || re.CtxLen != 500000 {
		t.Fatalf("gateway + pinned model: %+v", re)
	}

	// OpenAI and Google need the model named: their ids change too often to guess.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("MEMDOOR_MODEL", "")
	t.Setenv("OPENAI_API_KEY", "sk-company")
	if _, ok, reason := VendorEngine(); ok || reason == "" {
		t.Fatalf("OpenAI without MEMDOOR_MODEL must say why: ok=%v reason=%q", ok, reason)
	}
	t.Setenv("MEMDOOR_MODEL", "gpt-5")
	re, ok, _ = VendorEngine()
	if !ok || re.Vendor != VendorOpenAI || re.Endpoint != openaiDefaultBase || re.Model != "gpt-5" {
		t.Fatalf("openai engine: %+v", re)
	}
	if c, isOAI := newVendorClient(context.Background(), &re, re.Model).(*oaiClient); !isOAI || c.attribute || c.provider != nil || c.baseURL != openaiDefaultBase {
		t.Fatalf("an OpenAI key gets the plain OpenAI-shaped client, no routing policy, no attribution: %+v", c)
	}

	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "AIza-company")
	t.Setenv("MEMDOOR_MODEL", "gemini-3-pro")
	re, ok, _ = VendorEngine()
	if !ok || re.Vendor != VendorGoogle || re.Endpoint != googleDefaultBase {
		t.Fatalf("google engine: %+v", re)
	}

	// MEMDOOR_VENDOR=off ignores the keys (a developer who wants OpenRouter).
	t.Setenv("MEMDOOR_VENDOR", "off")
	if _, ok, _ := VendorEngine(); ok {
		t.Fatal("MEMDOOR_VENDOR=off must ignore the vendor keys")
	}
}

// VendorMode is read from the active engine, so the whole gateway — the
// decision model, the catalogue, the lease — agrees on what may be contacted.
func TestVendorModeIsTheActiveEngine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if VendorMode() {
		t.Fatal("no engine, no vendor mode")
	}
	if err := SetRemoteEngine(vendorEngine(VendorAnthropic, anthropicDefaultBase, "k", anthropicDefaultModel, 1000)); err != nil {
		t.Fatal(err)
	}
	if !VendorMode() {
		t.Fatal("a vendor engine set means vendor mode")
	}
	if err := SetRemoteEngine(RemoteEngine{Name: ByokEngineName, Endpoint: byokEndpoint, APIKey: "k", Model: byokDefaultModel, Byok: true}); err != nil {
		t.Fatal(err)
	}
	if VendorMode() {
		t.Fatal("an OpenRouter engine is not vendor mode")
	}
}

// The company's own AI gateway (Greg, 2026-10-02): a base URL, a personal or
// service-account token named after the company, the provider slug as the
// model, the Responses API unless told chat — and it comes before any vendor
// key in the same environment.
func TestTheCompanysOwnGatewayComesFirst(t *testing.T) {
	for _, v := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "XAI_API_KEY", "BASETEN_API_KEY", "GROQ_API_KEY", "DEEPSEEK_API_KEY", "GROK_API_KEY", "MEMDOOR_MODEL", "MEMDOOR_VENDOR", "MEMDOOR_MODEL_CONTEXT", "AI_GATEWAY_BASE_URL", "AI_GATEWAY_TOKEN", "AI_GATEWAY_API", "ACME_AI_GATEWAY_TOKEN"} {
		t.Setenv(v, "")
	}
	t.Setenv("AI_GATEWAY_BASE_URL", "https://ai-gateway.corp.example/ai")
	if _, ok, reason := VendorEngine(); ok || !strings.Contains(reason, "token") {
		t.Fatalf("a base without a token says so: ok=%v %q", ok, reason)
	}
	t.Setenv("ACME_AI_GATEWAY_TOKEN", "pat-123")
	if _, ok, reason := VendorEngine(); ok || !strings.Contains(reason, "MEMDOOR_MODEL") {
		t.Fatalf("a gateway without a slug says so: ok=%v %q", ok, reason)
	}
	t.Setenv("MEMDOOR_MODEL", "mytestprovider-slug")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-also-set")
	re, ok, _ := VendorEngine()
	if !ok || re.Vendor != VendorGateway || re.APIKey != "pat-123" || re.Model != "mytestprovider-slug" || re.Endpoint != "https://ai-gateway.corp.example/ai" {
		t.Fatalf("gateway engine, before the vendor key: %+v", re)
	}
	c, isResponses := newVendorClient(context.Background(), &re, re.Model).(*responsesClient)
	if !isResponses || c.url != "https://ai-gateway.corp.example/ai/v1/responses" || c.token != "pat-123" {
		t.Fatalf("the Responses client at <base>/v1/responses: %+v", c)
	}
	t.Setenv("AI_GATEWAY_API", "chat")
	re, _, _ = VendorEngine()
	if _, isChat := newVendorClient(context.Background(), &re, re.Model).(*oaiClient); re.Vendor != VendorGatewayChat || !isChat {
		t.Fatalf("AI_GATEWAY_API=chat is the chat/completions client: %+v", re)
	}
	for base, want := range map[string]string{"https://g/ai": "https://g/ai/v1/responses", "https://g/ai/v1/": "https://g/ai/v1/responses", "https://g/ai/v1/responses": "https://g/ai/v1/responses"} {
		if got := responsesURL(base); got != want {
			t.Errorf("responsesURL(%q) = %q, want %q", base, got, want)
		}
	}
}

// Attribution: the preset's user and team under the gateway's header
// names, the turn's session and project from the context; an unset field
// sends no header; the preset wins over the turn.
func TestAttributionHeaders(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("MEMDOOR_ATTRIBUTION", "user=me@corp.example; team=data-eng")
	t.Setenv("MEMDOOR_ATTRIBUTION_HEADERS", "user=X-Email;team=X-Team")
	old := attributionFunc
	SetAttributionFunc(func(ctx context.Context) map[string]string {
		return map[string]string{AttrProject: "aktapus", AttrUser: "someone-else"}
	})
	defer func() { attributionFunc = old }()
	ctx := context.WithValue(context.Background(), sharedctx.SessionIDKey, "sess-1")
	h := http.Header{}
	setAttributionHeaders(ctx, h)
	if h.Get("X-Email") != "me@corp.example" || h.Get("X-Team") != "data-eng" || h.Get("X-Memdoor-Project") != "aktapus" || h.Get("X-Memdoor-Session") != "sess-1" {
		t.Fatalf("headers: %v", h)
	}
	if h.Get("X-Memdoor-User") != "" {
		t.Fatalf("the user's header is the gateway's name only: %v", h)
	}
	t.Setenv("MEMDOOR_ATTRIBUTION", "")
	h = http.Header{}
	setAttributionHeaders(context.Background(), h)
	if h.Get("X-Email") != "someone-else" || h.Get("X-Team") != "" || h.Get("X-Memdoor-Session") != "" {
		t.Fatalf("no preset: the turn's user under the gateway's name, no team, no session: %v", h)
	}
}

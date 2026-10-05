package providers

import (
	"context"
	"net/http"
	"os"
	"strings"
)

// A COMPANY'S OWN KEY (Greg, 2026-10-02: "I want to use memdoor but it's not
// possible with my telemetry company that checks everything … Claude Code,
// Cursor, Warp, Codex are used … the company gives developers an API key").
//
// A monitored company approves VENDORS, and once approved, that vendor's
// endpoint is where code may go. So this engine sends every turn to the
// vendor whose key the developer was given — Anthropic's Messages API or any
// OpenAI-compatible endpoint — and nothing else leaves the machine: no
// OpenRouter, no memdoor.ai broker (decisions stay off unless the gateway
// holds a decision key of its own), no catalogue fetch, no web search. The
// data path is the one the company already reviewed for Claude Code or
// Codex. docs/SECURITY.md lists every host the binary can reach, per mode.
//
//	AI_GATEWAY_BASE_URL=https://ai-gateway.corp.example/ai   the company's own
//	AI_GATEWAY_TOKEN=…            a personal or service-account token (any
//	                              variable ending in _AI_GATEWAY_TOKEN counts)
//	AI_GATEWAY_API=responses|chat the shape it speaks (default responses:
//	                              POST <base>/v1/responses, model = provider slug)
//	ANTHROPIC_API_KEY=…           [ANTHROPIC_BASE_URL=https://api.anthropic.com]
//	OPENAI_API_KEY=…              [OPENAI_BASE_URL=https://api.openai.com/v1]
//	GEMINI_API_KEY=…              [GEMINI_BASE_URL=…/v1beta/openai]
//	XAI_API_KEY=… (or GROK_API_KEY) [XAI_BASE_URL=https://api.x.ai/v1]      Grok
//	BASETEN_API_KEY=…             [BASETEN_BASE_URL=https://inference.baseten.co/v1]
//	GROQ_API_KEY=…                [GROQ_BASE_URL=https://api.groq.com/openai/v1]
//	DEEPSEEK_API_KEY=…            [DEEPSEEK_BASE_URL=https://api.deepseek.com/v1]
//	MEMDOOR_MODEL=<model id>      required for OpenAI and Google (their ids
//	                              change; name the one your key may use);
//	                              Anthropic defaults to claude-sonnet-5
//	MEMDOOR_MODEL_CONTEXT=<tokens> the window (default 200000 / 128000)
//	MEMDOOR_VENDOR_HEADERS=…      extra headers on every vendor request, for a
//	                              company AI gateway's cost attribution
//	                              ("X-Team: data; X-Email: me@corp.example")
//	MEMDOOR_VENDOR=off            ignore the keys above (use OpenRouter or a seat)
//
// A company AI gateway (Greg, 2026-10-02: "golden env var preset at runtime …
// provider / model refreshed in place … tagged with your email and team for
// cost attribution") is exactly this contract: it sets the base URL, the key
// and the model in the environment, and the headers carry the tags. The
// environment is read when the gateway starts; a refreshed preset is a
// restart.
//
// A vendor key wins over an OpenRouter key when both are set: a company key
// in the environment is deliberate.
const (
	VendorEngineName = "vendor-key"
	VendorAnthropic  = "anthropic"
	VendorOpenAI     = "openai"
	VendorGoogle     = "google"
	VendorXAI        = "xai"
	VendorBaseten    = "baseten"
	VendorGroq       = "groq"
	VendorDeepSeek   = "deepseek"
	// VendorGateway is a company's own AI gateway speaking the OpenAI
	// Responses API (responses.go); VendorGatewayChat the same gateway on
	// the chat/completions shape (AI_GATEWAY_API=chat).
	VendorGateway         = "gateway"
	VendorGatewayChat     = "gateway-chat"
	gatewayDefaultContext = 128_000

	anthropicDefaultBase    = "https://api.anthropic.com"
	anthropicDefaultModel   = "claude-sonnet-5"
	anthropicDefaultContext = 200_000
	openaiDefaultBase       = "https://api.openai.com/v1"
	openaiDefaultContext    = 128_000
	// Gemini speaks the OpenAI shape at this base; the same client serves it.
	googleDefaultBase    = "https://generativelanguage.googleapis.com/v1beta/openai"
	googleDefaultContext = 1_000_000
	// Grok and Baseten speak the OpenAI shape too (Greg, 2026-10-02: "we
	// should try baseten and grok as well").
	xaiDefaultBase         = "https://api.x.ai/v1"
	xaiDefaultContext      = 256_000
	basetenDefaultBase     = "https://inference.baseten.co/v1"
	basetenDefaultContext  = 128_000
	groqDefaultBase        = "https://api.groq.com/openai/v1"
	groqDefaultContext     = 128_000
	deepseekDefaultBase    = "https://api.deepseek.com/v1"
	deepseekDefaultContext = 128_000
)

// VendorEngine is the engine for a gateway holding a company's vendor key,
// and whether there is one. Reason says why there is none when a key was
// found but could not be used (a missing model), so the gateway can log it.
func VendorEngine() (re RemoteEngine, ok bool, reason string) {
	if v := strings.TrimSpace(os.Getenv("MEMDOOR_VENDOR")); v == "0" || strings.EqualFold(v, "off") {
		return RemoteEngine{}, false, ""
	}
	model := strings.TrimSpace(os.Getenv("MEMDOOR_MODEL"))
	ctxLen := envInt("MEMDOOR_MODEL_CONTEXT")
	// The company's own gateway first: its base URL and token are the whole
	// integration (Greg, 2026-10-02: `AI_GATEWAY_BASE_URL` and a
	// `*_AI_GATEWAY_TOKEN`, `model` = the provider slug set up in its UI).
	if base := strings.TrimSpace(os.Getenv("AI_GATEWAY_BASE_URL")); base != "" {
		token := gatewayToken()
		switch {
		case token == "":
			return RemoteEngine{}, false, "AI_GATEWAY_BASE_URL is set but no AI_GATEWAY_TOKEN (or *_AI_GATEWAY_TOKEN): the gateway needs your personal or service-account token"
		case model == "":
			return RemoteEngine{}, false, "AI_GATEWAY_BASE_URL is set but MEMDOOR_MODEL is not: name the provider slug your gateway serves"
		}
		if ctxLen == 0 {
			ctxLen = gatewayDefaultContext
		}
		vendor := VendorGateway
		if strings.EqualFold(strings.TrimSpace(os.Getenv("AI_GATEWAY_API")), "chat") {
			vendor = VendorGatewayChat
		}
		return vendorEngine(vendor, base, token, model, ctxLen), true, ""
	}
	if key := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); key != "" {
		if model == "" {
			model = anthropicDefaultModel
		}
		if ctxLen == 0 {
			ctxLen = anthropicDefaultContext
		}
		return vendorEngine(VendorAnthropic, envOr("ANTHROPIC_BASE_URL", anthropicDefaultBase), key, model, ctxLen), true, ""
	}
	if strings.TrimSpace(os.Getenv("XAI_API_KEY")) == "" {
		if g := strings.TrimSpace(os.Getenv("GROK_API_KEY")); g != "" {
			os.Setenv("XAI_API_KEY", g) // the alias, for the loop below
		}
	}
	for _, v := range []struct {
		vendor, keyVar, baseVar, base string
		ctx                           int
	}{
		{VendorOpenAI, "OPENAI_API_KEY", "OPENAI_BASE_URL", openaiDefaultBase, openaiDefaultContext},
		{VendorGoogle, "GEMINI_API_KEY", "GEMINI_BASE_URL", googleDefaultBase, googleDefaultContext},
		{VendorXAI, "XAI_API_KEY", "XAI_BASE_URL", xaiDefaultBase, xaiDefaultContext},
		{VendorBaseten, "BASETEN_API_KEY", "BASETEN_BASE_URL", basetenDefaultBase, basetenDefaultContext},
		{VendorGroq, "GROQ_API_KEY", "GROQ_BASE_URL", groqDefaultBase, groqDefaultContext},
		{VendorDeepSeek, "DEEPSEEK_API_KEY", "DEEPSEEK_BASE_URL", deepseekDefaultBase, deepseekDefaultContext},
	} {
		key := strings.TrimSpace(os.Getenv(v.keyVar))
		if key == "" {
			continue
		}
		if model == "" {
			return RemoteEngine{}, false, v.keyVar + " is set but MEMDOOR_MODEL is not: name the model your key may use"
		}
		if ctxLen == 0 {
			ctxLen = v.ctx
		}
		return vendorEngine(v.vendor, envOr(v.baseVar, v.base), key, model, ctxLen), true, ""
	}
	return RemoteEngine{}, false, ""
}

func vendorEngine(vendor, base, key, model string, ctxLen int) RemoteEngine {
	return RemoteEngine{
		Name:        VendorEngineName,
		Vendor:      vendor,
		Endpoint:    strings.TrimRight(base, "/"),
		APIKey:      key,
		Model:       model,
		CtxLen:      ctxLen,
		AgentModels: map[string]string{"coder": model},
	}
}

// VendorMode reports whether the active engine is a company's vendor key:
// the mode in which nothing but that vendor is contacted.
func VendorMode() bool {
	re := ActiveRemoteEngine()
	return re != nil && re.Vendor != ""
}

// newVendorClient is the client for a vendor engine: Anthropic's own API, or
// the plain OpenAI-compatible client with no routing policy and no
// attribution — the request a company's gateway expects to see.
func newVendorClient(_ context.Context, re *RemoteEngine, model string) LLMClient {
	switch re.Vendor {
	case VendorAnthropic:
		return newAnthropicClient(re.APIKey, re.Endpoint, model)
	case VendorGateway:
		return newResponsesClient(re.APIKey, re.Endpoint, model)
	}
	return &oaiClient{
		apiKey:     re.APIKey,
		baseURL:    strings.TrimSuffix(strings.TrimRight(re.Endpoint, "/"), "/chat/completions"),
		httpClient: oaiHTTPClient(),
		model:      model,
		vendor:     true,
	}
}

// VendorHeaders is MEMDOOR_VENDOR_HEADERS parsed: "Name: value" pairs
// separated by ";" or newlines, each sent on every vendor request.
func VendorHeaders() map[string]string {
	out := map[string]string{}
	for _, pair := range strings.FieldsFunc(os.Getenv("MEMDOOR_VENDOR_HEADERS"), func(r rune) bool { return r == ';' || r == '\n' }) {
		name, value, ok := strings.Cut(pair, ":")
		if name, value = strings.TrimSpace(name), strings.TrimSpace(value); ok && name != "" && value != "" {
			out[name] = value
		}
	}
	return out
}

func setVendorHeaders(h http.Header) {
	for name, value := range VendorHeaders() {
		h.Set(name, value)
	}
}

// gatewayToken is AI_GATEWAY_TOKEN, or the first variable ending in
// _AI_GATEWAY_TOKEN — a company names it after itself (ACME_AI_GATEWAY_TOKEN).
func gatewayToken() string { t, _ := gatewayTokenVar(); return t }

// gatewayTokenVar is the token and the variable it came from, for the
// credential's source line.
func gatewayTokenVar() (string, string) {
	if t := strings.TrimSpace(os.Getenv("AI_GATEWAY_TOKEN")); t != "" {
		return t, "AI_GATEWAY_TOKEN"
	}
	for _, kv := range os.Environ() {
		name, value, _ := strings.Cut(kv, "=")
		if strings.HasSuffix(name, "_AI_GATEWAY_TOKEN") && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), name
		}
	}
	return "", "AI_GATEWAY_TOKEN"
}

// xaiKey is XAI_API_KEY, or GROK_API_KEY — the name people give it
// (Greg's .envrc, 2026-10-02).
func xaiKey() string { return strings.TrimSpace(os.Getenv(xaiKeyVar())) }

func xaiKeyVar() string {
	if strings.TrimSpace(os.Getenv("XAI_API_KEY")) == "" && strings.TrimSpace(os.Getenv("GROK_API_KEY")) != "" {
		return "GROK_API_KEY"
	}
	return "XAI_API_KEY"
}

func envOr(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

func envInt(name string) int {
	n := 0
	for _, c := range strings.TrimSpace(os.Getenv(name)) {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

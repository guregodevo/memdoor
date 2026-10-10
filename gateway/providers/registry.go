package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"memdoor/pkg/shared"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PROVIDERS, EACH WITH ITS LIST OF MODELS (Greg, 2026-10-02: "we had an
// OpenRouter integration … we can continue to extend it with different
// providers including the company provider … each provider has a list of
// models actually"). What omp's models.yml, Codex's model_providers and
// Claude Code's gateway discovery converge on, in one registry:
//
//   - a Provider is a name, a wire (openrouter | chat | responses |
//     anthropic), a base URL, a credential, headers, and its models —
//     fetched from the provider's own list endpoint, or declared by hand
//     when a gateway returns none;
//   - the built-in ones come from the environment (the company gateway,
//     Anthropic, OpenAI, Gemini, OpenRouter) exactly as before — nothing a
//     preset sets changes;
//   - the ones a person adds live in ~/.memdoor/providers.json (0600):
//     `/connect` writes it, nobody has to edit it;
//   - /model shows them all, grouped; picking a model pins its provider.
//
// A credential is a literal, the NAME of an environment variable that is
// set, or "!command" whose stdout is the key (Keychain, 1Password, Vault) —
// omp/pi's rule, so a company's secrets tooling fits without a change.

const (
	APIOpenRouter = "openrouter"
	APIChat       = "chat"
	APIResponses  = "responses"
	APIAnthropic  = "anthropic"
	// APIDecisions is a decision model's endpoint (System One: Jev, Kev),
	// not a chat one: it judges, it never answers a turn, and it lists no
	// models for /model (Greg, 2026-10-03: decisions without OpenRouter,
	// "if it's not we should have a hint for users to connect").
	APIDecisions = "decisions"

	// TypeSafeID is the decision provider `memdoor connect typesafe` adds.
	TypeSafeID      = "typesafe"
	typesafeDefault = "https://api.typesafe.ai"

	providersFile = "providers.json"
	modelsTTL     = time.Hour
)

// Model is one entry of a provider's list; prices are the provider's own
// when it states them (OpenRouter does), zero when it does not.
type Model struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Context  int     `json:"context,omitempty"`
	Tools    bool    `json:"tools,omitempty"`
	InPerM   float64 `json:"in_per_m,omitempty"`
	OutPerM  float64 `json:"out_per_m,omitempty"`
	Provider string  `json:"provider,omitempty"`
	// MaxOutput is the reply cap the reference states (reference.go); 0 =
	// unknown.
	MaxOutput int `json:"max_output,omitempty"`
	// ContextSource says where Context came from: "provider" (its own API),
	// "catalogue" (OpenRouter's list of the same model), "reference"
	// (models.dev), "declared" (providers.json / MEMDOOR_MODEL), or
	// "default" — a guess, said so wherever the model is shown or pinned.
	ContextSource string `json:"context_source,omitempty"`
	// What the provider's API says the model takes and does (catalogue.go).
	Inputs   []string `json:"inputs,omitempty"`   // "text", "image" …
	Thinking bool     `json:"thinking,omitempty"` // a reasoning/thinking mode
	Effort   bool     `json:"effort,omitempty"`   // an effort parameter
}

const (
	ContextFromProvider  = "provider"
	ContextFromCatalogue = "catalogue"
	ContextFromReference = "reference"
	ContextDeclared      = "declared"
	ContextDefault       = "default"
)

// Provider is one place models come from. Key is never serialised out.
type Provider struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	API       string            `json:"api"`
	Base      string            `json:"base"`
	Key       string            `json:"-"`
	KeySource string            `json:"key_source,omitempty"` // "env OPENAI_API_KEY", "providers.json", "command", ""
	Headers   map[string]string `json:"headers,omitempty"`
	Models    []Model           `json:"models,omitempty"` // declared by hand (a gateway that returns none)
	Context   int               `json:"context,omitempty"`
	BuiltIn   bool              `json:"built_in,omitempty"`
}

// Connected reports whether the provider has a credential to use.
func (p Provider) Connected() bool { return strings.TrimSpace(p.Key) != "" }

// Engine is the provider as the one active engine.
func (p Provider) Engine(model string) RemoteEngine {
	vendor := ""
	byok := false
	switch p.API {
	case APIAnthropic:
		vendor = VendorAnthropic
	case APIResponses:
		vendor = VendorGateway
	case APIChat:
		vendor = VendorGatewayChat
	case APIOpenRouter:
		byok = true
	case APIChatGPT:
		vendor = VendorChatGPT
	}
	ctxLen := p.Context
	if ctxLen == 0 {
		ctxLen = gatewayDefaultContext
	}
	re := RemoteEngine{Name: VendorEngineName, Vendor: vendor, Endpoint: p.Base, APIKey: p.Key, Model: model, CtxLen: ctxLen, AgentModels: map[string]string{"coder": model}}
	if byok {
		// The key is the provider's own: a connected OpenRouter lives in the
		// file, where ByokEngine (the environment's) does not look.
		re.Name, re.Byok = ByokEngineName, true
	}
	return re
}

// Client is the provider's client for one of its models.
func (p Provider) Client(ctx context.Context, model string) LLMClient {
	re := p.Engine(model)
	if p.API == APIOpenRouter {
		return newDirectOpenRouterClient(ctx, &re, model)
	}
	if p.API == APIChatGPT {
		return newChatGPTClient(model)
	}
	return newVendorClient(ctx, &re, model)
}

// providersFileSpec is the on-disk shape: what /connect writes. ProviderSpec
// is one entry of it.
type providersFileSpec struct {
	Providers []ProviderSpec `json:"providers"`
}

type ProviderSpec struct {
	ID      string            `json:"id"`
	Name    string            `json:"name,omitempty"`
	API     string            `json:"api"`
	Base    string            `json:"base"`
	Key     string            `json:"key"` // literal, an env var NAME, or "!command"
	Headers map[string]string `json:"headers,omitempty"`
	Models  []Model           `json:"models,omitempty"`
	Context int               `json:"context,omitempty"`
}

func providersPath() (string, error) {
	return shared.MemdoorHome(providersFile), nil
}

// resolveKey is the credential rule: an environment variable's NAME when
// one by that name is set, "!command" for the command's stdout, else the
// literal. The second value says which.
func resolveKey(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "":
		return "", ""
	case strings.HasPrefix(raw, "!"):
		out, err := exec.Command("sh", "-c", raw[1:]).Output()
		if err != nil {
			return "", "command failed"
		}
		return strings.TrimSpace(string(out)), "command"
	}
	if looksLikeEnvName(raw) {
		// A NAME, not a key: resolved in the gateway's environment. Unset
		// there, it is an error in words — live 2026-10-02 the name went to
		// Anthropic as the key and came back "invalid x-api-key".
		if v := strings.TrimSpace(os.Getenv(raw)); v != "" {
			return v, "env " + raw
		}
		return "", raw + " is not set in the gateway's environment (the gateway was started without it: set it where the gateway starts, then restart it)"
	}
	return raw, "providers.json"
}

// looksLikeEnvName: upper-case letters, digits and underscores only, at
// least one underscore — OPENAI_API_KEY, CORP_TOKEN — never a real key.
func looksLikeEnvName(s string) bool {
	if !strings.Contains(s, "_") {
		return false
	}
	for _, r := range s {
		if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// KeySpec is a credential as written, resolved: the value and where it
// came from ("env NAME", "command", "providers.json", or "command failed").
type KeySpec struct{ Value, Source string }

// ResolveKey is resolveKey for callers outside the package.
func ResolveKey(raw string) KeySpec { v, s := resolveKey(raw); return KeySpec{Value: v, Source: s} }

// Providers is every provider this gateway knows: the built-in ones from
// the environment, then the file's. Order is the order /model shows.
func Providers() []Provider {
	var out []Provider
	seen := map[string]bool{}
	add := func(p Provider) {
		if p.ID == "" || seen[p.ID] {
			return
		}
		seen[p.ID] = true
		out = append(out, p)
	}
	var fromFile []Provider
	if path, err := providersPath(); err == nil {
		if b, err := os.ReadFile(path); err == nil {
			var f providersFileSpec
			if json.Unmarshal(b, &f) == nil {
				for _, s := range f.Providers {
					key, source := resolveKey(s.Key)
					name := s.Name
					if name == "" {
						name = s.ID
					}
					fromFile = append(fromFile, Provider{ID: s.ID, Name: name, API: s.API, Base: strings.TrimRight(s.Base, "/"), Key: key, KeySource: source, Headers: s.Headers, Models: s.Models, Context: s.Context})
				}
			}
		}
	}
	for _, p := range builtInProviders() {
		// A built-in connected through /connect lives in the file under the
		// same id: what the environment does not give, the file does, so a
		// kept connection is there at every start (Greg, 2026-10-03: "once
		// it is connected store it so we don't need to reconnect").
		if !p.Connected() {
			for _, f := range fromFile {
				if f.ID == p.ID && f.Connected() {
					p.Key, p.KeySource = f.Key, f.KeySource
					if f.Base != "" {
						p.Base = f.Base
					}
					if len(f.Models) > 0 {
						p.Models = f.Models
					}
					if f.Context > 0 {
						p.Context = f.Context
					}
					if len(f.Headers) > 0 {
						p.Headers = f.Headers
					}
				}
			}
		}
		add(p)
	}
	for _, f := range fromFile {
		add(f)
	}
	return out
}

// builtInProviders are the environment's: the same variables the engines
// read (vendor.go, byok.go), listed whether or not a key is set, so /model
// can show a provider as locked and offer /connect.
func builtInProviders() []Provider {
	var out []Provider
	if base := strings.TrimSpace(os.Getenv("AI_GATEWAY_BASE_URL")); base != "" {
		api := APIResponses
		if strings.EqualFold(strings.TrimSpace(os.Getenv("AI_GATEWAY_API")), "chat") {
			api = APIChat
		}
		token, tokenVar := gatewayTokenVar()
		p := Provider{ID: "gateway", Name: "Company AI gateway", API: api, Base: strings.TrimRight(base, "/"), Key: token, KeySource: "env " + tokenVar, BuiltIn: true, Headers: VendorHeaders()}
		if m := strings.TrimSpace(os.Getenv("MEMDOOR_MODEL")); m != "" {
			p.Models = []Model{{ID: m, Name: m, Context: envInt("MEMDOOR_MODEL_CONTEXT")}}
		}
		out = append(out, p)
	}
	out = append(out,
		Provider{ID: "anthropic", Name: "Anthropic", API: APIAnthropic, Base: envOr("ANTHROPIC_BASE_URL", anthropicDefaultBase), Key: os.Getenv("ANTHROPIC_API_KEY"), KeySource: "env ANTHROPIC_API_KEY", Context: anthropicDefaultContext, BuiltIn: true},
		// OpenAI over the Responses API (2026-10-10): its chat endpoint refuses
		// the gpt-6 models with tools and a reasoning effort ("Function
		// tools with reasoning_effort are not supported for gpt-6-sol in
		// /v1/chat/completions", live, three models in a row); Responses
		// takes both. Same base, same key, same list endpoint.
		Provider{ID: "openai", Name: "OpenAI", API: APIResponses, Base: envOr("OPENAI_BASE_URL", openaiDefaultBase), Key: os.Getenv("OPENAI_API_KEY"), KeySource: "env OPENAI_API_KEY", Context: openaiDefaultContext, BuiltIn: true},
		chatgptProvider(),
		Provider{ID: "gemini", Name: "Google Gemini", API: APIChat, Base: envOr("GEMINI_BASE_URL", googleDefaultBase), Key: os.Getenv("GEMINI_API_KEY"), KeySource: "env GEMINI_API_KEY", Context: googleDefaultContext, BuiltIn: true},
		Provider{ID: "xai", Name: "xAI Grok", API: APIChat, Base: envOr("XAI_BASE_URL", xaiDefaultBase), Key: xaiKey(), KeySource: "env " + xaiKeyVar(), Context: xaiDefaultContext, BuiltIn: true},
		Provider{ID: "baseten", Name: "Baseten", API: APIChat, Base: envOr("BASETEN_BASE_URL", basetenDefaultBase), Key: os.Getenv("BASETEN_API_KEY"), KeySource: "env BASETEN_API_KEY", Context: basetenDefaultContext, BuiltIn: true},
		Provider{ID: "groq", Name: "Groq", API: APIChat, Base: envOr("GROQ_BASE_URL", groqDefaultBase), Key: os.Getenv("GROQ_API_KEY"), KeySource: "env GROQ_API_KEY", Context: groqDefaultContext, BuiltIn: true},
		Provider{ID: "deepseek", Name: "DeepSeek", API: APIChat, Base: envOr("DEEPSEEK_BASE_URL", deepseekDefaultBase), Key: os.Getenv("DEEPSEEK_API_KEY"), KeySource: "env DEEPSEEK_API_KEY", Context: deepseekDefaultContext, BuiltIn: true},
		Provider{ID: "openrouter", Name: "OpenRouter", API: APIOpenRouter, Base: "https://openrouter.ai/api/v1", Key: ByokKey(), KeySource: "env OPEN_ROUTER_API_KEY", Context: byokCtxLen, BuiltIn: true},
		Provider{ID: TypeSafeID, Name: "TypeSafe (Jev, decisions)", API: APIDecisions, Base: envOr("MEMDOOR_SYSTEMONE_URL", typesafeDefault), Key: typesafeKey(), KeySource: "env " + typesafeKeyVar(), BuiltIn: true},
	)
	for i := range out {
		if !out[i].Connected() {
			out[i].KeySource = ""
		}
	}
	return out
}

// FindProvider is a provider by id.
func FindProvider(id string) (Provider, bool) {
	for _, p := range Providers() {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// OpenRouterModels is how the OpenRouter list is read — the gateway's
// catalogue (byok_catalog.go) with its real prices; set at wiring.
var OpenRouterModels func(key string) ([]Model, error)

var modelLists = struct {
	mu   sync.Mutex
	at   map[string]time.Time
	list map[string][]Model
}{at: map[string]time.Time{}, list: map[string][]Model{}}

// ModelsOf is the provider's list: declared models first, then what its
// list endpoint says, cached for an hour. A provider with no credential
// has no list; one whose endpoint answers nothing keeps its declared
// models (a gateway's rule: add a model by hand only when it returns none).
func ModelsOf(ctx context.Context, p Provider) ([]Model, error) {
	if !p.Connected() {
		return nil, fmt.Errorf("%s is not connected", p.Name)
	}
	if p.API == APIDecisions {
		return nil, nil // it judges; no model of its answers a turn
	}
	modelLists.mu.Lock()
	if list, ok := modelLists.list[p.ID]; ok && time.Since(modelLists.at[p.ID]) < modelsTTL {
		modelLists.mu.Unlock()
		return list, nil
	}
	modelLists.mu.Unlock()
	// The provider's own metadata API, through its reader (catalogue.go).
	fetched, err := readerFor(p).ListModels(ctx, p)
	list := append([]Model{}, p.Models...)
	seen := map[string]int{}
	for i := range list {
		seen[list[i].ID] = i
		if list[i].Context > 0 {
			list[i].ContextSource = ContextDeclared
		}
	}
	for _, m := range fetched {
		if i, ok := seen[m.ID]; ok {
			// A declared model the provider also lists: what the declaration
			// left empty takes the provider's figures (live 2026-10-02: a
			// gateway slug stayed at the 128k default while the gateway's
			// list said 200k).
			d := &list[i]
			if d.Context == 0 && m.Context > 0 {
				d.Context, d.ContextSource = m.Context, ContextFromProvider
			}
			if d.MaxOutput == 0 {
				d.MaxOutput = m.MaxOutput
			}
			if len(d.Inputs) == 0 {
				d.Inputs = m.Inputs
			}
			if d.Name == d.ID && m.Name != "" {
				d.Name = m.Name
			}
			continue
		}
		if m.Context > 0 {
			m.ContextSource = ContextFromProvider
		}
		list = append(list, m)
	}
	for i := range list {
		list[i].Provider = p.ID
		// The window: the provider's own figure, else the same model in
		// OpenRouter's catalogue (an API too), else the reference
		// (reference.go), else the provider's default — a guess, marked.
		if list[i].Context == 0 {
			if lim, ok := CatalogueLimits(p.ID, list[i].ID); ok {
				list[i].Context, list[i].MaxOutput, list[i].ContextSource = lim.Context, lim.Output, ContextFromCatalogue
			} else if lim, ok := ReferenceLimits(ctx, p.ID, list[i].ID); ok {
				list[i].Context, list[i].MaxOutput, list[i].ContextSource = lim.Context, lim.Output, ContextFromReference
				if list[i].InPerM == 0 && list[i].OutPerM == 0 {
					list[i].InPerM, list[i].OutPerM = lim.InPerM, lim.OutPerM
				}
			} else {
				list[i].Context, list[i].ContextSource = p.Context, ContextDefault
				if list[i].Context == 0 {
					list[i].Context = gatewayDefaultContext
				}
			}
		} else if list[i].MaxOutput == 0 || list[i].InPerM == 0 {
			if lim, ok := ReferenceLimits(ctx, p.ID, list[i].ID); ok {
				if list[i].MaxOutput == 0 {
					list[i].MaxOutput = lim.Output
				}
				// A vendor whose API states no price (Anthropic, DeepSeek,
				// xAI's own) shows its list price from the reference.
				if list[i].InPerM == 0 && list[i].OutPerM == 0 {
					list[i].InPerM, list[i].OutPerM = lim.InPerM, lim.OutPerM
				}
			}
		}
	}
	if err != nil && len(list) == 0 {
		return nil, err
	}
	modelLists.mu.Lock()
	modelLists.at[p.ID], modelLists.list[p.ID] = time.Now(), list
	modelLists.mu.Unlock()
	return list, nil
}

// ForgetAllModels drops every cached list and the reference catalogue's
// in-memory copy: `memdoor providers --refresh`, when a vendor shipped.
func ForgetAllModels() {
	modelLists.mu.Lock()
	modelLists.at, modelLists.list = map[string]time.Time{}, map[string][]Model{}
	modelLists.mu.Unlock()
	modelsDev.mu.Lock()
	modelsDev.doc, modelsDev.at = nil, time.Time{}
	modelsDev.mu.Unlock()
	if path, err := modelsDevPath(); err == nil {
		_ = os.Remove(path)
	}
}

// ForgetModels drops a provider's cached list (after /connect, a refresh).
func ForgetModels(id string) {
	modelLists.mu.Lock()
	delete(modelLists.at, id)
	delete(modelLists.list, id)
	modelLists.mu.Unlock()
}

// modelsURL is the OpenAI-shaped list under a base: ".../v1" → ".../v1/
// models", ".../ai" → ".../ai/v1/models".
func modelsURL(base string) string {
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/v1") {
		return base + "/models"
	}
	return base + "/v1/models"
}

// SplitPin reads "provider:model" — the explicit form for an id two
// providers list (qwen/qwen3.8-27b on Groq and on OpenRouter). A prefix
// that is not a provider's id is part of the model id.
func SplitPin(id string) (provider, model string) {
	if i := strings.Index(id, ":"); i > 0 {
		for _, p := range Providers() {
			if p.ID == id[:i] {
				return p.ID, id[i+1:]
			}
		}
	}
	return "", id
}

// FindModel is the provider that serves a model id: the one named by a
// "provider:" prefix, else the active engine's provider when it lists it,
// else the first connected provider that does. The model returned carries
// the bare id, as the provider names it.
func FindModel(ctx context.Context, id string) (Provider, Model, bool) {
	if want, bare := SplitPin(id); want != "" {
		p, ok := FindProvider(want)
		if !ok || !p.Connected() {
			return Provider{}, Model{}, false
		}
		list, err := ModelsOf(ctx, p)
		if err != nil {
			return Provider{}, Model{}, false
		}
		for _, m := range list {
			if m.ID == bare {
				return p, m, true
			}
		}
		return Provider{}, Model{}, false
	}
	active := ActiveProviderID()
	var found *Provider
	var foundModel Model
	var plan *Provider
	var planModel Model
	for _, p := range Providers() {
		if !p.Connected() {
			continue
		}
		list, err := ModelsOf(ctx, p)
		if err != nil {
			continue
		}
		for _, m := range list {
			if m.ID != id {
				continue
			}
			if p.ID == ChatGPTID {
				pp := p
				plan, planModel = &pp, m
				continue
			}
			if p.ID == active && p.ID != "openai" {
				return p, m, true
			}
			if found == nil || (found.ID == "openai" && p.ID == active) {
				pp := p
				found, foundModel = &pp, m
			}
		}
	}
	// THE PLAN BEFORE THE KEY (live 2026-10-10): a signed-in ChatGPT plan
	// and an OPENAI_API_KEY in the environment list the same ids, and a
	// bare pin went to the key, whose chat endpoint refuses those models
	// with tools ("Function tools with reasoning_effort are not supported
	// for gpt-6-astra in /v1/chat/completions"). The plan is the person's
	// own sign-in and costs them nothing; the key is ambient. An explicit
	// "openai:<id>" still reaches the key (SplitPin, above).
	if plan != nil && (found == nil || found.ID == "openai") {
		return *plan, planModel, true
	}
	if found != nil {
		return *found, foundModel, true
	}
	return Provider{}, Model{}, false
}

// ActiveProviderID names the registry entry the active engine came from.
func ActiveProviderID() string { return providerOfEngine(ActiveRemoteEngine()) }

// SearchModels is a provider's list narrowed to q: an id or name
// containing it, ids that start with it first; the whole list when q is
// empty. At most limit rows.
func SearchModels(list []Model, q string, limit int) []Model {
	q = strings.ToLower(strings.TrimSpace(q))
	var starts, contains []Model
	for _, m := range list {
		id, name := strings.ToLower(m.ID), strings.ToLower(m.Name)
		switch {
		case q == "":
			contains = append(contains, m)
		case strings.HasPrefix(id, q):
			starts = append(starts, m)
		case strings.Contains(id, q) || strings.Contains(name, q):
			contains = append(contains, m)
		}
	}
	out := append(starts, contains...)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// providerOfEngine names the registry entry an active engine came from.
func providerOfEngine(re *RemoteEngine) string {
	switch {
	case re == nil:
		return ""
	case re.Byok:
		return "openrouter"
	case re.Vendor == VendorAnthropic:
		return "anthropic"
	case re.Vendor == VendorChatGPT:
		return ChatGPTID
	case re.Vendor == VendorGateway || re.Vendor == VendorGatewayChat:
		for _, p := range Providers() {
			if p.Base == re.Endpoint && p.Key == re.APIKey {
				return p.ID
			}
		}
		return "gateway"
	case re.Vendor == VendorOpenAI:
		return "openai"
	case re.Vendor == VendorGoogle:
		return "gemini"
	case re.Vendor == VendorXAI:
		return "xai"
	case re.Vendor == VendorBaseten:
		return "baseten"
	case re.Vendor == VendorGroq:
		return "groq"
	case re.Vendor == VendorDeepSeek:
		return "deepseek"
	}
	return ""
}

// SaveProvider writes or replaces one entry of ~/.memdoor/providers.json
// (0600, the directory 0700 if new): what /connect does.
func SaveProvider(s ProviderSpec) error {
	if s.ID == "" || s.API == "" || s.Base == "" {
		return fmt.Errorf("a provider needs an id, an api and a base URL")
	}
	path, err := providersPath()
	if err != nil {
		return err
	}
	var f providersFileSpec
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &f)
	}
	kept := f.Providers[:0]
	for _, p := range f.Providers {
		if p.ID != s.ID {
			kept = append(kept, p)
		}
	}
	f.Providers = append(kept, s)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(f, "", "  ")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	ForgetModels(s.ID)
	return nil
}

// RemoveProvider deletes an entry of the file.
func RemoveProvider(id string) error {
	path, err := providersPath()
	if err != nil {
		return err
	}
	var f providersFileSpec
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_ = json.Unmarshal(b, &f)
	kept := f.Providers[:0]
	for _, p := range f.Providers {
		if p.ID != id {
			kept = append(kept, p)
		}
	}
	f.Providers = kept
	out, _ := json.MarshalIndent(f, "", "  ")
	ForgetModels(id)
	return os.WriteFile(path, out, 0o600)
}

// typesafeKeyVar is the environment variable the TypeSafe key is under.
func typesafeKeyVar() string {
	for _, v := range []string{"MEMDOOR_SYSTEMONE_API_KEY", "TYPESAFE_API_KEY"} {
		if strings.TrimSpace(os.Getenv(v)) != "" {
			return v
		}
	}
	return "TYPESAFE_API_KEY"
}

func typesafeKey() string { return strings.TrimSpace(os.Getenv(typesafeKeyVar())) }

// DecisionProvider is the connected decision provider: the one connected
// by `memdoor connect typesafe`, else the built-in from the environment.
func DecisionProvider() (Provider, bool) {
	var fromEnv *Provider
	for _, p := range Providers() {
		if p.API != APIDecisions || !p.Connected() {
			continue
		}
		if !p.BuiltIn {
			return p, true
		}
		q := p
		fromEnv = &q
	}
	if fromEnv != nil {
		return *fromEnv, true
	}
	return Provider{}, false
}

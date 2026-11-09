package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/providers"
	"memdoor/pkg/authorization"
	"memdoor/pkg/decision"
	"memdoor/pkg/decision/systemone"
	"memdoor/pkg/llm"
)

// /connect (the maintainer, 2026-10-02, after reading how Cursor, Claude Code, Codex
// and omp do it): pick the kind of provider, give its base URL and token,
// and the gateway PROBES before it keeps anything — the provider's model
// list, then one tiny call — so a wrong URL or token fails here, in words,
// not in the middle of a turn. omp has no way to add a custom provider but
// a YAML file and never tests it; Claude Code tests with a curl the person
// runs by hand. Here it is one request: POST /api/providers/connect.

var providerIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// decisionsSuffix names the decision provider a local runtime brings with it.
const decisionsSuffix = "-decisions"

// localRuntimeBases are where the runtimes listen by default, janis first:
// a blank base on `connect local` tries them in this order on the gateway's
// machine (the box, when the gateway is there).
var localRuntimeBases = []string{
	"http://localhost:8090/v1",  // janis serve
	"http://localhost:11434/v1", // ollama
	"http://localhost:8080/v1",  // llama-server
	"http://localhost:1234/v1",  // LM Studio
	"http://localhost:8000/v1",  // vLLM
}

// runtimeNames are the runtimes by default port, for the menu.
var runtimeNames = map[string]string{"8090": "janis", "11434": "Ollama", "8080": "llama.cpp", "1234": "LM Studio", "8000": "vLLM"}

// localRuntime is a default address whose model list answered.
type localRuntime struct {
	Name   string `json:"name"`
	Base   string `json:"base"`
	Models int    `json:"models"`
}

// answeringRuntimes asks every default address at once and keeps the ones
// that list models, in localRuntimeBases order.
func answeringRuntimes(ctx context.Context) (found []localRuntime, tried []string) {
	results := make([]*localRuntime, len(localRuntimeBases))
	var wg sync.WaitGroup
	for i, base := range localRuntimeBases {
		tried = append(tried, base)
		wg.Add(1)
		go func(i int, base string) {
			defer wg.Done()
			pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			id := fmt.Sprintf("local-probe-%d", i)
			list, err := providers.ModelsOf(pctx, providers.Provider{ID: id, API: providers.APIChat, Base: base, Keyless: true})
			providers.ForgetModels(id)
			if err != nil || len(list) == 0 {
				return
			}
			name := "a model server"
			if u, err := url.Parse(base); err == nil {
				if n, ok := runtimeNames[u.Port()]; ok {
					name = n
				}
			}
			results[i] = &localRuntime{Name: name, Base: base, Models: len(list)}
		}(i, base)
	}
	wg.Wait()
	for _, r := range results {
		if r != nil {
			found = append(found, *r)
		}
	}
	return found, tried
}

// findLocalRuntime is the first default address whose model list answers.
func findLocalRuntime(ctx context.Context) (string, []string) {
	var tried []string
	for _, base := range localRuntimeBases {
		tried = append(tried, base)
		pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		p := providers.Provider{ID: "local", API: providers.APIChat, Base: base, Keyless: true}
		providers.ForgetModels("local")
		list, err := providers.ModelsOf(pctx, p)
		cancel()
		if err == nil && len(list) > 0 {
			return base, tried
		}
	}
	return "", tried
}

type connectRequest struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	API     string            `json:"api"`   // responses | chat | anthropic
	Base    string            `json:"base"`  // the provider's base URL
	Key     string            `json:"key"`   // literal, an env var NAME, or "!command"
	Model   string            `json:"model"` // a model to declare and test with (a gateway that lists none)
	Headers map[string]string `json:"headers"`
	Context int               `json:"context"`
	Probe   bool              `json:"probe"`   // true: test only, keep nothing
	Keyless bool              `json:"keyless"` // a local runtime (Ollama, llama.cpp, janis): no token, none sent
	Select  bool              `json:"select"`  // keep the tested model as the person's pin: every new conversation starts on it
}

type connectResult struct {
	OK        bool              `json:"ok"`
	ID        string            `json:"id"`
	API       string            `json:"api"`
	Models    int               `json:"models"`
	Sample    []string          `json:"sample,omitempty"`
	Tested    string            `json:"tested,omitempty"`   // the model the test call used
	Selected  string            `json:"selected,omitempty"` // the pin kept for the person: provider:model
	Answer    string            `json:"answer,omitempty"`
	Error     string            `json:"error,omitempty"`
	Advice    string            `json:"advice,omitempty"`
	Saved     bool              `json:"saved"`
	Headers   map[string]string `json:"headers,omitempty"`
	Decisions bool              `json:"decisions,omitempty"`  // a local runtime's decisions endpoint answered and was kept as <id>-decisions
	Base      string            `json:"base,omitempty"`       // the address a blank base resolved to (a local runtime found on this machine)
	FreeModel string            `json:"free_model,omitempty"` // an OpenRouter account with no credit: every turn runs on this free model
}

// handleProvidersConnect is POST /api/providers/connect and DELETE
// /api/providers/connect?id=. The gateway operator's alone: a "!command" key
// runs on this machine and an env var's NAME sends the gateway's own key to
// the base URL given, and the route answered anyone, no token needed, until
// 2026-10-30.
func (s *Server) handleProvidersConnect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !s.gatewayOperator(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		found, tried := answeringRuntimes(r.Context())
		_ = json.NewEncoder(w).Encode(map[string]any{"runtimes": found, "tried": tried})
		return
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if err := providers.RemoveProvider(id); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), http.StatusBadRequest)
			return
		}
		_ = providers.RemoveProvider(id + decisionsSuffix) // a local runtime's decisions go with it
		if id == providers.OpenRouterID {
			s.selectModelEngine() // its engine goes with the key
		}
		if s.rewireDecisions != nil {
			s.rewireDecisions()
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "id": id})
		return
	case http.MethodPost:
	default:
		http.Error(w, `{"error": "GET, POST or DELETE"}`, http.StatusMethodNotAllowed)
		return
	}
	var req connectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "a JSON body: id, api, base, key"}`, http.StatusBadRequest)
		return
	}
	// Always 200: a failed probe is an answer in words (error, advice), not
	// an HTTP error the client would have to unwrap.
	res := connectProvider(r.Context(), req)
	selectConnected(string(authorization.GetActorID(r.Context())), req, &res)
	openRouterKept := res.OK && res.Saved && res.ID == providers.OpenRouterID
	if openRouterKept {
		// The OpenRouter path from this turn on: its ladder, its privacy
		// policy, web search; no gateway start needed.
		s.selectModelEngine()
		if re := providers.ActiveRemoteEngine(); re != nil && re.Byok && re.FreeTier && re.APIKey == providers.ByokKey() {
			res.FreeModel = re.Model
		}
	}
	if res.OK && res.Saved && (req.API == providers.APIDecisions || res.Decisions || openRouterKept) && s.rewireDecisions != nil {
		s.rewireDecisions() // judges from this turn on, no gateway start needed
	}
	_ = json.NewEncoder(w).Encode(res)
}

// selectConnected keeps the model a local runtime was just tested with as the
// person's pin (persistent_pin.go), so the next conversation runs on it and not
// on a pin from before. Only a local runtime that was saved and asked for it:
// connecting a provider says nothing about which model the person wants.
func selectConnected(actor string, req connectRequest, res *connectResult) {
	if !req.Select || !req.Keyless || !res.OK || !res.Saved || res.Tested == "" {
		return
	}
	pin := res.ID + ":" + res.Tested
	if err := writePersistentPin(actor, &persistentPin{Model: pin}); err != nil {
		logs.New("Agent").Warn("pin not kept: " + err.Error())
		return
	}
	res.Selected = pin
}

// connectProvider probes, then saves unless asked only to probe. The API
// shape is tried when not given: responses, then chat.
func connectProvider(ctx context.Context, req connectRequest) connectResult {
	req.ID, req.Base, req.API = strings.TrimSpace(strings.ToLower(req.ID)), strings.TrimRight(strings.TrimSpace(req.Base), "/"), strings.TrimSpace(req.API)
	res := connectResult{ID: req.ID, API: req.API}
	switch {
	case !providerIDPattern.MatchString(req.ID):
		res.Error = "the provider needs a short id: letters, digits and dashes (corp, lab-proxy)"
		return res
	case req.Keyless && req.Base == "":
		// A blank base on a local runtime: find one on this machine.
		found, tried := findLocalRuntime(ctx)
		if found == "" {
			res.Error = "no runtime answers on this machine (tried " + strings.Join(tried, ", ") + ")"
			res.Advice = "start one: janis serve (github.com/guregodevo/janis), ollama serve, llama-server, LM Studio; or name its address with --base"
			return res
		}
		req.Base = found
		res.Base = found
	case req.Base == "" || !strings.HasPrefix(req.Base, "http"):
		res.Error = "the base URL must start with http:// or https://"
		return res
	case strings.TrimSpace(req.Key) == "" && !req.Keyless:
		res.Error = "a token or key is needed: the value, the NAME of an environment variable that holds it, or !command that prints it"
		return res
	}
	key, source := "", providers.KeySourceNone
	if !req.Keyless {
		key, source = resolveKeyForConnect(req.Key)
		if key == "" {
			res.Error = source
			return res
		}
	}
	if req.API == providers.APIDecisions {
		return connectDecisions(ctx, req, key, source)
	}
	apis := []string{req.API}
	if req.API == "" {
		apis = []string{providers.APIResponses, providers.APIChat}
	}
	var lastErr string
	for _, api := range apis {
		p := providers.Provider{ID: req.ID, Name: req.Name, API: api, Base: req.Base, Key: key, KeySource: source, Keyless: req.Keyless, Headers: req.Headers, Context: req.Context}
		if req.Model != "" {
			p.Models = []providers.Model{{ID: req.Model, Name: req.Model, Context: req.Context}}
		}
		providers.ForgetModels(req.ID)
		list, listErr := providers.ModelsOf(ctx, p)
		if listErr != nil {
			lastErr = listErr.Error()
			if req.Model == "" {
				continue
			}
		}
		model := req.Model
		if model == "" && len(list) > 0 {
			model = list[0].ID
		}
		if model == "" {
			lastErr = "the provider lists no models: name one to test with"
			continue
		}
		// The first model of the list may be one the provider will not
		// serve the way we ask (OpenRouter, 2026-10-24: "No endpoints found
		// for the request with quantization", the precision floor on a
		// model with no fp8 host): that answer still proves the URL and the
		// key, so the next models are tried, and failing all, the key is
		// connected with the model's refusal as the answer.
		candidates := []string{model}
		if req.Model == "" {
			for i := 1; i < len(list) && len(candidates) < 4; i++ {
				candidates = append(candidates, list[i].ID)
			}
		}
		var answer string
		var callErr error
		for _, m := range candidates {
			model = m
			answer, callErr = probeCall(ctx, p, m)
			if callErr == nil || !refusesTheModelNotTheKey(callErr) {
				break
			}
		}
		if callErr != nil && refusesTheModelNotTheKey(callErr) {
			answer, callErr = "(the key and URL answered; "+model+" itself was refused: "+strings.TrimSpace(callErr.Error())+")", nil
		}
		if callErr != nil && hasNoCredit(callErr) {
			// A new account (Sign in with OpenRouter, 2026-10-31) has a good
			// key and nothing to spend yet: connected, and told so.
			answer, callErr = noCreditAnswer(p), nil
		}
		if callErr != nil {
			lastErr = callErr.Error()
			continue
		}
		if answer == "" {
			// A reasoning model may spend the whole small cap thinking and
			// write nothing (live 2026-10-02, GLM 5.3 Flash): the 200 with
			// tokens counted still proves the URL, the token and the shape.
			answer = "(a reply with no text)"
		}
		res.OK, res.API, res.Models, res.Tested, res.Answer = true, api, len(list), model, answer
		if req.Probe {
			// A probe's list was read with the request's bare settings; the
			// registry's own entry (its context, its headers) reads again.
			providers.ForgetModels(req.ID)
		}
		for i, m := range list {
			if i == 5 {
				break
			}
			res.Sample = append(res.Sample, m.ID)
		}
		if !req.Probe {
			spec := providerSpecOf(req, api)
			if err := providers.SaveProvider(spec); err != nil {
				res.OK, res.Error = false, "probed fine but could not save: "+err.Error()
				return res
			}
			res.Saved = true
			logs.New("Providers").Info("provider connected", "id", req.ID, "api", api, "base", req.Base, "models", len(list), "key_source", source)
		}
		if req.Keyless {
			// A local runtime that also judges (janis serve answers
			// /v1/systemone): its decisions are kept as <id>-decisions, so
			// the decision model runs on this machine with no key. A runtime
			// without the endpoint (Ollama, llama.cpp) is simply chat.
			d := connectDecisions(ctx, connectRequest{ID: req.ID + decisionsSuffix, Name: req.Name + " (decisions)", API: providers.APIDecisions, Base: strings.TrimSuffix(req.Base, "/v1"), Keyless: true, Probe: req.Probe}, "", providers.KeySourceNone)
			res.Decisions = d.OK
		}
		return res
	}
	res.Error = oneLine(lastErr, 200)
	res.Advice = connectAdvice(lastErr, apis[len(apis)-1])
	return res
}

// probeCall is the one-token test: a reply, any reply, proves the URL, the
// token and the shape. An unknown-model error still proves the first two.
func probeCall(ctx context.Context, p providers.Provider, model string) (string, error) {
	// A hosted API answers in seconds. A local runtime first loads the model
	// into memory, minutes for a 5 GB file on a laptop, and the probe's 60 s
	// read as "not connected: context deadline exceeded" on a working Ollama
	// (2026-11-01, qwen3:8b with a 32K window on a 16 GB Mac).
	wait := 60 * time.Second
	if p.Keyless {
		wait = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(llm.WithUtilityCall(ctx), wait)
	defer cancel()
	msg, err := p.Client(ctx, model).Messages().New(ctx, llm.MessageNewParams{
		Model: llm.Model(model), MaxTokens: 512, Agent: "connect", // a reasoning model thinks first (gpt-5, 2026-10-02)
		Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("Reply with the single word: ok"))},
	})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, c := range msg.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	return strings.TrimSpace(b.String()), nil
}

// refusesTheModelNotTheKey: the provider read our request and refused the
// model in it, not us. The URL and the key are proven by such an answer.
// hasNoCredit says the provider took the key and refused for want of credit
// (HTTP 402): the key is good, the account is empty.
// noCreditAnswer says where to add credit: OpenRouter's page, with its free
// models, or the host of any other provider (LLM Gateway answers a new
// account's call with 402 too, 2026-11-09).
func noCreditAnswer(p providers.Provider) string {
	var host string
	if u, err := url.Parse(p.Base); err == nil {
		host = u.Hostname()
	}
	if p.API == providers.APIOpenRouter || strings.HasSuffix(host, "openrouter.ai") {
		return "(the key answered; the account has no credit yet: add some at openrouter.ai/settings/credits, or /model a :free model)"
	}
	return "(the key answered; the account has no credit yet: add some at " + strings.TrimPrefix(host, "api.") + ")"
}

func hasNoCredit(err error) bool {
	e := strings.ToLower(err.Error())
	return strings.Contains(e, "out of credit") || strings.Contains(e, "insufficient credits") || strings.Contains(e, "http 402")
}

func refusesTheModelNotTheKey(err error) bool {
	e := strings.ToLower(err.Error())
	for _, s := range []string{"no endpoints found", "not a valid model", "model_not_found", "does not exist", "unknown model", "no such model", "was retired", "deprecated"} {
		if strings.Contains(e, s) {
			return true
		}
	}
	return false
}

func connectAdvice(errText, api string) string {
	e := strings.ToLower(errText)
	switch {
	case strings.Contains(e, "401") || strings.Contains(e, "403"):
		if api == providers.APIAnthropic {
			return "the URL answers but refused the key: an Anthropic API key starts with sk-ant-api…, made in the console (console.anthropic.com → API Keys); a claude.ai login or a Claude Code token is not one"
		}
		return "the URL answers but refused the token: check it is the one this endpoint issues (a gateway's personal or service-account token; a vendor's console key)"
	case strings.Contains(e, "404"):
		return "nothing at that path: the base is usually the part before /v1 (https://host/ai, not https://host/ai/v1/responses)"
	case strings.Contains(e, "did not answer") || strings.Contains(e, "no such host") || strings.Contains(e, "connection refused"):
		return "nothing answered at that host: a VPN or the address"
	case strings.Contains(e, "not json") || strings.Contains(e, "html"):
		return "the URL answered with a page, not an API: a login page or a proxy in front"
	}
	return ""
}

func resolveKeyForConnect(raw string) (string, string) {
	spec := providers.ResolveKey(raw)
	return spec.Value, spec.Source
}

func providerSpecOf(req connectRequest, api string) providers.ProviderSpec {
	spec := providers.ProviderSpec{ID: req.ID, Name: req.Name, API: api, Base: req.Base, Key: req.Key, Keyless: req.Keyless, Headers: req.Headers, Context: req.Context}
	if req.Model != "" {
		spec.Models = []providers.Model{{ID: req.Model, Name: req.Model, Context: req.Context}}
	}
	return spec
}

// connectDecisions probes a decision provider with one real yes/no — the
// only thing it answers — and keeps it. Decisions read it at the next
// gateway start (decisions.go systemOneFromEnv).
func connectDecisions(ctx context.Context, req connectRequest, key, source string) connectResult {
	res := connectResult{ID: req.ID, API: providers.APIDecisions}
	c, err := systemone.New(systemone.Config{BaseURL: req.Base, APIKey: key, Model: req.Model})
	if err != nil {
		res.Error = err.Error()
		return res
	}
	q, _ := decision.Boolean("Is the sky usually blue on a clear day?", "", "")
	pctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := c.Evaluate(pctx, "", decision.Request{State: "A connection test.", Questions: map[string]decision.Question{"q": q}})
	if err != nil || !out.OK() {
		why := string(out.Reason) + " " + out.Detail
		if err != nil {
			why = err.Error()
		}
		res.Error = oneLine(strings.TrimSpace(why), 200)
		res.Advice = "check the key (a TypeSafe key from typesafe.ai, or an OpenRouter key) and the URL"
		return res
	}
	res.OK, res.Tested = true, "jev"
	res.Answer = fmt.Sprintf("P(yes)=%.2f", out.Answers["q"].ProbabilityTrue)
	if req.Probe {
		return res
	}
	if err := providers.SaveProvider(providerSpecOf(req, providers.APIDecisions)); err != nil {
		res.OK, res.Error = false, "probed fine but could not save: "+err.Error()
		return res
	}
	res.Saved = true
	logs.New("Providers").Info("decision provider connected", "id", req.ID, "base", req.Base, "key_source", source)
	return res
}

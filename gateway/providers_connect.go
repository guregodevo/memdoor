package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/providers"
	"memdoor/pkg/decision"
	"memdoor/pkg/decision/systemone"
	"memdoor/pkg/llm"
)

// /connect (Greg, 2026-10-02, after reading how Cursor, Claude Code, Codex
// and omp do it): pick the kind of provider, give its base URL and token,
// and the gateway PROBES before it keeps anything — the provider's model
// list, then one tiny call — so a wrong URL or token fails here, in words,
// not in the middle of a turn. omp has no way to add a custom provider but
// a YAML file and never tests it; Claude Code tests with a curl the person
// runs by hand. Here it is one request: POST /api/providers/connect.

var providerIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

type connectRequest struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	API     string            `json:"api"`   // responses | chat | anthropic
	Base    string            `json:"base"`  // the provider's base URL
	Key     string            `json:"key"`   // literal, an env var NAME, or "!command"
	Model   string            `json:"model"` // a model to declare and test with (a gateway that lists none)
	Headers map[string]string `json:"headers"`
	Context int               `json:"context"`
	Probe   bool              `json:"probe"` // true: test only, keep nothing
}

type connectResult struct {
	OK      bool              `json:"ok"`
	ID      string            `json:"id"`
	API     string            `json:"api"`
	Models  int               `json:"models"`
	Sample  []string          `json:"sample,omitempty"`
	Tested  string            `json:"tested,omitempty"` // the model the test call used
	Answer  string            `json:"answer,omitempty"`
	Error   string            `json:"error,omitempty"`
	Advice  string            `json:"advice,omitempty"`
	Saved   bool              `json:"saved"`
	Headers map[string]string `json:"headers,omitempty"`
}

// handleProvidersConnect is POST /api/providers/connect and DELETE
// /api/providers/connect?id=.
func (s *Server) handleProvidersConnect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if err := providers.RemoveProvider(id); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "id": id})
		return
	case http.MethodPost:
	default:
		http.Error(w, `{"error": "POST or DELETE"}`, http.StatusMethodNotAllowed)
		return
	}
	var req connectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "a JSON body: id, api, base, key"}`, http.StatusBadRequest)
		return
	}
	// Always 200: a failed probe is an answer in words (error, advice), not
	// an HTTP error the client would have to unwrap.
	_ = json.NewEncoder(w).Encode(connectProvider(r.Context(), req))
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
	case req.Base == "" || !strings.HasPrefix(req.Base, "http"):
		res.Error = "the base URL must start with http:// or https://"
		return res
	case strings.TrimSpace(req.Key) == "":
		res.Error = "a token or key is needed: the value, the NAME of an environment variable that holds it, or !command that prints it"
		return res
	}
	key, source := resolveKeyForConnect(req.Key)
	if key == "" {
		res.Error = source
		return res
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
		p := providers.Provider{ID: req.ID, Name: req.Name, API: api, Base: req.Base, Key: key, Headers: req.Headers, Context: req.Context}
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
		answer, callErr := probeCall(ctx, p, model)
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
		return res
	}
	res.Error = oneLine(lastErr, 200)
	res.Advice = connectAdvice(lastErr, apis[len(apis)-1])
	return res
}

// probeCall is the one-token test: a reply, any reply, proves the URL, the
// token and the shape. An unknown-model error still proves the first two.
func probeCall(ctx context.Context, p providers.Provider, model string) (string, error) {
	ctx, cancel := context.WithTimeout(llm.WithUtilityCall(ctx), 60*time.Second)
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
	spec := providers.ProviderSpec{ID: req.ID, Name: req.Name, API: api, Base: req.Base, Key: req.Key, Headers: req.Headers, Context: req.Context}
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

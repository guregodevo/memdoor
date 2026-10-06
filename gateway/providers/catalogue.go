package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// EACH PROVIDER HAS A METADATA API (Greg, 2026-10-02: "check if each
// provider has an API that provides model metadata … let's add this in our
// provider interface, getModel"). Read on 2026-10-02, with keys:
//
//	Anthropic   GET /v1/models            max_input_tokens, max_tokens, capabilities (image, thinking, effort), line
//	Groq        GET /openai/v1/models     context_window, max_completion_tokens, pricing, input_modalities, active
//	OpenRouter  GET /api/v1/models        context_length, top_provider.max_completion_tokens, pricing, supported_parameters
//	Gemini      GET /v1beta/models?key=   inputTokenLimit, outputTokenLimit, supportedGenerationMethods, thinking
//	xAI         GET /v1/models            context_length, prices; GET /v1/language-models: input_modalities, aliases
//	OpenAI      GET /v1/models            id, owned_by, created — nothing else
//	DeepSeek    GET /v1/models            context_window, max_output_tokens, effort, input_modalities
//	Baseten     GET /v1/models            context_length, max_completion_tokens, pricing, modalities, supported_features
//
// A ModelReader is one of those, behind the registry: ModelsOf and GetModel
// call it, fill what it leaves empty from the catalogue and the reference
// (reference.go), and mark where every figure came from. OpenAI, the one
// vendor with no metadata, is why the fallbacks exist at all.

// ModelReader reads a provider's models as its own API states them.
type ModelReader interface {
	// ListModels is the provider's list; fields it does not state stay zero.
	ListModels(ctx context.Context, p Provider) ([]Model, error)
	// GetModel is one model's own entry, for a provider with a per-model
	// endpoint; the others answer from ListModels.
	GetModel(ctx context.Context, p Provider, id string) (Model, error)
}

// readerFor is the reader behind a provider: by id for the built-ins whose
// API has more to say than the OpenAI shape, else by wire.
func readerFor(p Provider) ModelReader {
	switch p.ID {
	case "anthropic":
		return anthropicReader{}
	case "groq":
		return groqReader{}
	case "gemini":
		return geminiReader{}
	case "xai":
		return xaiReader{}
	case "openrouter":
		return openRouterReader{}
	case "deepseek":
		return deepseekReader{}
	case "baseten":
		return basetenReader{}
	}
	switch p.API {
	case APIAnthropic:
		return anthropicReader{}
	case APIOpenRouter:
		return openRouterReader{}
	}
	return openAIShapeReader{}
}

// GetModel is one model as its provider states it: the cached list first,
// then the provider's per-model endpoint when it has one.
func GetModel(ctx context.Context, p Provider, id string) (Model, error) {
	if list, err := ModelsOf(ctx, p); err == nil {
		for _, m := range list {
			if m.ID == id {
				return m, nil
			}
		}
	}
	m, err := readerFor(p).GetModel(ctx, p, id)
	if err != nil {
		return Model{}, err
	}
	m.Provider = p.ID
	return m, nil
}

// getJSON reads a provider's endpoint into v with the headers the provider
// takes; a company's attribution headers travel too.
func getJSON(ctx context.Context, rawURL string, headers map[string]string, v any) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	for k, val := range headers {
		req.Header.Set(k, val)
	}
	setVendorHeaders(req.Header)
	setAttributionHeaders(ctx, req.Header)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("the model list did not answer: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("the model list answered HTTP %d: %s", resp.StatusCode, truncate(string(body), 160))
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("the model list was not JSON")
	}
	return nil
}

func bearer(p Provider) map[string]string {
	h := map[string]string{"Authorization": "Bearer " + p.Key}
	for k, v := range AppHeaders(p.Base) {
		h[k] = v
	}
	return h
}

func sortModels(out []Model) []Model {
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ---- the OpenAI shape: ids, and whatever extra fields a host adds -------

// openAIShapeReader reads GET <base>/models: {"data":[{"id":…}]}. OpenAI
// itself states nothing more; hosts that add context_length / max_model_len
// (vLLM, a gateway) are read for them.
type openAIShapeReader struct{}

func (openAIShapeReader) ListModels(ctx context.Context, p Provider) ([]Model, error) {
	var raw struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			DisplayName   string `json:"display_name"`
			ContextLength int    `json:"context_length"`
			MaxModelLen   int    `json:"max_model_len"`
		} `json:"data"`
	}
	if err := getJSON(ctx, modelsURL(p.Base), bearer(p), &raw); err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(raw.Data))
	for _, d := range raw.Data {
		if d.ID == "" {
			continue
		}
		name := d.DisplayName
		if name == "" {
			name = d.Name
		}
		if name == "" {
			name = d.ID
		}
		ctxLen := d.ContextLength
		if ctxLen == 0 {
			ctxLen = d.MaxModelLen
		}
		out = append(out, Model{ID: d.ID, Name: name, Context: ctxLen, Tools: true})
	}
	return sortModels(out), nil
}

func (r openAIShapeReader) GetModel(ctx context.Context, p Provider, id string) (Model, error) {
	return fromList(ctx, r, p, id)
}

func fromList(ctx context.Context, r ModelReader, p Provider, id string) (Model, error) {
	list, err := r.ListModels(ctx, p)
	if err != nil {
		return Model{}, err
	}
	for _, m := range list {
		if m.ID == id {
			return m, nil
		}
	}
	return Model{}, fmt.Errorf("%s does not list %s", p.Name, id)
}

// ---- Anthropic ---------------------------------------------------------

type anthropicReader struct{}

type anthropicModel struct {
	ID             string `json:"id"`
	DisplayName    string `json:"display_name"`
	MaxInputTokens int    `json:"max_input_tokens"`
	MaxTokens      int    `json:"max_tokens"`
	Line           string `json:"line"`
	Capabilities   struct {
		ImageInput struct {
			Supported bool `json:"supported"`
		} `json:"image_input"`
		Thinking struct {
			Supported bool `json:"supported"`
		} `json:"thinking"`
		Effort struct {
			Supported bool `json:"supported"`
		} `json:"effort"`
	} `json:"capabilities"`
}

func (m anthropicModel) model() Model {
	out := Model{ID: m.ID, Name: m.DisplayName, Context: m.MaxInputTokens, MaxOutput: m.MaxTokens, Tools: true, Inputs: []string{"text"}, Thinking: m.Capabilities.Thinking.Supported, Effort: m.Capabilities.Effort.Supported}
	if out.Name == "" {
		out.Name = m.ID
	}
	if m.Capabilities.ImageInput.Supported {
		out.Inputs = append(out.Inputs, "image")
	}
	return out
}

func anthropicHeaders(p Provider) map[string]string {
	return map[string]string{"x-api-key": p.Key, "anthropic-version": anthropicVersion}
}

func (anthropicReader) ListModels(ctx context.Context, p Provider) ([]Model, error) {
	var raw struct {
		Data []anthropicModel `json:"data"`
	}
	if err := getJSON(ctx, strings.TrimRight(p.Base, "/")+"/v1/models?limit=1000", anthropicHeaders(p), &raw); err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(raw.Data))
	for _, d := range raw.Data {
		if d.ID != "" {
			out = append(out, d.model())
		}
	}
	return sortModels(out), nil
}

func (anthropicReader) GetModel(ctx context.Context, p Provider, id string) (Model, error) {
	var d anthropicModel
	if err := getJSON(ctx, strings.TrimRight(p.Base, "/")+"/v1/models/"+url.PathEscape(id), anthropicHeaders(p), &d); err != nil {
		return Model{}, err
	}
	if d.ID == "" {
		return Model{}, fmt.Errorf("%s does not list %s", p.Name, id)
	}
	return d.model(), nil
}

// ---- Groq --------------------------------------------------------------

type groqReader struct{}

func (groqReader) ListModels(ctx context.Context, p Provider) ([]Model, error) {
	var raw struct {
		Data []struct {
			ID                  string   `json:"id"`
			Name                string   `json:"name"`
			Active              bool     `json:"active"`
			ContextWindow       int      `json:"context_window"`
			MaxCompletionTokens int      `json:"max_completion_tokens"`
			InputModalities     []string `json:"input_modalities"`
			Pricing             struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := getJSON(ctx, modelsURL(p.Base), bearer(p), &raw); err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(raw.Data))
	for _, d := range raw.Data {
		if d.ID == "" || (!d.Active && d.ContextWindow == 0) {
			continue
		}
		name := d.Name
		if name == "" {
			name = d.ID
		}
		out = append(out, Model{ID: d.ID, Name: name, Context: d.ContextWindow, MaxOutput: d.MaxCompletionTokens, Tools: true,
			Inputs: d.InputModalities, InPerM: perMillion(d.Pricing.Prompt), OutPerM: perMillion(d.Pricing.Completion)})
	}
	return sortModels(out), nil
}

func (r groqReader) GetModel(ctx context.Context, p Provider, id string) (Model, error) {
	return fromList(ctx, r, p, id)
}

// perMillion turns a per-token price string ("0.0000008") into dollars per
// million tokens.
func perMillion(perToken string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(perToken), 64)
	if err != nil {
		return 0
	}
	return f * 1_000_000
}

// ---- Gemini: the native API, not the OpenAI-shaped one ------------------

type geminiReader struct{}

// geminiNativeBase turns the OpenAI-shaped base (…/v1beta/openai) into the
// native one (…/v1beta); a base already native is kept.
func geminiNativeBase(base string) string {
	base = strings.TrimRight(base, "/")
	return strings.TrimSuffix(base, "/openai")
}

type geminiModel struct {
	Name             string   `json:"name"`
	DisplayName      string   `json:"displayName"`
	InputTokenLimit  int      `json:"inputTokenLimit"`
	OutputTokenLimit int      `json:"outputTokenLimit"`
	Methods          []string `json:"supportedGenerationMethods"`
	Thinking         bool     `json:"thinking"`
}

func (m geminiModel) model() Model {
	id := strings.TrimPrefix(m.Name, "models/")
	name := m.DisplayName
	if name == "" {
		name = id
	}
	return Model{ID: id, Name: name, Context: m.InputTokenLimit, MaxOutput: m.OutputTokenLimit, Tools: true, Thinking: m.Thinking, Inputs: []string{"text"}}
}

func (m geminiModel) generates() bool {
	for _, x := range m.Methods {
		if x == "generateContent" {
			return true
		}
	}
	return false
}

func (geminiReader) ListModels(ctx context.Context, p Provider) ([]Model, error) {
	var raw struct {
		Models []geminiModel `json:"models"`
	}
	if err := getJSON(ctx, geminiNativeBase(p.Base)+"/models?pageSize=1000&key="+url.QueryEscape(p.Key), nil, &raw); err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(raw.Models))
	for _, d := range raw.Models {
		if d.Name == "" || !d.generates() {
			continue
		}
		out = append(out, d.model())
	}
	return sortModels(out), nil
}

func (geminiReader) GetModel(ctx context.Context, p Provider, id string) (Model, error) {
	var d geminiModel
	if err := getJSON(ctx, geminiNativeBase(p.Base)+"/models/"+url.PathEscape(strings.TrimPrefix(id, "models/"))+"?key="+url.QueryEscape(p.Key), nil, &d); err != nil {
		return Model{}, err
	}
	if d.Name == "" {
		return Model{}, fmt.Errorf("%s does not list %s", p.Name, id)
	}
	return d.model(), nil
}

// ---- xAI ---------------------------------------------------------------

type xaiReader struct{}

func (xaiReader) ListModels(ctx context.Context, p Provider) ([]Model, error) {
	var raw struct {
		Data []struct {
			ID            string   `json:"id"`
			ContextLength int      `json:"context_length"`
			Aliases       []string `json:"aliases"`
		} `json:"data"`
	}
	if err := getJSON(ctx, modelsURL(p.Base), bearer(p), &raw); err != nil {
		return nil, err
	}
	// Modalities live on the other endpoint; read when it answers, skipped
	// when it does not (the window is the figure that matters).
	inputs := map[string][]string{}
	var lm struct {
		Models []struct {
			ID              string   `json:"id"`
			InputModalities []string `json:"input_modalities"`
		} `json:"models"`
	}
	if getJSON(ctx, strings.TrimSuffix(modelsURL(p.Base), "/models")+"/language-models", bearer(p), &lm) == nil {
		for _, m := range lm.Models {
			inputs[m.ID] = m.InputModalities
		}
	}
	out := make([]Model, 0, len(raw.Data))
	for _, d := range raw.Data {
		if d.ID == "" {
			continue
		}
		out = append(out, Model{ID: d.ID, Name: d.ID, Context: d.ContextLength, Tools: true, Inputs: inputs[d.ID]})
	}
	return sortModels(out), nil
}

func (r xaiReader) GetModel(ctx context.Context, p Provider, id string) (Model, error) {
	return fromList(ctx, r, p, id)
}

// ---- OpenRouter: the gateway's catalogue, with prices -------------------

type openRouterReader struct{}

func (openRouterReader) ListModels(ctx context.Context, p Provider) ([]Model, error) {
	if OpenRouterModels == nil {
		return nil, fmt.Errorf("no OpenRouter catalogue on this host")
	}
	return OpenRouterModels(p.Key)
}

func (r openRouterReader) GetModel(ctx context.Context, p Provider, id string) (Model, error) {
	return fromList(ctx, r, p, id)
}

// ---- DeepSeek ----------------------------------------------------------

type deepseekReader struct{}

func (deepseekReader) ListModels(ctx context.Context, p Provider) ([]Model, error) {
	var raw struct {
		Data []struct {
			ID              string   `json:"id"`
			Name            string   `json:"name"`
			ContextWindow   int      `json:"context_window"`
			MaxOutputTokens int      `json:"max_output_tokens"`
			Effort          any      `json:"effort"`
			InputModalities []string `json:"input_modalities"`
		} `json:"data"`
	}
	if err := getJSON(ctx, modelsURL(p.Base), bearer(p), &raw); err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(raw.Data))
	for _, d := range raw.Data {
		if d.ID == "" {
			continue
		}
		name := d.Name
		if name == "" {
			name = d.ID
		}
		out = append(out, Model{ID: d.ID, Name: name, Context: d.ContextWindow, MaxOutput: d.MaxOutputTokens, Tools: true,
			Inputs: d.InputModalities, Effort: d.Effort != nil, Thinking: d.Effort != nil})
	}
	return sortModels(out), nil
}

func (r deepseekReader) GetModel(ctx context.Context, p Provider, id string) (Model, error) {
	return fromList(ctx, r, p, id)
}

// ---- Baseten -----------------------------------------------------------

type basetenReader struct{}

func (basetenReader) ListModels(ctx context.Context, p Provider) ([]Model, error) {
	var raw struct {
		Data []struct {
			ID                  string   `json:"id"`
			Name                string   `json:"name"`
			ContextLength       int      `json:"context_length"`
			MaxCompletionTokens int      `json:"max_completion_tokens"`
			InputModalities     []string `json:"input_modalities"`
			Features            []string `json:"supported_features"`
			Pricing             struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := getJSON(ctx, modelsURL(p.Base), bearer(p), &raw); err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(raw.Data))
	for _, d := range raw.Data {
		if d.ID == "" {
			continue
		}
		name := d.Name
		if name == "" {
			name = d.ID
		}
		tools, reasoning := false, false
		for _, f := range d.Features {
			switch f {
			case "tools":
				tools = true
			case "reasoning":
				reasoning = true
			}
		}
		if !tools {
			continue // a model that cannot call a tool cannot run a turn
		}
		out = append(out, Model{ID: d.ID, Name: name, Context: d.ContextLength, MaxOutput: d.MaxCompletionTokens, Tools: true,
			Inputs: d.InputModalities, Thinking: reasoning, InPerM: perMillion(d.Pricing.Prompt), OutPerM: perMillion(d.Pricing.Completion)})
	}
	return sortModels(out), nil
}

func (r basetenReader) GetModel(ctx context.Context, p Provider, id string) (Model, error) {
	return fromList(ctx, r, p, id)
}

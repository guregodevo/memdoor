package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"memdoor/gateway/providers"
)

// The model catalogue for a gateway on the person's own OpenRouter key
// (providers/byok.go), read from OpenRouter itself, so `/model-search` and
// `memdoor model search` show the real list price per million tokens — the
// number they are choosing on (Greg, 2026-09-26: "show the real price").

type byokModel struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Context int     `json:"context"`
	Tools   bool    `json:"tools"`
	Band    string  `json:"band"`
	InPerM  float64 `json:"in_per_m"`
	OutPerM float64 `json:"out_per_m"`
	Created int64   `json:"created"`
	// Reasoning: the model takes OpenRouter's reasoning parameter.
	Reasoning bool `json:"reasoning,omitempty"`
	// MaxOutput: the top host's completion cap (top_provider.max_completion_tokens).
	MaxOutput int `json:"max_output,omitempty"`
}

type byokCatalog struct {
	mu   sync.Mutex
	at   time.Time
	list []byokModel
}

var theByokCatalog byokCatalog

// byokCatalogTTL keeps the listing for an hour: it is a few hundred models
// and it changes when a vendor ships, not between two searches.
const byokCatalogTTL = time.Hour

func (c *byokCatalog) models(key string) ([]byokModel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.at) < byokCatalogTTL && len(c.list) > 0 {
		return c.list, nil
	}
	if key == "" {
		// No OpenRouter key: nothing to ask it with, and on a company's
		// vendor key (providers/vendor.go) nothing may be asked at all.
		return nil, fmt.Errorf("no OpenRouter key: the catalogue is OpenRouter's")
	}
	req, err := http.NewRequest(http.MethodGet, providers.ByokModelsURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("the model catalogue did not answer: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("the model catalogue answered HTTP %d", resp.StatusCode)
	}
	var raw struct {
		Data []struct {
			ID            string   `json:"id"`
			Name          string   `json:"name"`
			ContextLength int      `json:"context_length"`
			Created       int64    `json:"created"`
			Params        []string `json:"supported_parameters"`
			Pricing       struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
			TopProvider struct {
				MaxCompletionTokens int `json:"max_completion_tokens"`
			} `json:"top_provider"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&raw); err != nil {
		return nil, err
	}
	out := make([]byokModel, 0, len(raw.Data))
	for _, m := range raw.Data {
		tools, reasoning := false, false
		for _, p := range m.Params {
			switch p {
			case "tools":
				tools = true
			case "reasoning":
				reasoning = true
			}
		}
		// A model that cannot call a tool cannot run an agent turn, and the
		// "~" ids are the decision models, not chat.
		if !tools || strings.HasPrefix(m.ID, "~") {
			continue
		}
		in, _ := strconv.ParseFloat(m.Pricing.Prompt, 64)
		outP, _ := strconv.ParseFloat(m.Pricing.Completion, 64)
		out = append(out, byokModel{
			ID: m.ID, Name: m.Name, Context: m.ContextLength, Tools: true,
			Band: byokBand(in), InPerM: in * 1e6, OutPerM: outP * 1e6, Created: m.Created, Reasoning: reasoning,
			MaxOutput: m.TopProvider.MaxCompletionTokens,
		})
	}
	c.list, c.at = out, time.Now()
	return out, nil
}

// byokBand is the glanceable price class of a price per token.
func byokBand(pricePerToken float64) string {
	perM := pricePerToken * 1e6
	switch {
	case perM <= 0:
		return "free"
	case perM < 0.30:
		return "$"
	case perM < 2:
		return "$$"
	default:
		return "$$$"
	}
}

// byokProvidersResponse answers /api/models/providers from OpenRouter's
// endpoints listing (providers.FetchEndpoints).
func byokProvidersResponse(key, id string) (map[string]any, error) {
	endpoints, err := providers.FetchEndpoints(key, id)
	if err != nil {
		return nil, err
	}
	type providerRow struct {
		Slug     string  `json:"slug"`
		Name     string  `json:"name"`
		Quant    string  `json:"quant"`
		Context  int     `json:"context"`
		Uptime   int     `json:"uptime_pct"`
		Band     string  `json:"band"`
		InPerM   float64 `json:"in_per_m"`
		OutPerM  float64 `json:"out_per_m"`
		Tools    bool    `json:"tools"`
		Excluded bool    `json:"excluded"`
	}
	rows := make([]providerRow, 0, len(endpoints))
	for _, e := range endpoints {
		in, _ := strconv.ParseFloat(e.Pricing.Prompt, 64)
		outP, _ := strconv.ParseFloat(e.Pricing.Completion, 64)
		tools := false
		for _, p := range e.SupportedParams {
			if p == "tools" {
				tools = true
				break
			}
		}
		name := e.ProviderName
		if name == "" {
			name = e.Name
		}
		rows = append(rows, providerRow{
			// The slug provider.order takes ("atlas-cloud"), not the
			// display name ("AtlasCloud") this used to send.
			Slug: e.Slug(), Name: name, Quant: e.Quantization, Context: e.ContextLength,
			Uptime: int(e.UptimeLast30m + 0.5), Band: byokBand(in),
			InPerM: in * 1e6, OutPerM: outP * 1e6, Tools: tools,
			// A host that drops the tool parameters cannot run a turn: the
			// request policy already excludes it (require_parameters).
			Excluded: !tools,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].InPerM < rows[j].InPerM })
	return map[string]any{"id": id, "providers": rows, "source": "openrouter (your key)"}, nil
}

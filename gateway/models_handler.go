package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"memdoor/gateway/providers"
)

// handleModels is GET /api/models?q=: the model catalogue for the window's
// /model-search and `memdoor model search`, read from the connected providers
// themselves (providers/registry.go): each with its own list, its real prices
// and where each figure came from. Never from the broker — no seat supplies a
// key (2026-10-04) — so with none connected it answers 409 and the step that
// adds one.
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "GET only"}`, http.StatusMethodNotAllowed)
		return
	}
	limit := 40
	if r.URL.Query().Get("limit") == "0" {
		limit = 0 // every model: the audit reads whole lists
	}
	if out, ok := providerModelsResponse(r.Context(), r.URL.Query().Get("q"), limit); ok {
		// Providers, each with its list (providers/registry.go): the company
		// gateway, Anthropic, OpenAI, Gemini, OpenRouter with its prices, and
		// the ones /connect added. Grouped, and flat for the picker.
		_ = json.NewEncoder(w).Encode(out)
		return
	}
	// Nothing connected: there is no catalogue to read — no seat supplies a
	// key (2026-10-04) — only the step that adds one.
	http.Error(w, `{"error": "no provider connected: memdoor connect (or export a provider's key)"}`, http.StatusConflict)
}

// handleModelProviders is GET /api/models/providers?id=: a model's hosts,
// so the person can prefer or reorder them (/model <id> order a,b).
func (s *Server) handleModelProviders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "GET only"}`, http.StatusMethodNotAllowed)
		return
	}
	if key := providers.ByokKey(); key != "" {
		out, err := byokProvidersResponse(key, r.URL.Query().Get("id"))
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(out)
		return
	}
	http.Error(w, `{"error": "a model's hosts are read on an OpenRouter key: memdoor connect openrouter"}`, http.StatusConflict)
}

// providerModelsResponse is /api/models when at least one provider is
// connected: {"providers":[{id,name,api,connected,active,key_source,models}],
// "models":[every connected provider's matches, with provider]}. False when
// none is, and the handler then answers 409 with the step that connects one.
func providerModelsResponse(ctx context.Context, q string, limit int) (map[string]any, bool) {
	active := providers.ActiveProviderID()
	var groups []map[string]any
	var flat []providers.Model
	connected := false
	byProvider := false
	for _, p := range providers.Providers() {
		if strings.EqualFold(strings.TrimSpace(q), p.ID) && p.Connected() {
			byProvider = true
		}
	}
	for _, p := range providers.Providers() {
		g := map[string]any{"id": p.ID, "name": p.Name, "api": p.API, "connected": p.Connected(), "active": p.ID == active, "key_source": p.KeySource}
		if p.Connected() {
			connected = true
			if list, err := providers.ModelsOf(ctx, p); err != nil {
				g["error"] = err.Error()
			} else {
				g["count"] = len(list)
				switch {
				case strings.EqualFold(strings.TrimSpace(q), p.ID):
					// The query names the provider: its whole list, as
					// /model-search deepseek after /connect deepseek.
					g["models"] = list
					flat = append(flat, list...)
				case byProvider:
					g["models"] = []providers.Model{}
				default:
					list = providers.SearchModels(list, q, limit)
					g["models"] = list
					flat = append(flat, list...)
				}
			}
		}
		groups = append(groups, g)
	}
	if !connected {
		return nil, false
	}
	// The flat list the picker reads is one row per model id: two providers
	// listing the same id (a proxy in front of the same catalogue) would
	// show it twice with nothing to tell them apart. The grouped answer
	// above keeps both.
	seen := map[string]bool{}
	dedup := flat[:0]
	for _, m := range flat {
		if !seen[m.ID] {
			seen[m.ID] = true
			dedup = append(dedup, m)
		}
	}
	return map[string]any{"providers": groups, "models": dedup, "source": "your providers"}, true
}

// handleProviders is GET /api/providers: every provider, connected or not,
// never a key. POST /api/providers?refresh=1 drops every cached list and
// the reference catalogue so the next read is fresh (a vendor shipped).
func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodPost && r.URL.Query().Get("refresh") == "1" {
		providers.ForgetAllModels()
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "refreshed": true})
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "GET only"}`, http.StatusMethodNotAllowed)
		return
	}
	active := providers.ActiveProviderID()
	type row struct {
		providers.Provider
		Connected bool `json:"connected"`
		Active    bool `json:"active"`
	}
	var out []row
	for _, p := range providers.Providers() {
		out = append(out, row{Provider: p, Connected: p.Connected(), Active: p.ID == active})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"providers": out})
}

package providers

import (
	"context"
	"encoding/json"
	"io"
	"memdoor/pkg/shared"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// THE REFERENCE CATALOGUE (Greg, 2026-10-02: "make sure you have the
// correct context size; if we can get it dynamically that's better"). A
// provider that states a model's window in its own list is believed
// (OpenRouter's context_length, Groq's context_window). Anthropic, OpenAI
// and Gemini list ids only, so their windows come from models.dev — the
// public catalogue every coding agent reads (omp bundles it): per provider,
// per model, `limit.context` and `limit.output`. Read at most once a day,
// kept on disk so an offline or company-key gateway reads its last copy
// and never contacts the host; a model neither source names gets the
// provider's conservative default.

const (
	modelsDevTTL  = 24 * time.Hour
	modelsDevFile = "models-dev.json"
)

// modelsDevURL is a variable so a test can serve its own copy.
var modelsDevURL = "https://models.dev/api.json"

// modelsDevProvider maps a registry id to models.dev's provider key.
var modelsDevProvider = map[string]string{
	"anthropic": "anthropic", "openai": "openai", "gemini": "google", "groq": "groq",
	"xai": "xai", "baseten": "baseten", "openrouter": "openrouter", "deepseek": "deepseek",
}

// Limits is what the reference knows about a model.
type Limits struct {
	Context int
	Output  int
	InPerM  float64 // the vendor's list price per million input tokens, when stated
	OutPerM float64
}

type modelsDevDoc map[string]struct {
	Models map[string]struct {
		Limit struct {
			Context int `json:"context"`
			Output  int `json:"output"`
		} `json:"limit"`
		Cost struct {
			Input  float64 `json:"input"`
			Output float64 `json:"output"`
		} `json:"cost"`
	} `json:"models"`
}

var modelsDev = struct {
	mu   sync.Mutex
	at   time.Time
	doc  modelsDevDoc
	seen bool
}{}

func modelsDevPath() (string, error) {
	return shared.MemdoorHome(modelsDevFile), nil
}

// referenceDoc is the catalogue: memory, then the disk copy when fresh
// enough or when nothing may be fetched, then the host.
func referenceDoc(ctx context.Context) modelsDevDoc {
	modelsDev.mu.Lock()
	defer modelsDev.mu.Unlock()
	if modelsDev.doc != nil && time.Since(modelsDev.at) < modelsDevTTL {
		return modelsDev.doc
	}
	path, _ := modelsDevPath()
	var disk modelsDevDoc
	var diskAt time.Time
	if path != "" {
		if st, err := os.Stat(path); err == nil {
			if b, err := os.ReadFile(path); err == nil && json.Unmarshal(b, &disk) == nil {
				diskAt = st.ModTime()
			}
		}
	}
	if disk != nil && (time.Since(diskAt) < modelsDevTTL || VendorMode()) {
		modelsDev.doc, modelsDev.at = disk, diskAt
		return disk
	}
	if VendorMode() {
		// Nothing but the company's endpoint is contacted; no copy yet means
		// the defaults until one is made outside that mode.
		return disk
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsDevURL, nil)
	if err != nil {
		return disk
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return disk
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil || resp.StatusCode != http.StatusOK {
		return disk
	}
	var fresh modelsDevDoc
	if json.Unmarshal(body, &fresh) != nil || len(fresh) == 0 {
		return disk
	}
	if path != "" {
		_ = os.MkdirAll(filepath.Dir(path), 0o700)
		_ = os.WriteFile(path, body, 0o600)
	}
	modelsDev.doc, modelsDev.at = fresh, time.Now()
	return fresh
}

// dateSuffix: a dated id — claude-haiku-4-5-20251001, gpt-4o-mini-2024-07-18
// — matched to its undated entry.
var dateSuffix = regexp.MustCompile(`-(\d{8}|\d{4}-\d{2}-\d{2})$`)

// ReferenceLimits is a model's window and output cap as models.dev states
// them for its provider: the exact id, else the id without a date suffix
// (claude-haiku-4-5-20251001 → claude-haiku-4-5), else the id without a
// vendor prefix. False when the catalogue has nothing.
func ReferenceLimits(ctx context.Context, providerID, modelID string) (Limits, bool) {
	key, ok := modelsDevProvider[providerID]
	if !ok {
		return Limits{}, false
	}
	doc := referenceDoc(ctx)
	if doc == nil {
		return Limits{}, false
	}
	prov, ok := doc[key]
	if !ok {
		return Limits{}, false
	}
	bare := dateSuffix.ReplaceAllString(modelID[strings.LastIndex(modelID, "/")+1:], "")
	for _, id := range []string{modelID, dateSuffix.ReplaceAllString(modelID, ""), bare} {
		if m, ok := prov.Models[id]; ok && m.Limit.Context > 0 {
			return Limits{Context: m.Limit.Context, Output: m.Limit.Output, InPerM: m.Cost.Input, OutPerM: m.Cost.Output}, true
		}
	}
	// No family guessing (gpt-5.1-codex-max → gpt-5.1): a model the
	// catalogue does not name is unknown, said so (Greg, 2026-10-02: "we
	// should call an API for context_size").
	return Limits{}, false
}

// ReferenceAge is how old the on-disk reference catalogue is, and whether
// there is one at all — the startup check says so when it is a week old
// and could not be refreshed, since every window read from it ages with it.
func ReferenceAge() (time.Duration, bool) {
	path, err := modelsDevPath()
	if err != nil {
		return 0, false
	}
	st, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	return time.Since(st.ModTime()), true
}

// catalogueVendor maps a registry id to OpenRouter's vendor prefix.
var catalogueVendor = map[string]string{"anthropic": "anthropic", "openai": "openai", "gemini": "google", "xai": "x-ai", "deepseek": "deepseek"}

// normalizeModelID makes "claude-haiku-4-5-20251001" and "claude-haiku-4.5"
// the same word: lower case, no date suffix, dots as dashes.
func normalizeModelID(id string) string {
	id = strings.ToLower(dateSuffix.ReplaceAllString(id, ""))
	return strings.ReplaceAll(id, ".", "-")
}

// CatalogueLimits is a vendor's model as OpenRouter's catalogue lists it
// (anthropic/claude-haiku-4.5, openai/gpt-5-codex: context_length and the
// completion cap) — an API, read on the person's key. False without a key
// or when the catalogue has no such model.
func CatalogueLimits(providerID, modelID string) (Limits, bool) {
	vendor, ok := catalogueVendor[providerID]
	if !ok || OpenRouterModels == nil || ByokKey() == "" {
		return Limits{}, false
	}
	list, err := OpenRouterModels(ByokKey())
	if err != nil {
		return Limits{}, false
	}
	want := vendor + "/" + normalizeModelID(modelID)
	for _, m := range list {
		if normalizeModelID(m.ID) == want && m.Context > 0 {
			return Limits{Context: m.Context, Output: m.MaxOutput}, true
		}
	}
	return Limits{}, false
}

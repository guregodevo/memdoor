package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"memdoor/gateway/logs"
)

// Hosts on OpenRouter: one model is served by many (DeepSeek V4.1 Flash by
// ~30), each with its own prompt cache. With sort=price every request picked
// again, and consecutive requests of one conversation landed on different
// hosts: a switch re-sends the whole prefix at the full price. Measured
// 2026-09-30 on 8 coder tasks: 27% of the uncached input was a turn's first
// request missing entirely, 22% a re-sent prefix missing (AtlasCloud →
// Together → AtlasCloud within a turn); a cached token costs 2% of an
// uncached one on that model ($0.006 vs $0.30 per million).
//
// So the host that served a model is asked first for that model's next
// requests (order + allow_fallbacks: a host that is down still falls back to
// the cheapest), gateway-wide so the shared system prompt and tools stay warm
// across conversations too. It is forgotten after stickyHostFor, so a cheaper
// host is picked up again, and replaced whenever a fallback answers instead.
// A person's own order (/model … order) always wins.
//
// A single other host answering is NOT taken as a move: under parallel load
// one host falling back would re-pin every conversation at once and throw
// away all their prompt caches. A different host must answer twice in a row
// for the model before the pin moves.

const stickyHostFor = time.Hour

// Endpoint is one host serving a model, as OpenRouter lists it.
type Endpoint struct {
	Name            string   `json:"name"`
	ProviderName    string   `json:"provider_name"`
	Tag             string   `json:"tag"`
	Quantization    string   `json:"quantization"`
	ContextLength   int      `json:"context_length"`
	SupportedParams []string `json:"supported_parameters"`
	UptimeLast30m   float64  `json:"uptime_last_30m"`
	Pricing         struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
}

// Slug is the provider slug OpenRouter's provider.order takes ("atlas-cloud"
// for AtlasCloud): the tag without its variant ("atlas-cloud/fp8").
func (e Endpoint) Slug() string {
	if s, _, _ := strings.Cut(e.Tag, "/"); s != "" {
		return s
	}
	return strings.ToLower(e.ProviderName)
}

// FetchEndpoints lists the hosts serving model id (vendor/name).
func FetchEndpoints(key, id string) ([]Endpoint, error) {
	author, slug, ok := strings.Cut(strings.TrimSpace(id), "/")
	if !ok || author == "" || slug == "" {
		return nil, fmt.Errorf("a model id looks like vendor/name, not %q", id)
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://openrouter.ai/api/v1/models/%s/%s/endpoints", author, slug), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("the host listing did not answer: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("the host listing answered HTTP %d", resp.StatusCode)
	}
	var raw struct {
		Data struct {
			Endpoints []Endpoint `json:"endpoints"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&raw); err != nil {
		return nil, err
	}
	return raw.Data.Endpoints, nil
}

type stickyEntry struct {
	slug   string    // the host asked first
	at     time.Time // when it was pinned (forgotten stickyHostFor later)
	cand   string    // a different host answering, one answer in
	streak int       // consecutive answers from cand
}

var sticky = struct {
	sync.Mutex
	hosts   map[string]stickyEntry       // model → the host asked first
	slugs   map[string]map[string]string // model → provider name → slug
	fetched map[string]time.Time
}{hosts: map[string]stickyEntry{}, slugs: map[string]map[string]string{}, fetched: map[string]time.Time{}}

// fetchEndpoints is FetchEndpoints, a variable so a test serves the listing.
var fetchEndpoints = FetchEndpoints

// stickyHost is the host to ask first for model, "" when none is known.
func stickyHost(model string) string {
	sticky.Lock()
	defer sticky.Unlock()
	if e, ok := sticky.hosts[model]; ok && time.Since(e.at) < stickyHostFor {
		return e.slug
	}
	return ""
}

// noteServedHost records that providerName answered for model. The name →
// slug lookup is one listing per model per stickyHostFor; a name it cannot
// resolve pins nothing (an order OpenRouter does not know is silently
// ignored, which would look like a pin and do nothing).
func noteServedHost(key, model, providerName string) {
	if providerName == "" || model == "" {
		return
	}
	sticky.Lock()
	names, fresh := sticky.slugs[model], time.Since(sticky.fetched[model]) < stickyHostFor
	sticky.Unlock()
	if !fresh {
		eps, err := fetchEndpoints(key, model)
		if err != nil {
			logs.New("Remote").Debug("host listing unavailable; not pinning", "model", model, "error", err.Error())
			return
		}
		names = map[string]string{}
		for _, e := range eps {
			names[e.ProviderName] = e.Slug()
		}
		sticky.Lock()
		sticky.slugs[model], sticky.fetched[model] = names, time.Now()
		sticky.Unlock()
	}
	slug := names[providerName]
	if slug == "" {
		return
	}
	sticky.Lock()
	defer sticky.Unlock()
	prev, had := sticky.hosts[model]
	switch {
	case !had || time.Since(prev.at) >= stickyHostFor:
		sticky.hosts[model] = stickyEntry{slug: slug, at: time.Now()}
	case prev.slug == slug:
		prev.cand, prev.streak = "", 0
		sticky.hosts[model] = prev
	case prev.cand != slug:
		prev.cand, prev.streak = slug, 1
		sticky.hosts[model] = prev
	default:
		prev.streak++
		if prev.streak >= 2 {
			sticky.hosts[model] = stickyEntry{slug: slug, at: time.Now()}
		} else {
			sticky.hosts[model] = prev
		}
	}
}

// withStickyHost is the request's provider policy with the model's host
// asked first — unless the policy already names an order.
func withStickyHost(policy map[string]any, model string) map[string]any {
	if policy == nil {
		return nil
	}
	if _, ordered := policy["order"]; ordered {
		return policy
	}
	host := stickyHost(model)
	if host == "" {
		return policy
	}
	out := make(map[string]any, len(policy)+2)
	for k, v := range policy {
		out[k] = v
	}
	out["order"] = []string{host}
	out["allow_fallbacks"] = true
	return out
}

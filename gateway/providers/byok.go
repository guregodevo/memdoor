package providers

import (
	"context"
	"net/http"
	"os"
	"strings"

	"memdoor/pkg/attribution"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

// BYOK — the person's own OpenRouter key serves their turns.
//
// Greg, 2026-09-27: "it should be BYOK with openrouter api key … they bring
// their own open router key, memdoor cuts the bill". The product is the layer,
// not the tokens: a solo developer already pays a provider, and the decision
// model makes those tokens go further (docs/features/DECIDE.md). So when the gateway's environment carries an
// OpenRouter key, chat goes STRAIGHT to OpenRouter on that key — nothing of
// the person's code passes through memdoor.ai.
//
// Decided here:
//   - the ladder per agent (byokLadders)
//   - the request's provider policy (byokProviderPolicy): never a host that
//     trains on what it is sent, only hosts that honour every parameter, and
//     the cheapest such host unless the person ordered otherwise
const (
	ByokEngineName = "openrouter-byok"
	byokEndpoint   = "https://openrouter.ai/api/v1/chat/completions"
	byokModelsURL  = "https://openrouter.ai/api/v1/models"
)

// ByokKey is the person's OpenRouter key, or "" when there is none. The same
// key the decision model uses (ADR-0015) — one key for the whole product.
func ByokKey() string {
	for _, name := range []string{"OPEN_ROUTER_API_KEY", "OPENROUTER_API_KEY"} {
		if k := strings.TrimSpace(os.Getenv(name)); k != "" {
			return k
		}
	}
	return ""
}

// WebSearch is where the web_search tool goes: OpenRouter, on the person's
// key, answered by the cheapest rung of the coder's ladder (the search
// itself is OpenRouter's server tool, the model only answers from it).
func WebSearch() tools.WebSearchBackend {
	return tools.WebSearchBackend{Endpoint: byokEndpoint, Key: ByokKey(), Model: byokLadders["coder"][0], Headers: AppHeaders(byokEndpoint)}
}

// byokLadders is each agent's ladder, cheapest rung first. A conversation
// starts on rung 1 and `/model` pins any rung or any catalogue model; the
// prices quoted are the hosts' own, per million tokens.
var byokLadders = map[string][]string{
	// GLM 5.3 Flash → DeepSeek V4.1 Flash → GLM 5.3. No price is written
	// here: hosts change theirs daily (`memdoor model providers <id>`).
	"coder": {"z-ai/glm-5.3-flash", "deepseek/deepseek-v4.1-flash", "z-ai/glm-5.3"},
}

// byokDefaultModel answers for an agent with no ladder of its own.
const byokDefaultModel = "z-ai/glm-5.3-flash"

// byokCtxLen is the window a turn is measured against, well under every rung's own
// (1M–1.3M on the catalogue, 2026-09-27). Left at zero, the turn fell back to
// the local 32K profile and compacted twice in one review.
const byokCtxLen = 262_144

// ByokEngine is the engine for a gateway holding its own key, and whether
// there is one at all.
func ByokEngine() (RemoteEngine, bool) {
	key := ByokKey()
	if key == "" {
		return RemoteEngine{}, false
	}
	models := map[string]string{}
	for agent, ladder := range byokLadders {
		if len(ladder) > 0 {
			models[agent] = ladder[0] // the rung a conversation starts on
		}
	}
	return RemoteEngine{
		Name:         ByokEngineName,
		Endpoint:     byokEndpoint,
		APIKey:       key,
		Model:        byokDefaultModel,
		Byok:         true,
		CtxLen:       byokCtxLen,
		AgentModels:  models,
		AgentLadders: byokLadders,
	}, true
}

// byokProviderPolicy is OpenRouter's top-level "provider" block for a turn:
// the privacy and correctness floor for the person's own key, plus their
// host preference when they set one with
// `/model <id> [price|throughput|latency] [order a,b]`.
func byokProviderPolicy(ctx context.Context, model string) map[string]any {
	p := map[string]any{
		// Never a host that may train on what it is sent: the person's code
		// goes through here. Sent per request, not left to an account toggle.
		// THE ONE EXCEPTION IS A FREE MODEL (Greg, 2026-10-10: "for free we
		// allow data collection"): every `:free` endpoint on OpenRouter is
		// served on the condition that it may train on what it is sent, and
		// with deny they all answer "no endpoints match your data policy"
		// (live, 16 of 16). A person who pins one has chosen a trial over
		// privacy; the pin says so (cmd/cli/cmd/model_pin_warning.go).
		"data_collection": dataCollectionFor(model),
		// Only a host that supports every parameter sent (tools, tool_choice):
		// one that silently drops them answers wrong.
		"require_parameters": true,
		// Never a host that serves the model below fp8. Sorted by price, GLM
		// 5.3 Flash went first to a 4-bit host ($0.020/M against $0.15
		// list): 3 of 3 turns there failed — "file reads come back
		// truncated" on whole files, word salad, an edit announced and never
		// made — and 1 of 1 passed on an fp8 host (2026-09-30). Hosts that
		// publish no precision stay in: leaving them out halved DeepSeek's
		// hosts and quadrupled its cheapest price, with nothing against them.
		"quantizations": byokQuantizations,
	}
	if IsFreeModel(model) {
		// A free model is served as its host offers it: the only host of
		// nvidia/nemotron-3.5-lightning:free is nvfp4, and the floor refused
		// it ("No endpoints found for the request with quantization", live
		// 2026-10-10). The pin notice says so.
		delete(p, "quantizations")
	}
	sort, _ := ctx.Value(sharedctx.SortKey).(string)
	order, _ := ctx.Value(sharedctx.OrderKey).(string)
	if o := splitHosts(order); len(o) > 0 {
		p["order"] = o
		p["allow_fallbacks"] = true
	}
	switch sort {
	case "throughput", "latency", "price":
		p["sort"] = sort
	case "default":
		// OpenRouter's own pick: no sort at all.
	default:
		if _, ordered := p["order"]; !ordered {
			// The cheapest host that meets the policy. Left to the default,
			// a live pin of z-ai/glm-5.3 went to a $0.66/M host with a
			// $0.38/M one available (2026-09-26).
			p["sort"] = "price"
		}
	}
	return p
}

// IsFreeModel reports whether an OpenRouter model id is a free one: the
// `:free` variants and the free-models router. Free endpoints train on what
// they are sent; nothing else about them is different.
func IsFreeModel(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	return strings.HasSuffix(id, ":free") || id == "openrouter/free"
}

// dataCollectionFor is the request's data policy: deny, except for a free
// model, which exists only on hosts that train.
func dataCollectionFor(model string) string {
	if IsFreeModel(model) {
		return "allow"
	}
	return "deny"
}

// byokQuantizations is OpenRouter's provider.quantizations allow list: fp8 and
// better, and hosts that do not say.
var byokQuantizations = []string{"int8", "fp8", "fp16", "bf16", "fp32", "unknown"}

func splitHosts(order string) []string {
	var out []string
	for _, h := range strings.Split(order, ",") {
		if h = strings.TrimSpace(h); h != "" {
			out = append(out, h)
		}
	}
	return out
}

// ByokModelsURL is where the catalogue comes from on the person's OpenRouter key.
func ByokModelsURL() string { return byokModelsURL }

// APP ATTRIBUTION. OpenRouter credits a request to an app by two headers, and
// its public ranking (openrouter.ai/rankings) is built from them. Without them
// a person's turns count for nobody, which is how it was until 2026-09-27: the
// ranking is exactly where this audience compares tools, so the app that makes
// their tokens go further should appear on it.
//
// The headers carry the app's name and site, nothing about the person, their
// code or their prompt — and OpenRouter already sees the traffic either way.
// MEMDOOR_OPENROUTER_ATTRIBUTION=0 leaves them off for someone who would rather
// not be counted.
// The canonical name and site live in pkg/attribution, imported by everything
// that talks to OpenRouter (chat and decisions): two spellings
// would split us across two rows in their ranking.
const (
	OpenRouterAppName = attribution.AppName
	OpenRouterAppURL  = attribution.AppURL
)

// SetOpenRouterAttribution names this app on a request bound for OpenRouter.
func SetOpenRouterAttribution(h http.Header) { attribution.Set(h) }

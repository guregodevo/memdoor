package providers

import (
	"context"
	"sort"
	"strings"
	"time"
)

// A FREE ACCOUNT WORKS ON ITS FIRST TURN (2026-10-31). Sign in with OpenRouter
// makes an account with no credit; the ladder's models are paid, so its first
// turn would answer "out of credit". aider asks OpenRouter whether the key is
// on the free tier and picks a free model; so does this, from OpenRouter's own
// list rather than a name written here (a window, a price or a model is data,
// never a constant). A free model's hosts may train on what they are sent
// (the maintainer, 2026-10-10: "for free we allow data collection"), so when the tier
// cannot be read the paid ladder stays: a private turn that asks for credit
// beats a public one nobody chose.

// openRouterKeyURL answers what OpenRouter knows of a key.
var openRouterKeyURL = "https://openrouter.ai/api/v1/key"

// freeTierTimeout bounds the question: it is asked when the engine is chosen
// (the gateway's start, a connect), and an offline start must not wait long.
var freeTierTimeout = 4 * time.Second

// OpenRouterFreeTier says whether the key's account never bought credit, and
// whether OpenRouter said so at all: known is false offline, on a refused key
// or on an answer without the field.
func OpenRouterFreeTier(ctx context.Context, key string) (free, known bool) {
	if strings.TrimSpace(key) == "" {
		return false, false
	}
	ctx, cancel := context.WithTimeout(ctx, freeTierTimeout)
	defer cancel()
	headers := AppHeaders(openRouterKeyURL)
	headers["Authorization"] = "Bearer " + key
	var out struct {
		Data struct {
			IsFreeTier *bool `json:"is_free_tier"`
		} `json:"data"`
	}
	if err := getJSON(ctx, openRouterKeyURL, headers, &out); err != nil || out.Data.IsFreeTier == nil {
		return false, false
	}
	return *out.Data.IsFreeTier, true
}

// freeModelSuffix marks a free model on OpenRouter.
const freeModelSuffix = ":free"

// FreeTierModel is the model a free account's turns run on: of the list, the
// free models that take tools (the coder calls them), the strongest first by
// the one measure the list gives, the list price of its paid twin (the same id
// without ":free"; live 2026-10-31 that ranks inkling:free, twin $4.05/M out,
// above inkling-small:free, $1.20/M, which a context-then-id order picked by
// the "-" sorting first), then the longer context, then the id so the choice
// is stable; "" when there is none.
func FreeTierModel(list []Model) string {
	twin := map[string]float64{}
	var free []Model
	for _, m := range list {
		if strings.HasSuffix(m.ID, freeModelSuffix) {
			if m.Tools {
				free = append(free, m)
			}
			continue
		}
		twin[m.ID] = m.OutPerM
	}
	if len(free) == 0 {
		return ""
	}
	strength := func(m Model) float64 { return twin[strings.TrimSuffix(m.ID, freeModelSuffix)] }
	sort.Slice(free, func(i, j int) bool {
		if si, sj := strength(free[i]), strength(free[j]); si != sj {
			return si > sj
		}
		if free[i].Context != free[j].Context {
			return free[i].Context > free[j].Context
		}
		return free[i].ID < free[j].ID
	})
	return free[0].ID
}

// OnFreeTier is the engine with every agent and every rung on one model:
// a free account has no paid rung to climb to.
func (re RemoteEngine) OnFreeTier(model string) RemoteEngine {
	re.Model = model
	re.FreeTier = true
	agents := map[string]string{}
	ladders := map[string][]string{}
	for agent := range re.AgentModels {
		agents[agent] = model
	}
	for agent := range re.AgentLadders {
		ladders[agent] = []string{model}
	}
	re.AgentModels, re.AgentLadders = agents, ladders
	return re
}

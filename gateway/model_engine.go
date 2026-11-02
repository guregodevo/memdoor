package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/providers"
)

// Which model serves this gateway, at start: a company's vendor key, else
// the person's own OpenRouter key. Nothing is leased from memdoor.ai's broker
// any more — no seat supplies a model key (the maintainer, 2026-10-04: "Pro seat never
// offer key. It's always with user key"; the MEMDOOR_BRAIN_CLASS and
// MEMDOOR_BYOK=0 switches that reached the lease were removed the same day).

// warnStaleReference says at startup when the model reference catalogue on
// disk is a week old or more and nothing newer could be read: every window
// taken from it is that old.
func warnStaleReference() {
	if age, ok := providers.ReferenceAge(); ok && age > 7*24*time.Hour {
		logs.New("Providers").Warn(fmt.Sprintf("the model reference catalogue (~/.memdoor/models-dev.json) is %d days old; context windows read from it may be stale — `memdoor providers --refresh` when the network allows", int(age.Hours()/24)))
	}
}

func (s *Server) selectModelEngine() {
	warnStaleReference()
	// A COMPANY'S VENDOR KEY FIRST (providers/vendor.go): every turn goes to
	// the vendor the company approved, on the key it gave the developer, and
	// nothing reaches OpenRouter or memdoor.ai's broker. The one mode a
	// monitored laptop allows (the maintainer, 2026-10-02).
	if re, ok, reason := providers.VendorEngine(); ok {
		cur := providers.ActiveRemoteEngine()
		if cur == nil || cur.Vendor != re.Vendor || cur.APIKey != re.APIKey || cur.Endpoint != re.Endpoint || cur.Model != re.Model || cur.CtxLen != re.CtxLen {
			if err := providers.SetRemoteEngine(re); err != nil {
				logs.New("Brain").Warn("your company's " + re.Vendor + " key could not be used: " + err.Error())
			} else {
				logs.New("Brain").Info("your company's "+re.Vendor+" key serves this gateway; nothing else is contacted",
					slog.String("endpoint", re.Endpoint), slog.String("model", re.Model))
			}
		}
		return
	} else if reason != "" {
		logs.New("Brain").Warn(reason)
	}
	// A SIGNED-IN CHATGPT PLAN NEXT (providers/chatgpt_engine.go): the person
	// signed in to spend their plan, so it answers before their OpenRouter
	// key; the decision model and web search still run on that key.
	ctx, cancel := context.WithTimeout(context.Background(), planListTimeout)
	plan, onPlan := chatGPTPlanEngine(ctx)
	cancel()
	if onPlan {
		cur := providers.ActiveRemoteEngine()
		if cur == nil || !cur.IsChatGPTPlan() || cur.Model != plan.Model {
			if err := providers.SetRemoteEngine(plan); err != nil {
				logs.New("Brain").Warn("the ChatGPT plan could not be used: " + err.Error())
			} else {
				logs.New("Brain").Info("your ChatGPT plan serves this gateway", slog.String("model", plan.Model))
			}
		}
		return
	}
	// BYOK FIRST (the maintainer, 2026-09-27: "it should be BYOK with openrouter api
	// key"). A gateway holding the person's own OpenRouter key needs no seat,
	// no lease and no heartbeat: their turns go straight to OpenRouter on
	// their key, with the ladder and the provider policy resolved in the
	// binary (providers/byok.go). Nothing of their code reaches memdoor.ai:
	// they are paying their own bill.
	if re, ok := providers.ByokEngine(); ok {
		re = onFreeTierIfFree(re)
		cur := providers.ActiveRemoteEngine()
		if cur == nil || !cur.Byok || cur.APIKey != re.APIKey || cur.CtxLen != re.CtxLen || cur.Model != re.Model {
			if err := providers.SetRemoteEngine(re); err != nil {
				logs.New("Brain").Warn("your own OpenRouter key could not be used: " + err.Error())
			} else {
				logs.New("Brain").Info("your own OpenRouter key serves this gateway",
					slog.String("first_rung", re.ModelForTier("coder", 0)), slog.Bool("free_tier", re.FreeTier))
			}
		}
		return
	}
	// The OpenRouter key is gone (`memdoor connect --remove openrouter`), or
	// the plan signed out: the engine it chose must not keep answering on it.
	if cur := providers.ActiveRemoteEngine(); cur != nil && (cur.Byok || cur.IsChatGPTPlan()) {
		_ = providers.ClearRemoteEngine()
		logs.New("Brain").Info("no OpenRouter key or ChatGPT plan any more: its engine is cleared")
	}
}

// planListTimeout bounds reading the plan's model list when the engine is
// chosen: a plan OpenAI does not answer for gives way to the next engine.
const planListTimeout = 10 * time.Second

// The free-tier question, the catalogue and the plan, replaced in a test.
var (
	openRouterFreeTier = providers.OpenRouterFreeTier
	openRouterCatalog  = byokModels
	chatGPTPlanEngine  = providers.ChatGPTPlanEngine
)

// onFreeTierIfFree puts every agent on a free model when OpenRouter says the
// key's account never bought credit (providers/byok_free_tier.go). Anything
// short of that answer keeps the paid ladder: a free model's hosts may train
// on what they are sent, and nobody chose that.
func onFreeTierIfFree(re providers.RemoteEngine) providers.RemoteEngine {
	free, known := openRouterFreeTier(context.Background(), re.APIKey)
	switch {
	case !known:
		logs.New("Brain").Info("OpenRouter did not say whether the account has credit: the paid ladder stays")
		return re
	case !free:
		return re
	}
	list, err := openRouterCatalog(re.APIKey)
	model := providers.FreeTierModel(list)
	if err != nil || model == "" {
		logs.New("Brain").Warn("your OpenRouter account has no credit yet and no free model with tools could be read: the paid ladder stays, and a turn asks for credit (openrouter.ai/settings/credits)")
		return re
	}
	logs.New("Brain").Info("your OpenRouter account has no credit yet: every turn runs on a free model until it has some",
		slog.String("model", model))
	return re.OnFreeTier(model)
}

// BrainStatus is what /api/llm/burst reports about the model: "ready" when a
// model is connected (a provider key, a vendor key), "none" when nothing can
// answer — the window then says how to connect one (cmd/cli tui_ops brainGate).
type BrainStatus struct {
	State string `json:"state"` // ready | none
	Note  string `json:"note,omitempty"`
}

func (s *Server) BrainStatus() BrainStatus {
	if modelConnected() {
		return BrainStatus{State: "ready"}
	}
	return BrainStatus{State: "none"}
}

// modelConnected reports whether anything can answer a turn: the active
// engine (an OpenRouter or vendor key), or any provider with a credential
// that serves chat — a key added with `memdoor connect` sets no engine, and a
// pin reaches it (factory.GetClientFor). A decisions-only provider (TypeSafe)
// answers no turn. "none" here blocks the window's turns (brainGate), so a
// false "none" locks out a working setup (review, 2026-10-04).
func modelConnected() bool {
	if providers.ActiveRemoteEngine() != nil {
		return true
	}
	for _, p := range providers.Providers() {
		if p.Connected() && p.API != providers.APIDecisions {
			return true
		}
	}
	return false
}

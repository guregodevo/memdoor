package providers

import (
	"context"
	"sort"
)

// THE PLAN ANSWERS BY DEFAULT (the maintainer, 2026-10-31: "if you login to chatgpt you
// should get chatgpt model by default in the list"). Signing in with ChatGPT
// is the person choosing to spend their plan, so a signed-in plan is the
// engine before their OpenRouter key (a company's vendor key still comes
// first: that is policy, not a preference), on the first model the plan
// lists: the list is OpenAI's, never a name kept here. Live the same day:
// with the plan signed in, turns still went to OpenRouter, and /model offered
// gpt-6-astra under the OpenAI key (the first provider listing the id won),
// so picking it pinned openai:gpt-6-astra and billed the API.

// ChatGPTPlanEngine is the signed-in plan as the engine, on its first listed
// model. False with no sign-in, or when the plan's list cannot be read or is
// empty: the next engine in line answers then.
func ChatGPTPlanEngine(ctx context.Context) (RemoteEngine, bool) {
	p, ok := FindProvider(ChatGPTID)
	if !ok || !p.Connected() {
		return RemoteEngine{}, false
	}
	list, err := ModelsOf(ctx, p)
	if err != nil || len(list) == 0 {
		return RemoteEngine{}, false
	}
	return p.Engine(list[0].ID), true
}

// IsChatGPTPlan says an engine is a ChatGPT plan's.
func (re RemoteEngine) IsChatGPTPlan() bool { return re.Vendor == VendorChatGPT }

// AnswerOrder is every provider in the order a choice between them is
// offered: the one answering first, then a signed-in ChatGPT plan before the
// OpenAI key (the plan costs nothing per call; FindModel prefers it the same
// way), then the registry's order. /model's list keeps the first provider of
// an id two of them list, so this order decides which one a pick pins.
func AnswerOrder() []Provider {
	list := Providers()
	active := ActiveProviderID()
	rank := func(p Provider) int {
		switch {
		case p.ID == active:
			return 0
		case p.ID == ChatGPTID && p.Connected():
			return 1
		}
		return 2
	}
	sort.SliceStable(list, func(i, j int) bool { return rank(list[i]) < rank(list[j]) })
	return list
}

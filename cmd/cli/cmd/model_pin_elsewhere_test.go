package cmd

import "testing"

// An id OpenRouter does not serve is pinned on the first other provider that
// lists exactly it, with that provider's spelling; OpenRouter's own row and a
// near match are not.
func TestListedElsewhere(t *testing.T) {
	models := []catalogModel{
		{ID: "qwen/qwen-plus", Provider: "openrouter"},
		{ID: "Alibaba/Qwen-Plus", Provider: "llmgateway"},
		{ID: "alibaba/qwen-plus", Provider: "aimlapi"},
		{ID: "alibaba/qwen-plus-latest", Provider: "aimlapi"},
	}
	if e := listedElsewhere("alibaba/qwen-plus", models); e == nil || e.Provider != "llmgateway" || e.ID != "Alibaba/Qwen-Plus" {
		t.Fatalf("the first provider listing the id, its spelling: %+v", e)
	}
	if e := listedElsewhere("qwen/qwen-plus", models); e != nil {
		t.Fatalf("OpenRouter's own id is its hosts' business: %+v", e)
	}
	if e := listedElsewhere("alibaba/qwen", models); e != nil {
		t.Fatalf("a near match is not the id: %+v", e)
	}
}

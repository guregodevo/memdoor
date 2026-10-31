package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeKeyEndpoint answers /api/v1/key with status and body, and records the
// Authorization header it was sent.
func fakeKeyEndpoint(t *testing.T, status int, body string, delay time.Duration) *string {
	t.Helper()
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		time.Sleep(delay)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	saved, savedTimeout := openRouterKeyURL, freeTierTimeout
	openRouterKeyURL, freeTierTimeout = srv.URL, 300*time.Millisecond
	t.Cleanup(func() { openRouterKeyURL, freeTierTimeout = saved, savedTimeout })
	return &auth
}

func TestOpenRouterFreeTier(t *testing.T) {
	auth := fakeKeyEndpoint(t, 200, `{"data":{"label":"Memdoor","usage":0,"is_free_tier":true}}`, 0)
	if free, known := OpenRouterFreeTier(context.Background(), "sk-or-v1-k"); !free || !known {
		t.Fatalf("free account: got free=%v known=%v", free, known)
	}
	if *auth != "Bearer sk-or-v1-k" {
		t.Fatalf("the key goes as a bearer token: %q", *auth)
	}
	fakeKeyEndpoint(t, 200, `{"data":{"is_free_tier":false}}`, 0)
	if free, known := OpenRouterFreeTier(context.Background(), "sk-or-v1-k"); free || !known {
		t.Fatalf("paid account: got free=%v known=%v", free, known)
	}
}

// Every answer short of a clear one is "not known", which keeps the paid
// ladder (the mutation check: answer free=true on an error and these fail).
func TestOpenRouterFreeTierNegativePaths(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		delay  time.Duration
		key    string
	}{
		{"refused key", 401, `{"error":{"message":"User not found."}}`, 0, "sk-or-v1-k"},
		{"no field", 200, `{"data":{"label":"x"}}`, 0, "sk-or-v1-k"},
		{"not json", 200, `<html>`, 0, "sk-or-v1-k"},
		{"server error", 500, `oops`, 0, "sk-or-v1-k"},
		{"too slow", 200, `{"data":{"is_free_tier":true}}`, 2 * time.Second, "sk-or-v1-k"},
		{"no key", 200, `{"data":{"is_free_tier":true}}`, 0, "  "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeKeyEndpoint(t, c.status, c.body, c.delay)
			if free, known := OpenRouterFreeTier(context.Background(), c.key); free || known {
				t.Fatalf("want not known, got free=%v known=%v", free, known)
			}
		})
	}
}

func TestFreeTierModel(t *testing.T) {
	list := []Model{
		{ID: "z-ai/glm-5.3-flash", Context: 1_000_000, Tools: true},   // paid
		{ID: "a/no-tools:free", Context: 2_000_000, Tools: false},     // cannot call tools
		{ID: "b/small:free", Context: 32_000, Tools: true},            // shorter
		{ID: "d/long:free", Context: 262_144, Tools: true},            // tie on context…
		{ID: "c/long:free", Context: 262_144, Tools: true},            // …the id decides
		{ID: "e/freeish", Context: 9_000_000, Tools: true, InPerM: 0}, // not marked :free
	}
	if got := FreeTierModel(list); got != "c/long:free" {
		t.Fatalf("no paid twins: want the longest-context free model with tools, stable on ties: got %q", got)
	}
	// The live shape of 2026-10-31: the twin's price ranks the free models,
	// so inkling:free beats inkling-small:free though "-" sorts first.
	live := []Model{
		{ID: "thinkingmachines/inkling-small:free", Context: 1_048_576, Tools: true},
		{ID: "thinkingmachines/inkling:free", Context: 1_048_576, Tools: true},
		{ID: "cohere/north-mini-code:free", Context: 256_000, Tools: true}, // no twin
		{ID: "thinkingmachines/inkling", OutPerM: 4.05, Tools: true},
		{ID: "thinkingmachines/inkling-small", OutPerM: 1.20, Tools: true},
	}
	if got := FreeTierModel(live); got != "thinkingmachines/inkling:free" {
		t.Fatalf("want the free model with the priciest twin, got %q", got)
	}
	if got := FreeTierModel([]Model{{ID: "x/paid", Tools: true}, {ID: "y/notools:free"}}); got != "" {
		t.Fatalf("no free model with tools: want \"\", got %q", got)
	}
	if got := FreeTierModel(nil); got != "" {
		t.Fatalf("no list: got %q", got)
	}
}

// On a free account every agent and every rung is the free model, and the
// answering model follows (no paid rung to climb to).
func TestOnFreeTier(t *testing.T) {
	re := RemoteEngine{Byok: true, Model: "paid/default", AgentModels: map[string]string{"coder": "paid/a"},
		AgentLadders: map[string][]string{"coder": {"paid/a", "paid/b"}}}
	f := re.OnFreeTier("c/long:free")
	if !f.FreeTier || f.Model != "c/long:free" || f.AgentModels["coder"] != "c/long:free" || len(f.AgentLadders["coder"]) != 1 {
		t.Fatalf("free engine: %+v", f)
	}
	for tier := 0; tier < 3; tier++ {
		if m := f.ModelForTier("coder", tier); m != "c/long:free" {
			t.Fatalf("tier %d answers on %q", tier, m)
		}
	}
	if m := f.AnsweringModel(context.Background(), "chief"); m != "c/long:free" {
		t.Fatalf("an agent with no ladder answers on %q", m)
	}
	if re.AgentLadders["coder"][0] != "paid/a" {
		t.Fatal("the original engine is untouched")
	}
}

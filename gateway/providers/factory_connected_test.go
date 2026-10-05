package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"memdoor/pkg/llm"
)

// A PROVIDER ADDED WITH `memdoor connect` ANSWERS A TURN. It lives in
// ~/.memdoor/providers.json and sets no engine; the status read "ready" and
// every turn waited eight minutes for a broker brain, then failed. A
// connected OpenRouter (a built-in kept in the file) went out with no key:
// its engine was copied from the environment's. Mutation checks: drop the
// connectedClient fallback and GetClientFor errors; copy ByokEngine() in
// Provider.Engine again and the OpenRouter request carries no key.
func TestAConnectedProviderAnswersWithNoEngine(t *testing.T) {
	for _, tc := range []struct{ id, api, key string }{
		{"corp", APIChat, "corp-key"},
		{"openrouter", APIOpenRouter, "or-key"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			clearProviderEnv(t)
			var auth string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/models" {
					_, _ = io.WriteString(w, `{"data":[{"id":"m1"}]}`)
					return
				}
				auth = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: "+`{"choices":[{"index":0,"delta":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`+"\n\n")
				_, _ = io.WriteString(w, "data: [DONE]\n\n")
			}))
			defer srv.Close()
			if err := SaveProvider(ProviderSpec{ID: tc.id, API: tc.api, Base: srv.URL, Key: tc.key, Models: []Model{{ID: "m1"}}}); err != nil {
				t.Fatal(err)
			}
			if ActiveRemoteEngine() != nil {
				t.Fatal("the test needs no engine")
			}
			c, err := (&ClientFactory{}).GetClientFor(context.Background(), "coder")
			if err != nil {
				t.Fatalf("a connected provider answers: %v", err)
			}
			if _, err := c.Messages().New(context.Background(), llm.MessageNewParams{
				MaxTokens: 16,
				Messages:  []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hello"))},
			}); err != nil {
				t.Fatal(err)
			}
			if auth != "Bearer "+tc.key {
				t.Fatalf("the provider's own key: got %q", auth)
			}
		})
	}
}

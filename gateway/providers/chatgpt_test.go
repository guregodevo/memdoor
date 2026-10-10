package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/chatgpt"
	"memdoor/pkg/llm"
	"memdoor/pkg/shared"
)

// A signed-in ChatGPT plan is a connected provider: it lists the plan's
// models (the ones marked for listing), answers a turn with no engine
// set, and every request carries the plan's bearer, store:false and none of
// the fields the preview refuses (temperature, max_output_tokens).
func TestAChatGPTPlanAnswersAsAConnectedProvider(t *testing.T) {
	clearProviderEnv(t)
	_ = logs.InitGlobalLogger(t.TempDir(), false)
	if err := chatgpt.NewStore(shared.MemdoorHome()).Save(&chatgpt.Credential{ClientID: "oaiapp_1", Email: "dev@example.com", AccessToken: "plan-token", RefreshToken: "rt", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var auths []string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auths = append(auths, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{
				{"slug": "gpt-6.1-sol", "display_name": "GPT-6.1 Sol", "visibility": "list"},
				{"slug": "internal-only", "display_name": "Hidden", "visibility": "hidden"},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/responses":
			raw, _ := io.ReadAll(r.Body)
			mu.Lock()
			_ = json.Unmarshal(raw, &body)
			mu.Unlock()
			responsesSSE(w,
				`{"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"m1"}}`,
				`{"type":"response.output_text.delta","output_index":0,"delta":"ok"}`,
				`{"type":"response.completed","response":{"id":"r1","model":"gpt-6.1-sol","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":1}}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	oldBase := chatgptBase
	chatgptBase = srv.URL + "/v1"
	t.Cleanup(func() { chatgptBase = oldBase })

	p, ok := FindProvider(ChatGPTID)
	if !ok || !p.Connected() || !strings.Contains(p.KeySource, "dev@example.com") || p.API != APIChatGPT {
		t.Fatalf("the plan is a connected built-in provider: %+v %v", p, ok)
	}
	list, err := ModelsOf(context.Background(), p)
	if err != nil || len(list) != 1 || list[0].ID != "gpt-6.1-sol" || list[0].Name != "GPT-6.1 Sol" {
		t.Fatalf("the plan's listed models: %+v %v", list, err)
	}
	if ActiveRemoteEngine() != nil {
		t.Fatal("no engine is set for a sign-in")
	}
	c, err := (&ClientFactory{}).GetClientFor(context.Background(), "coder")
	if err != nil {
		t.Fatal(err)
	}
	msg, err := c.Messages().New(context.Background(), llm.MessageNewParams{Model: "x", MaxTokens: 512, Temperature: 0.7, Agent: "coder",
		Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}})
	if err != nil || len(msg.Content) == 0 || msg.Content[0].Text != "ok" || string(msg.Model) != "gpt-6.1-sol" {
		t.Fatalf("the turn's answer: %+v %v", msg, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if body["store"] != false || body["model"] != "gpt-6.1-sol" || body["stream"] != true {
		t.Fatalf("a plan request says store:false and stream:true on the listed model: %v", body)
	}
	for _, refused := range []string{"temperature", "max_output_tokens", "top_p", "metadata", "user", "truncation"} {
		if _, there := body[refused]; there {
			t.Fatalf("%s is refused on a plan request: %v", refused, body)
		}
	}
	for _, a := range auths {
		if !strings.HasSuffix(a, "Bearer plan-token") {
			t.Fatalf("every request carries the plan's bearer: %v", auths)
		}
	}
}

// A refusal of the plan reads in the person's terms, with what to do.
func TestAPlanRefusalSaysWhatToDo(t *testing.T) {
	clearProviderEnv(t)
	_ = logs.InitGlobalLogger(t.TempDir(), false)
	_ = chatgpt.NewStore(shared.MemdoorHome()).Save(&chatgpt.Credential{ClientID: "oaiapp_1", AccessToken: "plan-token", RefreshToken: "rt", ExpiresAt: time.Now().Add(time.Hour)})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"code":"subscription_sharing_usage_limit_exceeded","message":"limit"}}`)
	}))
	defer srv.Close()
	old := oaiRetryAfter
	oaiRetryAfter = nil
	t.Cleanup(func() { oaiRetryAfter = old })
	oldBase := chatgptBase
	chatgptBase = srv.URL + "/v1"
	t.Cleanup(func() { chatgptBase = oldBase })
	_, err := newChatGPTClient("gpt-6.1-sol").Messages().New(context.Background(), llm.MessageNewParams{Model: "x", Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}})
	if err == nil || !strings.Contains(err.Error(), "allowance") || !strings.Contains(err.Error(), "/model") {
		t.Fatalf("an exhausted allowance says so and what to do: %v", err)
	}
	// Nobody signed in: the client says how.
	_ = chatgpt.NewStore(shared.MemdoorHome()).Delete()
	if _, err := newChatGPTClient("gpt-6.1-sol").Messages().New(context.Background(), llm.MessageNewParams{Model: "x"}); err == nil || !strings.Contains(err.Error(), "connect chatgpt") {
		t.Fatalf("no sign-in names the command: %v", err)
	}
	if p, _ := FindProvider(ChatGPTID); p.Connected() {
		t.Fatal("after a sign-out the plan is not connected")
	}
}

// A bare id both the plan and an OpenAI key list goes to the plan; the key
// is reached by name (openai:<id>); an id only the key lists stays on it.
func TestABareIDThePlanAndTheOpenAIKeyListGoesToThePlan(t *testing.T) {
	clearProviderEnv(t)
	_ = logs.InitGlobalLogger(t.TempDir(), false)
	_ = chatgpt.NewStore(shared.MemdoorHome()).Save(&chatgpt.Credential{ClientID: "oaiapp_1", AccessToken: "plan-token", RefreshToken: "rt", ExpiresAt: time.Now().Add(time.Hour)})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.Header.Get("Authorization"), "Bearer plan-token"):
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{{"slug": "gpt-6-astra", "visibility": "list"}}})
		default: // the OpenAI key's list
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "gpt-6-astra"}, {"id": "gpt-4.1-mini"}}})
		}
	}))
	defer srv.Close()
	oldBase := chatgptBase
	chatgptBase = srv.URL + "/v1"
	t.Cleanup(func() { chatgptBase = oldBase })
	t.Setenv("OPENAI_API_KEY", "sk-key")
	t.Setenv("OPENAI_BASE_URL", srv.URL+"/v1")
	ctx := context.Background()
	if p, _, ok := FindModel(ctx, "gpt-6-astra"); !ok || p.ID != ChatGPTID {
		t.Fatalf("a bare id both list goes to the plan, got %q %v", p.ID, ok)
	}
	if p, _, ok := FindModel(ctx, "openai:gpt-6-astra"); !ok || p.ID != "openai" {
		t.Fatalf("the key by name, got %q %v", p.ID, ok)
	}
	if p, _, ok := FindModel(ctx, "gpt-4.1-mini"); !ok || p.ID != "openai" {
		t.Fatalf("an id only the key lists stays on it, got %q %v", p.ID, ok)
	}
}

// A plan refusal that arrives inside the stream (response.failed) reads
// the same as one that arrives as a status: the allowance, and what to do.
func TestAPlanRefusalInsideTheStreamSaysWhatToDo(t *testing.T) {
	clearProviderEnv(t)
	_ = logs.InitGlobalLogger(t.TempDir(), false)
	_ = chatgpt.NewStore(shared.MemdoorHome()).Save(&chatgpt.Credential{ClientID: "oaiapp_1", AccessToken: "plan-token", RefreshToken: "rt", ExpiresAt: time.Now().Add(time.Hour)})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responsesSSE(w, `{"type":"response.failed","response":{"id":"r1","status":"failed","error":{"code":"subscription_sharing_usage_limit_exceeded","message":"The ChatGPT user has reached their Subscription Sharing usage limit."}}}`)
	}))
	defer srv.Close()
	oldBase := chatgptBase
	chatgptBase = srv.URL + "/v1"
	t.Cleanup(func() { chatgptBase = oldBase })
	_, err := newChatGPTClient("gpt-6-sol").Messages().New(context.Background(), llm.MessageNewParams{Model: "x", Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}})
	if err == nil || !strings.Contains(err.Error(), "allowance") || strings.Contains(err.Error(), "stream failed") {
		t.Fatalf("a mid-stream allowance refusal is worded: %v", err)
	}
}

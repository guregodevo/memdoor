package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
	sharedctx "memdoor/pkg/shared/context"
)

func withStallAfter(t *testing.T, d time.Duration) {
	t.Helper()
	old := oaiStallAfter
	oaiStallAfter = d
	t.Cleanup(func() { oaiStallAfter = old })
}

// silent holds a request open without sending a byte, until the client
// gives up on it.
func silent(r *http.Request) {
	select {
	case <-r.Context().Done():
	case <-time.After(time.Second):
	}
}

// A provider that sends nothing is asked again, not waited on for the
// client's 20 minutes (live 2026-09-29: four minutes of silence, then the
// person gave up).
func TestASilentProviderIsAskedAgain(t *testing.T) {
	withStallAfter(t, 150*time.Millisecond)
	var calls int32
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			silent(r)
			return
		}
		sse(w, `{"choices":[{"index":0,"delta":{"content":"OK"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
	})
	defer done()
	start := time.Now()
	msg, err := ask(c)
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&calls) != 2 || msg.Content[0].Text != "OK" {
		t.Fatalf("calls=%d reply=%+v", calls, msg.Content)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("the silence was waited out: %s", time.Since(start))
	}
}

// A model thinking for a long time is not silent: OpenRouter's keep-alive
// comments restart the clock, and the one request is answered.
func TestKeepAlivesAreNotSilence(t *testing.T) {
	withStallAfter(t, 150*time.Millisecond)
	var calls int32
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 8; i++ {
			_, _ = io.WriteString(w, ": OPENROUTER PROCESSING\n\n")
			w.(http.Flusher).Flush()
			time.Sleep(60 * time.Millisecond)
		}
		sse(w, `{"choices":[{"index":0,"delta":{"content":"thought hard"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2}}`)
	})
	defer done()
	msg, err := ask(c)
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&calls) != 1 || msg.Content[0].Text != "thought hard" {
		t.Fatalf("calls=%d reply=%+v", calls, msg.Content)
	}
}

// Silent on every attempt: the turn fails saying so, in seconds.
func TestAProviderSilentEveryTimeFailsClearly(t *testing.T) {
	withStallAfter(t, 100*time.Millisecond)
	var calls int32
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		silent(r)
	})
	defer done()
	_, err := ask(c)
	if err == nil || !strings.Contains(err.Error(), "sent nothing") {
		t.Fatalf("want the silence named, got %v", err)
	}
	if n := atomic.LoadInt32(&calls); int(n) != len(oaiRetryAfter)+1 {
		t.Fatalf("every attempt is spent before failing: %d", n)
	}
}

// Headers, then nothing: the stream's silence is caught between bytes and
// asked again, the same as a request that never got an answer.
func TestAStreamThatGoesSilentIsAskedAgain(t *testing.T) {
	withStallAfter(t, 150*time.Millisecond)
	var calls int32
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			silent(r)
			return
		}
		sse(w, `{"choices":[{"index":0,"delta":{"content":"OK"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
	})
	defer done()
	msg, err := ask(c)
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&calls) != 2 || msg.Content[0].Text != "OK" {
		t.Fatalf("calls=%d reply=%+v", calls, msg.Content)
	}
}

// The person is told when a request is asked again: the screen showed only a
// clock while a provider was silent (2026-09-29).
func TestARetryAfterSilenceIsAnnounced(t *testing.T) {
	withStallAfter(t, 150*time.Millisecond)
	var calls int32
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			silent(r)
			return
		}
		sse(w, `{"choices":[{"index":0,"delta":{"content":"OK"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
	})
	defer done()
	var said []string
	ctx := llm.WithNotice(context.Background(), func(text string) { said = append(said, text) })
	if _, err := c.Messages().New(ctx, llm.MessageNewParams{MaxTokens: 64,
		Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}}); err != nil {
		t.Fatal(err)
	}
	if len(said) != 1 || !strings.Contains(said[0], "sent nothing") || !strings.Contains(said[0], "asking again") {
		t.Fatalf("one notice naming the silence and the retry: %q", said)
	}
}

// A runtime connected with no key reads the prompt before it sends a byte,
// and a CPU reads slowly (a runner's 4 vCPUs: eight minutes for a 9B's
// review prompt, 2026-10-27). The cloud's minute of silence does not apply
// to it, and a step's thinking off reaches it as Ollama's reasoning_effort.
func TestAKeylessRuntimeMayReadInSilence(t *testing.T) {
	withStallAfter(t, 150*time.Millisecond)
	old := keylessStallAfter
	keylessStallAfter = 5 * time.Second
	t.Cleanup(func() { keylessStallAfter = old })
	var calls int32
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		time.Sleep(600 * time.Millisecond)
		sse(w, `{"choices":[{"index":0,"delta":{"content":"OK"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
	}))
	defer srv.Close()
	_ = logs.InitGlobalLogger(t.TempDir(), false)
	t.Setenv("HOME", t.TempDir())
	local := Provider{ID: "local", API: APIChat, Base: srv.URL + "/v1", Keyless: true}.Client(context.Background(), "qwen3:4b")
	ctx := context.WithValue(context.Background(), sharedctx.EffortKey, "off")
	msg, err := local.Messages().New(ctx, llm.MessageNewParams{MaxTokens: 64,
		Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}})
	if err != nil || atomic.LoadInt32(&calls) != 1 || msg.Content[0].Text != "OK" {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	if !strings.Contains(body, `"reasoning_effort":"none"`) {
		t.Fatalf("thinking off was not sent as reasoning_effort none: %s", body)
	}

	// With no thinking off, nothing is sent: a model with no thinking mode
	// refuses an effort.
	if _, err := ask(local); err != nil || strings.Contains(body, "reasoning_effort") {
		t.Fatalf("err=%v body=%s", err, body)
	}
}

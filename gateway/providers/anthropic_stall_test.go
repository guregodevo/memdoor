package providers

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A vendor that speaks the Messages API went quiet before the headers too
// (live 2026-11-18: two coder turns died on "timeout awaiting response
// headers" after a minute of silence): it is asked again, the same as an
// OpenAI-shaped one.
func TestASilentAnthropicProviderIsAskedAgain(t *testing.T) {
	withStallAfter(t, 150*time.Millisecond)
	var calls int32
	c, done := anthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			silent(r)
			return
		}
		anthropicSSE(w,
			`{"type":"message_start","message":{"id":"msg_1","model":"claude-sonnet-5","usage":{"input_tokens":5}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"OK"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
			`{"type":"message_stop"}`)
	})
	defer done()
	start := time.Now()
	msg, err := ask(c)
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&calls) != 2 || len(msg.Content) == 0 || msg.Content[0].Text != "OK" {
		t.Fatalf("calls=%d reply=%+v", calls, msg.Content)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("the silence was waited out: %s", time.Since(start))
	}
}

// Silent on every attempt: the turn fails saying so, in seconds.
func TestAnAnthropicProviderSilentEveryTimeFailsClearly(t *testing.T) {
	withStallAfter(t, 100*time.Millisecond)
	var calls int32
	c, done := anthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
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

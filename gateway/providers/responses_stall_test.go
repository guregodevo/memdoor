package providers

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A gateway that speaks the Responses API went quiet before the headers:
// asked again, the same as the other shapes.
func TestASilentResponsesProviderIsAskedAgain(t *testing.T) {
	withStallAfter(t, 150*time.Millisecond)
	var calls int32
	c, done := responsesTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			silent(r)
			return
		}
		responsesSSE(w,
			`{"type":"response.created","response":{"id":"resp_1"}}`,
			`{"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1"}}`,
			`{"type":"response.output_text.delta","output_index":0,"delta":"OK"}`,
			`{"type":"response.completed","response":{"id":"resp_1","model":"mytestprovider-slug","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"OK"}]}],"usage":{"input_tokens":5,"output_tokens":1}}}`)
	})
	defer done()
	msg, err := ask(c)
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&calls) != 2 || len(msg.Content) == 0 || msg.Content[0].Text != "OK" {
		t.Fatalf("calls=%d reply=%+v", calls, msg.Content)
	}
}

// Silent on every attempt: the turn fails saying so, in seconds.
func TestAResponsesProviderSilentEveryTimeFailsClearly(t *testing.T) {
	withStallAfter(t, 100*time.Millisecond)
	var calls int32
	c, done := responsesTestClient(t, func(w http.ResponseWriter, r *http.Request) {
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

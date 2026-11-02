package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"memdoor/pkg/llm"
	sharedctx "memdoor/pkg/shared/context"
)

// The turn's effort travels as reasoning.effort, and only to a model that
// takes the parameter: requests also say require_parameters, so sending it to
// one that does not would leave no host.
func TestEffortIsSentOnlyToAModelThatTakesIt(t *testing.T) {
	var sent []map[string]any
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Reasoning map[string]any `json:"reasoning"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		sent = append(sent, body.Reasoning)
		sse(w, `{"choices":[{"index":0,"delta":{"content":"OK"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
	})
	defer done()
	c.(*oaiClient).attribute = true
	old := reasoningSupport
	t.Cleanup(func() { reasoningSupport = old })
	params := llm.MessageNewParams{MaxTokens: 8, Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}}
	withEffort := context.WithValue(context.Background(), sharedctx.EffortKey, "low")
	for _, takes := range []bool{true, false} {
		reasoningSupport = func(string) bool { return takes }
		if _, err := c.Messages().New(withEffort, params); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.Messages().New(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 3 || sent[0]["effort"] != "low" || sent[1] != nil || sent[2] != nil {
		t.Fatalf("reasoning sent = %v, want [{effort:low} nil nil]", sent)
	}
}

// A model whose thinking is mandatory refuses "thinking off" with a 400; the
// request is sent once more with effort low, and the step gets its answer.
func TestMandatoryReasoningIsAskedAgainWithLowEffort(t *testing.T) {
	var sent []map[string]any
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Reasoning map[string]any `json:"reasoning"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		sent = append(sent, body.Reasoning)
		if body.Reasoning["enabled"] == false {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"Reasoning is mandatory for this endpoint and cannot be disabled.","code":400}}`)
			return
		}
		sse(w, `{"choices":[{"index":0,"delta":{"content":"OK"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
	})
	defer done()
	c.(*oaiClient).attribute = true
	old := reasoningSupport
	t.Cleanup(func() { reasoningSupport = old })
	reasoningSupport = func(string) bool { return true }
	params := llm.MessageNewParams{MaxTokens: 8, Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}}
	off := context.WithValue(context.Background(), sharedctx.EffortKey, "off")
	msg, err := c.Messages().New(off, params)
	if err != nil || msg.Content[0].Text != "OK" {
		t.Fatalf("the step is answered: err=%v", err)
	}
	if len(sent) != 2 || sent[0]["enabled"] != false || sent[1]["effort"] != "low" {
		t.Fatalf("reasoning sent = %v, want [{enabled:false} {effort:low}]", sent)
	}
}

package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"memdoor/pkg/llm"
)

// A forced call is "required" on the OpenRouter path; without it the retry of
// a reply that only announced its work was asked the same way and announced
// again (live 2026-09-29, GLM 5.3 Flash, "update docs").
func TestForcedToolCallIsRequiredOnOpenRouter(t *testing.T) {
	var choice []string
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ToolChoice string `json:"tool_choice"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		choice = append(choice, body.ToolChoice)
		sse(w, `{"choices":[{"index":0,"delta":{"content":"OK"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
	})
	defer done()
	tool := llm.ToolParam{Name: "bash", InputSchema: llm.ToolInputSchemaParam{Properties: map[string]any{"command": map[string]any{"type": "string"}}}}
	params := llm.MessageNewParams{MaxTokens: 64, Tools: []llm.ToolUnionParam{{OfTool: &tool}},
		Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}}
	for _, ctx := range []context.Context{context.Background(), llm.WithForcedToolCall(context.Background())} {
		if _, err := c.Messages().New(ctx, params); err != nil {
			t.Fatal(err)
		}
	}
	if len(choice) != 2 || choice[0] != "auto" || choice[1] != "required" {
		t.Fatalf("tool_choice = %v, want [auto required]", choice)
	}
}

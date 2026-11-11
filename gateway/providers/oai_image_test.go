package providers

import (
	"encoding/json"
	"testing"

	"memdoor/pkg/llm"
)

// A picture in a user message reaches a chat-completions provider as an
// image_url part; a message without one keeps its plain string content.
func TestAPictureBecomesAnImageURLPart(t *testing.T) {
	msgs := convertAnthropicToOAI(llm.MessageNewParams{Messages: []llm.MessageParam{
		llm.NewUserMessage(
			llm.ContentBlockParamUnion{OfImage: &llm.ImageBlockParam{Type: "image", MediaType: "image/png", Data: "QUJD"}},
			llm.NewTextBlock("what is this?")),
		llm.NewUserMessage(llm.NewTextBlock("plain")),
	}})
	raw, err := json.Marshal(msgs)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"role":"user","content":[{"text":"what is this?","type":"text"},{"image_url":{"url":"data:image/png;base64,QUJD"},"type":"image_url"}]},{"role":"user","content":"plain"}]`
	if string(raw) != want {
		t.Fatalf("\n got %s\nwant %s", raw, want)
	}
}

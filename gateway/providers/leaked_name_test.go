package providers

import (
	"strings"
	"testing"
)

// Live 2026-09-30: GLM quoted "<tool_call>" in its prose, the host started the
// call there, and the tool name came back as the prose up to the real call.
// The call runs as the tool it names; the prose goes back into the text.
func TestALeakedToolNameIsSplitBackIntoTextAndName(t *testing.T) {
	choice := chatChoice{FinishReason: "tool_calls"}
	choice.Message.Content = "Found it: the shear cuts the comment to `//\\t"
	choice.Message.ToolCalls = []chatToolCall{{ID: "c1"}}
	choice.Message.ToolCalls[0].Function.Name = "{`. That leaves the file broken. Let me look at the shear function:<tool_call>jread"
	choice.Message.ToolCalls[0].Function.Arguments = `{"path":"tools/apply_patch.go","task":"the shear"}`
	tools := []chatTool{{Type: "function"}}
	tools[0].Function.Name = "jread"
	tools[0].Function.Parameters = map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{}, "task": map[string]any{}}, "required": []string{"path"}}

	msg := convertOAIToAnthropicWithTools(choice, chatUsage{}, "z-ai/glm-5.3-flash", tools)
	var text, name string
	for _, b := range msg.Content {
		if b.Type == "text" {
			text += b.Text
		}
		if b.Type == "tool_use" {
			name = b.Name
		}
	}
	if name != "jread" {
		t.Fatalf("tool name %q, want jread", name)
	}
	if !strings.Contains(text, "That leaves the file broken") || strings.Contains(text, "<tool_call>") {
		t.Fatalf("the prose must be back in the text, without the tag: %q", text)
	}
}

// A name that is not an offered tool after the split is left alone: no guessing.
func TestALeakedNameNotOfferedIsLeftAlone(t *testing.T) {
	if _, name, ok := splitLeakedName("prose<tool_call>nosuchtool", []ToolShape{{Name: "jread"}}); ok || name != "prose<tool_call>nosuchtool" {
		t.Fatalf("split an unknown tool: %q %v", name, ok)
	}
	if _, name, ok := splitLeakedName("jread", []ToolShape{{Name: "jread"}}); ok || name != "jread" {
		t.Fatalf("a clean name changed: %q %v", name, ok)
	}
}

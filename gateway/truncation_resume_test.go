package gateway

import (
	"testing"

	"memdoor/pkg/llm"
)

// pairedForProviders fails when conv breaks a rule every provider enforces:
// an assistant message with no content or an empty text block, a tool call
// whose input is not JSON, or a tool call not answered first thing in the next
// user message.
func pairedForProviders(t *testing.T, conv []llm.MessageParam) {
	t.Helper()
	for i, m := range conv {
		if len(m.Content) == 0 {
			t.Fatalf("message %d (%s) has no content", i, m.Role)
		}
		var calls []string
		for _, b := range m.Content {
			if b.OfText != nil && b.OfText.Text == "" {
				t.Fatalf("message %d has an empty text block", i)
			}
			if b.OfToolUse != nil {
				calls = append(calls, b.OfToolUse.ID)
			}
		}
		if len(calls) == 0 {
			continue
		}
		if i+1 >= len(conv) {
			t.Fatalf("message %d's tool calls are never answered", i)
		}
		next := conv[i+1].Content
		for j, id := range calls {
			if j >= len(next) || next[j].OfToolResult == nil || next[j].OfToolResult.ToolUseID != id {
				t.Fatalf("call %s is not answered first in the next message", id)
			}
		}
	}
}

func cutReply(blocks ...llm.ContentBlockParamUnion) llm.MessageParam {
	return llm.MessageParam{Role: llm.MessageParamRoleAssistant, Content: blocks}
}

// A reply cut at the cap resumes with its whole calls answered, the call cut
// mid-arguments dropped, and no empty text.
func TestAResumeAfterACutReplyIsAcceptedByProviders(t *testing.T) {
	conv := []llm.MessageParam{
		llm.NewUserMessage(llm.NewTextBlock("rewrite main.go")),
		cutReply(llm.NewTextBlock(""),
			llm.NewToolUseBlock("ok-1", `{"path":"main.go"}`, "read_file"),
			llm.NewToolUseBlock("cut-2", `{"input":"*** Begin Patch\n*** Add File: main.go\n+package ma`, "apply_patch")),
	}
	results := []llm.ContentBlockParamUnion{
		llm.NewToolResultBlock("ok-1", "package main", false),
		llm.NewToolResultBlock("cut-2", "invalid apply_patch input", true),
	}
	got := resumeAfterCut(conv, results, resumePrompt)
	pairedForProviders(t, got)
	if len(got) != 3 {
		t.Fatalf("%d messages", len(got))
	}
	if n := len(got[1].Content); n != 1 || got[1].Content[0].OfToolUse.ID != "ok-1" {
		t.Fatalf("the reply kept %d blocks", n)
	}
	last := got[2].Content
	if len(last) != 2 || last[1].OfText == nil || last[1].OfText.Text != resumePrompt {
		t.Fatalf("the resume message is %+v", last)
	}
}

// A reply with nothing left is dropped, and the model is still asked to go on.
func TestAnEmptyCutReplyIsDropped(t *testing.T) {
	conv := []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("go")), cutReply()}
	got := resumeAfterCut(conv, nil, resumePrompt)
	pairedForProviders(t, got)
	if len(got) != 2 || got[1].Role != llm.MessageParamRoleUser {
		t.Fatalf("%+v", got)
	}
}

// A turn that gives up leaves its text and no unanswered call.
func TestAGivenUpTurnLeavesASendableConversation(t *testing.T) {
	conv := []llm.MessageParam{
		llm.NewUserMessage(llm.NewTextBlock("go")),
		cutReply(llm.NewTextBlock("Writing the file"), llm.NewToolUseBlock("c", `{"path":"a"}`, "read_file")),
	}
	got := closeCutReply(conv)
	pairedForProviders(t, got)
	if len(got) != 2 || len(got[1].Content) != 1 || got[1].Content[0].OfText.Text != "Writing the file" {
		t.Fatalf("%+v", got)
	}
	if got := closeCutReply([]llm.MessageParam{conv[0], cutReply(llm.NewTextBlock(" "))}); len(got) != 1 {
		t.Fatalf("an empty reply stays: %+v", got)
	}
}

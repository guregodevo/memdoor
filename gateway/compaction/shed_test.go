package compaction

import (
	"strings"
	"testing"

	"memdoor/pkg/llm"
)

func toolResultMsg(id, body string) llm.MessageParam {
	return llm.MessageParam{
		Role: llm.MessageParamRoleUser,
		Content: []llm.ContentBlockParamUnion{{OfToolResult: &llm.ToolResultBlockParam{
			ToolUseID: id,
			Content: []llm.ToolResultBlockParamContentUnion{{OfText: &llm.TextBlockParam{
				Type: "text", Text: body,
			}}},
		}}},
	}
}

func bodyOf(m llm.MessageParam) string {
	var b strings.Builder
	for _, blk := range m.Content {
		if blk.OfToolResult == nil {
			continue
		}
		for _, cu := range blk.OfToolResult.Content {
			if cu.OfText != nil {
				b.WriteString(cu.OfText.Text)
			}
		}
	}
	return b.String()
}

// SHED, DO NOT DIE.
//
// Emergency compaction keeps five MESSAGES and never looks inside them — and
// a single message can be a whole file or a long command's output. A turn
// could be compacted to five messages and still not fit, and the run died
// (live 2026-09-16, a turn lost three tools in). The tool payloads are the
// biggest thing in the window and the most replaceable.
func TestShedToolResultsGivesUpThePayloadsNotTheTurn(t *testing.T) {
	big := strings.Repeat("transcript line. ", 500)
	msgs := []llm.MessageParam{
		toolResultMsg("a", big),
		toolResultMsg("b", big),
		toolResultMsg("c", big),
	}

	// keepLast 2: the oldest payload goes, the working ones stay.
	shed, changed := ShedToolResults(msgs, 2)
	if !changed {
		t.Fatal("nothing was shed")
	}
	if !strings.Contains(bodyOf(shed[0]), "elided") {
		t.Fatalf("the oldest payload survived: %.60s", bodyOf(shed[0]))
	}
	if bodyOf(shed[2]) != big {
		t.Fatal("the newest payload must be kept intact while it can be")
	}
	// The structure is preserved: same count, same tool_use ids, same roles.
	if len(shed) != len(msgs) {
		t.Fatalf("messages were dropped: %d vs %d", len(shed), len(msgs))
	}
	if shed[0].Content[0].OfToolResult.ToolUseID != "a" {
		t.Fatal("a shed result lost its tool_use id, which breaks the pairing")
	}

	// keepLast 0: everything goes, and the stub says how to get it back.
	all, changed := ShedToolResults(msgs, 0)
	if !changed {
		t.Fatal("nothing was shed at keepLast 0")
	}
	for i := range all {
		body := bodyOf(all[i])
		if !strings.Contains(body, "elided") {
			t.Fatalf("payload %d survived a full shed", i)
		}
		if !strings.Contains(body, "recall {\"id\":") {
			t.Fatalf("the stub does not tell the model how to recover: %s", body)
		}
		if len(body) > 200 {
			t.Fatalf("the stub is not small: %d chars", len(body))
		}
	}

	// A short result is left alone: stubbing it saves nothing and loses signal.
	small := []llm.MessageParam{toolResultMsg("d", "ok, 3 files")}
	if _, changed := ShedToolResults(small, 0); changed {
		t.Fatal("a two-line result was stubbed")
	}
}

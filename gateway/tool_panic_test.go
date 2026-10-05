package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"memdoor/pkg/llm"
	"memdoor/tools"
)

// A TOOL THAT PANICS MUST NOT TAKE THE GATEWAY WITH IT.
//
// Live 2026-09-17: one caption card with no lit word indexed words[-1],
// and "index out of range" ended the PROCESS mid-turn — the film, the
// session, the websocket and every other agent in it, for a bad index in a
// picture. Nothing between the tool and main() caught anything.
func TestAPanickingToolFailsItsCallAndNothingElse(t *testing.T) {
	ar, err := NewAgentRuntime("", false)
	if err != nil {
		t.Skipf("runtime unavailable here: %v", err)
	}
	ar.tools = append(ar.tools, tools.ToolDefinition{
		Name:        "boom",
		Description: "a tool that crashes",
		Function: func(json.RawMessage) (string, error) {
			var words []string
			return words[-1+len(words)], nil // index out of range, as it happened
		},
	})

	session := &Session{ID: "workspace:w:channel:c", Metadata: map[string]interface{}{}}
	info, result := ar.executeTool(context.Background(),
		&llm.ToolUseBlock{ID: "tu-1", Name: "boom", Input: json.RawMessage(`{}`)},
		"run-1", session)

	// The call failed; the process did not.
	if info.Error == "" {
		t.Fatal("a panicking tool must report an error")
	}
	if !strings.Contains(info.Error, "crashed") || !strings.Contains(info.Error, "boom") {
		t.Fatalf("the error must name the tool and say it crashed: %q", info.Error)
	}
	// And the model is told that repeating it is pointless, so it does not
	// spend the turn on the same crash.
	if !strings.Contains(info.Error, "crash again") {
		t.Fatalf("the error must say a retry changes nothing: %q", info.Error)
	}
	if result.OfToolResult == nil || !result.OfToolResult.IsError {
		t.Fatal("the result must be flagged as an error, or the loop reads it as success")
	}
	if info.Name != "boom" {
		t.Fatalf("the failed call lost its name: %q", info.Name)
	}

	// The runtime is still usable afterwards: a second call runs normally.
	ar.tools = append(ar.tools, tools.ToolDefinition{
		Name: "fine", Description: "works",
		Function: func(json.RawMessage) (string, error) { return "ok", nil },
	})
	after, _ := ar.executeTool(context.Background(),
		&llm.ToolUseBlock{ID: "tu-2", Name: "fine", Input: json.RawMessage(`{}`)},
		"run-1", session)
	if after.Error != "" || after.Output != "ok" {
		t.Fatalf("the runtime did not survive the panic: %+v", after)
	}
}

// THE STACK GOES IN THE MESSAGE. `memdoor logs query` renders the message
// and drops slog attrs, so the first crash logged with the stack in an
// attribute was unreadable from the CLI: "PANICKED: nil pointer
// dereference" and nothing to act on (2026-09-17).
func TestThePanicLineNamesOurOwnFrames(t *testing.T) {
	stack := `goroutine 42 [running]:
runtime/debug.Stack()
	/usr/local/go/src/runtime/debug/stack.go:26 +0x64
github.com/go-text/typesetting/shaping.(*HarfbuzzShaper).Shape(0x14000051768)
	/Users/x/go/pkg/mod/github.com/go-text/typesetting@v0.3.4/shaping/shaping.go:89 +0x1e4
memdoor/tools.shapeWord(0x14000430000)
	/Users/you/Dev/aktapus/tools/captions_render.go:118 +0x224
memdoor/tools.renderCard(0x14000430000)
	/Users/you/Dev/aktapus/tools/captions_render.go:193 +0x5f4
memdoor/tools.RenderTool({0x1400003a280})
	/Users/you/Dev/aktapus/tools/render.go:405 +0x938`

	got := ourFrames(stack, 4)
	if !strings.Contains(got, "captions_render.go:118") {
		t.Fatalf("the crashing line is missing: %q", got)
	}
	if !strings.Contains(got, "render.go:405") {
		t.Fatalf("the tool entry point is missing: %q", got)
	}
	if strings.Contains(got, "shaping.go") {
		t.Fatalf("a library frame was reported as ours: %q", got)
	}
	if got := ourFrames("nothing useful here", 4); got != "an unreadable stack" {
		t.Fatalf("a stack with no frames of ours: %q", got)
	}
}

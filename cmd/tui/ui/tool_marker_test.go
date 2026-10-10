package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"memdoor/tools"
)

// A tool that finished with nothing to say is finished. `Bash(touch x)` kept a
// frozen spinner for the rest of the session (live 2026-10-01), which reads as
// a tool that never came back — and kept the animation ticking for it.
func TestAToolThatSaidNothingStillLooksFinished(t *testing.T) {
	m := NewModel("http://x", "ws", "chan", "", nil)
	m.width = 100
	m.messages = []Message{{
		Role: "tool_call", ToolName: "bash", ToolInput: `{"command":"touch x"}`,
		Timestamp: time.Now(), ToolDone: true,
	}}

	out := m.renderMessages()
	for _, frame := range brailleFrames {
		if strings.Contains(out, frame) {
			t.Fatalf("a finished tool must not spin (%s):\n%s", frame, out)
		}
	}
	if !strings.Contains(out, "⏺") {
		t.Fatalf("a finished tool wears the bullet:\n%s", out)
	}
	if !m.messages[0].toolSettled() {
		t.Fatal("the frame should count as settled")
	}

	// Still running: no result yet, so the spinner is right. A second model,
	// because the first render freezes a settled frame into scrollback.
	r := NewModel("http://x", "ws", "chan", "", nil)
	r.width = 100
	r.messages = []Message{{
		Role: "tool_call", ToolName: "bash", ToolInput: `{"command":"touch x"}`,
		Timestamp: time.Now(),
	}}
	if r.messages[0].toolSettled() {
		t.Fatal("a tool with no result is not settled")
	}
	spinning := r.renderMessages()
	found := false
	for _, frame := range brailleFrames {
		if strings.Contains(spinning, frame) {
			found = true
		}
	}
	if !found {
		t.Fatalf("a running tool spins:\n%s", spinning)
	}
}

// The real sequence the gateway sends for `touch x`: a start with an id, then
// a complete with the same id and nothing in it.
func TestASilentToolSettlesFromTheRealEventSequence(t *testing.T) {
	m := NewModel("http://x", "ws", "chan", "", nil)
	m.width = 100
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); _ = cmd }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(toolCallStartMsg{toolID: "tu_1", toolName: "bash", toolInput: `{"command":"touch x"}`})
	step(toolCallCompleteMsg{toolID: "tu_1", toolName: "bash", toolOutput: "", toolError: ""})

	var frames int
	for _, msg := range m.messages {
		if msg.Role != "tool_call" {
			continue
		}
		frames++
		if !msg.ToolDone {
			t.Fatalf("the frame did not settle: %+v", msg)
		}
	}
	if frames != 1 {
		t.Fatalf("want one tool frame, got %d: %+v", frames, m.messages)
	}
}

// The render cache must not outlive the frame's state. A tool with no output
// and no error hashes the same before and after its result, so the cached
// spinner was served for the rest of the session (live 2026-10-01).
func TestTheRenderCacheDoesNotFreezeASilentToolAsASpinner(t *testing.T) {
	running := Message{Role: "tool_call", ToolName: "bash", ToolInput: `{"command":"touch x"}`, Timestamp: time.Now()}
	done := running
	done.ToolDone = true
	if blockKey(running, 100, false, "") == blockKey(done, 100, false, "") {
		t.Fatal("a finished frame must not hash like the running one")
	}

	m := NewModel("http://x", "ws", "chan", "", nil)
	m.width = 100
	m.blocks = blockCache{}
	m.messages = []Message{running}
	first := m.renderBlock(0, m.messages[0])
	m.messages[0].ToolDone = true
	second := m.renderBlock(0, m.messages[0])
	if first == second {
		t.Fatalf("the cache served the running render after the tool finished:\n%s", second)
	}
	if !strings.Contains(second, "⏺") {
		t.Fatalf("a finished tool wears the bullet:\n%s", second)
	}
}

// A command that exits non-zero reports through its output, so the bullet has
// to read that: a failed build wore the green one (live 2026-10-01).
func TestAFailedCommandWearsTheErrorBullet(t *testing.T) {
	failed := Message{Role: "tool_call", ToolName: "bash", ToolDone: true,
		ToolOutput: tools.CommandFailedPrefix + "exit status 1): `cat /nope`\nOutput: no such file"}
	if got := toolState(failed); got != toolFailed {
		t.Fatalf("a failed command is a failure, got %v", got)
	}
	if got := toolState(Message{Role: "tool_call", ToolName: "bash", ToolOutput: "hi", ToolDone: true}); got != toolSucceeded {
		t.Fatalf("a command that worked is a success, got %v", got)
	}
	if got := toolState(Message{Role: "tool_call", ToolName: "bash"}); got != toolRunning {
		t.Fatalf("no result yet is running, got %v", got)
	}
	if got := toolState(Message{Role: "tool_call", ToolName: "read_file", ToolError: "refused in plan mode"}); got != toolPlanBlocked {
		t.Fatalf("a plan-mode block is not a failure, got %v", got)
	}
}

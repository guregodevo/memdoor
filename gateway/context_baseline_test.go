package gateway

import (
	"testing"
	"time"

	ctxmgmt "memdoor/gateway/context"
	"memdoor/gateway/infra"
	"memdoor/pkg/llm"
	"memdoor/tools"
)

// THE WINDOW /CONTEXT SHOWS IS THE MODEL'S, NOT THE COMPACTION CEILING.
// Before the first turn the TUI said "0 / 200.0k": the limit it held was
// ctxmgmt.CompactionCeiling — a knob for when to compact — displayed as if
// it were the window, and "0" hid the system prompt and tool schemas every
// request already carries. The baseline emit says both at subscribe time.
func TestEmitContextBaselineReportsTheWindowAndTheFixedCost(t *testing.T) {
	ar, err := NewAgentRuntime("", false)
	if err != nil {
		t.Skipf("runtime unavailable in this environment: %v", err)
	}

	// One turn ran once: the state holds the answering model, the prompt and
	// the tool schemas. The baseline is measured against them, not against a
	// conversation (EmitContextBaseline passes nil).
	tools.ContextToolState.Model = "default"
	tools.ContextToolState.SystemPrompt = "You are the coder, a DOER on a coding team."
	tools.ContextToolState.ToolSchemas = []llm.ToolParam{{Name: "bash"}}
	t.Cleanup(func() {
		tools.ContextToolState = struct {
			Model        string
			SystemPrompt string
			ToolSchemas  []llm.ToolParam
			Messages     []llm.MessageParam
		}{}
	})

	events := make(chan infra.AgentEvent, 8)
	unregister := ar.events.OnEvent(func(e infra.AgentEvent) {
		if e.Stream == infra.EventStreamContext {
			events <- e
		}
	})
	defer unregister()

	ar.EmitContextBaseline("workspace:w:channel:c")

	select {
	case e := <-events:
		if e.Data["event"] != "context_update" {
			t.Fatalf("wrong event: %v", e.Data["event"])
		}
		limit, _ := e.Data["limit"].(int)
		tokens, _ := e.Data["tokens"].(int)
		// The limit the checker itself resolves for the answering model —
		// which on this machine is the remote engine's serving window. What
		// it must never be is 200_000, the compaction ceiling the TUI used to
		// display as if it were the window.
		limits, err := ctxmgmt.GetModelLimits(tools.ContextToolState.Model)
		if err != nil {
			t.Fatal(err)
		}
		if limit != limits.EffectiveLimit() {
			t.Fatalf("limit %d is not the model's effective window %d", limit, limits.EffectiveLimit())
		}
		if limit == 200_000 {
			t.Fatal("the compaction ceiling leaked into the window again")
		}
		if tokens <= 0 {
			t.Fatalf("tokens %d: the baseline must count the prompt and tools it carries", tokens)
		}
		if e.SessionID != "workspace:w:channel:c" {
			t.Fatalf("baseline carried session %q", e.SessionID)
		}
	case <-time.After(time.Second):
		t.Fatal("no context event emitted at subscribe")
	}
}

// No state measured yet — a gateway that has not run a turn emits nothing,
// and the TUI keeps saying "no report yet" instead of inventing a window.
func TestEmitContextBaselineIsSilentWithoutState(t *testing.T) {
	ar, err := NewAgentRuntime("", false)
	if err != nil {
		t.Skipf("runtime unavailable in this environment: %v", err)
	}

	events := make(chan infra.AgentEvent, 8)
	unregister := ar.events.OnEvent(func(e infra.AgentEvent) {
		if e.Stream == infra.EventStreamContext {
			events <- e
		}
	})
	defer unregister()

	saved := tools.ContextToolState
	tools.ContextToolState = struct {
		Model        string
		SystemPrompt string
		ToolSchemas  []llm.ToolParam
		Messages     []llm.MessageParam
	}{} // nothing measured
	t.Cleanup(func() { tools.ContextToolState = saved })

	ar.EmitContextBaseline("workspace:w:channel:c")

	select {
	case e := <-events:
		t.Fatalf("emitted %+v with nothing measured", e.Data)
	default:
	}
}

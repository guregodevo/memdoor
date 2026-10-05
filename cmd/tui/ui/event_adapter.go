package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"memdoor/gateway/infra"
	"memdoor/gateway/streaming/consumer"
)

// TUIEventAdapter wraps the UI-agnostic consumer for TUI use
// Pattern: Adapter pattern - converts consumer.ProcessedEvent to Bubble Tea messages
type TUIEventAdapter struct {
	consumer *consumer.Consumer
	program  msgSender
}

// NewTUIEventAdapter creates a new TUI event adapter
func NewTUIEventAdapter(program *tea.Program) *TUIEventAdapter {
	adapter := &TUIEventAdapter{}
	if program != nil {
		adapter.program = program
	}

	// Create consumer with TUI-specific handler
	opts := consumer.DefaultConsumerOptions()
	opts.EnableLogging = false // Disable logging in TUI mode

	adapter.consumer = consumer.NewConsumer(adapter.handleProcessedEvent, opts)

	return adapter
}

// SetProgram updates the program reference
func (a *TUIEventAdapter) SetProgram(program *tea.Program) {
	if program == nil {
		a.SetSink(nil)
		return
	}
	a.SetSink(program)
}

// SetSink sets where messages go.
func (a *TUIEventAdapter) SetSink(s msgSender) { a.program = s }

// OnAgentEvent processes an incoming agent event
func (a *TUIEventAdapter) OnAgentEvent(event infra.AgentEvent) {
	// Forward to consumer for processing
	a.consumer.ProcessEvent(event)
}

// handleProcessedEvent converts ProcessedEvent to Bubble Tea messages
// This is the event handler callback registered with the consumer
func (a *TUIEventAdapter) handleProcessedEvent(event *consumer.ProcessedEvent) {
	if a.program == nil {
		return
	}

	// Get stream type from metadata
	stream, _ := event.Metadata["stream"].(string)

	// Handle based on stream type and event type
	switch stream {
	case "assistant":
		a.handleAssistantUpdate(event)
	case "tool":
		a.handleToolUpdate(event)
	case "lifecycle":
		a.handleLifecycleUpdate(event)
	case "context":
		a.handleContextUpdate(event)
	case "error":
		a.handleErrorUpdate(event)
	}
}

// handleAssistantUpdate handles assistant stream events
func (a *TUIEventAdapter) handleAssistantUpdate(event *consumer.ProcessedEvent) {
	switch event.EventType {
	case "thinking":
		a.program.Send(assistantThinkingMsg{})

	case "text_start":
		// Text streaming started (after thinking)
		// No specific message needed - handled by text_delta

	case "text_delta":
		// Incremental text chunk
		if event.TextDelta != "" {
			a.program.Send(assistantStreamingMsg{
				content: event.TextDelta,
			})
		}

	case "text_end":
		// Final text (streaming complete)
		if event.FullText != "" {
			a.program.Send(assistantResponseMsg{
				content: event.FullText,
			})
		}

	case "text":
		// Complete text (non-streaming), or — appended — only the tail the
		// rounds never streamed (the receipt line, a note).
		if event.FullText != "" {
			a.program.Send(assistantResponseMsg{
				content:  event.FullText,
				appended: event.TextAppend,
			})
		}
	}
}

// handleToolUpdate handles tool stream events
func (a *TUIEventAdapter) handleToolUpdate(event *consumer.ProcessedEvent) {
	// The status beats carry no ToolState: they are read from the parsed
	// data and drive the status line only. They were dropped here from
	// b6d918f8 (the old websocket handler that decoded them went) until
	// 2026-10-05: a dispatched coder's work and a long tool's timer both
	// vanished from the screen.
	if data, ok := event.Metadata["parsed_data"].(consumer.ToolEventData); ok {
		switch event.EventType {
		case "progress":
			a.program.Send(toolProgressMsg{toolName: data.Tool, seconds: data.Seconds})
			return
		case "subagent":
			a.program.Send(subagentWorkMsg{sessionID: data.Session, agent: data.Agent, tool: data.Tool, seconds: data.Seconds, done: data.State == "done"})
			return
		}
	}
	if event.ToolUpdate == nil {
		return
	}

	// These tools have dedicated TUI surfaces — the interactive question picker
	// and the plan proposal — delivered via their own "question"/"plan" events.
	// Suppress their GENERIC tool_call rendering, which would otherwise dump the
	// raw {"question","options"} / plan JSON into the thread beside the real UI.
	switch event.ToolUpdate.Name {
	case "ask_user_question", "exit_plan_mode":
		return
	}

	switch event.EventType {
	case "start":
		// Tool execution started
		a.program.Send(toolCallStartMsg{
			toolName:  event.ToolUpdate.Name,
			toolInput: event.ToolUpdate.Input,
		})

	case "update":
		// Progress from a streaming tool (bash): the output SO FAR. Carries the
		// whole accumulated buffer, so the TUI replaces rather than appends.
		a.program.Send(toolOutputDeltaMsg{
			toolName: event.ToolUpdate.Name,
			chunk:    event.ToolUpdate.Output,
		})

	case "complete":
		// Tool execution completed
		a.program.Send(toolCallCompleteMsg{
			toolName:   event.ToolUpdate.Name,
			toolOutput: event.ToolUpdate.Output,
			toolError:  event.ToolUpdate.Error,
		})

	case "result":
		// Tool execution result
		a.program.Send(toolCallCompleteMsg{
			toolName:   event.ToolUpdate.Name,
			toolOutput: event.ToolUpdate.Output,
			toolError:  event.ToolUpdate.Error,
		})
	}
}

// handleLifecycleUpdate handles lifecycle stream events. The run-complete signal
// is load-bearing: it's the ONLY thing that clears the spinner on a SILENT turn
// (the agent's last act is a tool call, or it produced no final text), where no
// assistant message arrives to do it. Without this the spinner hangs until the
// stall watchdog — the "still spinning after the task is done" bug. Gated to the
// exact completion events so compaction_* (same stream) don't falsely end the run.
func (a *TUIEventAdapter) handleLifecycleUpdate(event *consumer.ProcessedEvent) {
	switch event.EventType {
	case "complete", "end", "lifecycle_complete":
		done := runCompleteMsg{}
		if d, ok := event.Metadata["parsed_data"].(consumer.LifecycleEventData); ok {
			done.model = d.Model
		}
		a.program.Send(done)
	case "error":
		// The runtime wraps every failed turn in this event (with the
		// reason); it is what ends the turn on screen and says why. The
		// error STREAM event that precedes it only clears the spinner, so
		// the reason is printed once.
		errText := event.Error
		if errText == "" {
			errText = "the turn ended with an error"
		}
		a.program.Send(executionFailedMsg{errText: errText})
	}
}

// handleErrorUpdate handles error stream events: the turn is over, and
// the frame carries the reason. (The consumer finalizes the run on it
// and drops the lifecycle "error" that follows; a run that emits only
// the lifecycle event is handled above. The screen prints one line
// either way — executionFailedMsg is idempotent.)
func (a *TUIEventAdapter) handleErrorUpdate(event *consumer.ProcessedEvent) {
	if event.Error == "" {
		a.program.Send(runCompleteMsg{})
		return
	}
	a.program.Send(executionFailedMsg{errText: event.Error})
}

// handleContextUpdate handles context stream events
func (a *TUIEventAdapter) handleContextUpdate(event *consumer.ProcessedEvent) {
	// Extract context metrics from metadata
	// The consumer stores parsed ContextEventData in metadata["parsed_data"]
	parsedData, ok := event.Metadata["parsed_data"].(consumer.ContextEventData)
	if !ok {
		return
	}

	a.program.Send(contextUpdateMsg{
		tokens:  parsedData.Tokens,
		limit:   parsedData.Limit,
		percent: parsedData.Percent,
		parts: contextParts{system: parsedData.System, tools: parsedData.Tools,
			conversation: parsedData.Conversation, toolOutput: parsedData.ToolOutput, compactAt: parsedData.CompactAt,
			compactRule: parsedData.CompactRule, keepRecent: parsedData.KeepRecent},
	})
}

// Stop stops the consumer
func (a *TUIEventAdapter) Stop() {
	if a.consumer != nil {
		a.consumer.Stop()
	}
}

// GetStats returns consumer statistics
func (a *TUIEventAdapter) GetStats() map[string]interface{} {
	if a.consumer != nil {
		return a.consumer.GetStats()
	}
	return nil
}

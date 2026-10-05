package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"memdoor/gateway/client"
	"memdoor/gateway/infra"
	"memdoor/gateway/protocol"

	tea "github.com/charmbracelet/bubbletea"
)

// Bubble Tea message types used by the adapter

type websocketConnectedMsg struct{}

type websocketDisconnectedMsg struct {
	err error
}

type toolCallStartMsg struct {
	// toolID is the tool_use id. It is what lets a RESULT find the frame that
	// asked for it; matching on toolName alone cannot tell three concurrent
	// bash calls apart.
	toolID    string
	toolName  string
	toolInput string
}

// toolOutputDeltaMsg carries a chunk of a streaming tool's output (bash) while
// it is still running, so the frame can render a live tail.
type toolOutputDeltaMsg struct {
	toolName string
	chunk    string
}

type toolCallCompleteMsg struct {
	toolID     string
	toolName   string
	toolOutput string
	toolError  string
}

// toolProgressMsg is a running tool saying it is still running: the same tool,
// N seconds in. It carries no output — it exists so a long render is
// distinguishable from a hang.
type toolProgressMsg struct {
	toolName string
	seconds  int
}

// subagentWorkMsg is what a SPAWNED run is doing, mirrored onto the session
// that asked for it (the gateway emits it in agent_runtime_progress.go). tool
// is empty between tool calls; done means that run is over.
type subagentWorkMsg struct {
	sessionID string
	agent     string
	tool      string
	seconds   int
	done      bool
}

// WebSocketAdapter wraps the common client library and adapts it for TUI use
// Pattern: Adapter pattern - converts protocol.Message to Bubble Tea messages
type WebSocketAdapter struct {
	client       *client.Client
	program      msgSender
	eventAdapter *TUIEventAdapter // UI-agnostic event consumer
}

// NewWebSocketAdapter creates a new WebSocket adapter for TUI. The connection
// subscribes by workspace + channel: the gateway builds the session key
// server-side, the same key the agent run emits its events on. No client-side
// session-key construction (single source of truth in the domain model).
func NewWebSocketAdapter(gatewayURL string, workspace string, channel string, token string) *WebSocketAdapter {
	// Configure client for TUI mode
	opts := client.DefaultOptions()
	opts.EnableLogging = false           // Disable logging in TUI mode (would interfere with display)
	opts.CompressionThreshold = 8 * 1024 // 8KB threshold (matches gateway)
	opts.Workspace = workspace           // scope runs to this workspace
	opts.Channel = channel               // with Workspace, the gateway builds the session key
	opts.Token = token                   // the same credentials the REST calls carry

	c := client.New(gatewayURL, "", opts)

	adapter := &WebSocketAdapter{
		client: c,
	}

	// Create event adapter (will be initialized with program later)
	adapter.eventAdapter = NewTUIEventAdapter(nil)

	return adapter
}

func (a *WebSocketAdapter) Connect() error {
	// Register message handlers BEFORE connecting
	a.setupHandlers()

	// Connect to gateway
	if err := a.client.Connect(); err != nil {
		return err
	}

	// Register event handler
	a.client.OnEvent(func(event client.Event) {
		if a.program == nil {
			return
		}

		switch event.Type {
		case client.EventConnected:
			a.program.Send(websocketConnectedMsg{})
		case client.EventDisconnected:
			a.program.Send(websocketDisconnectedMsg{err: event.Error})
		case client.EventError:
			a.program.Send(websocketDisconnectedMsg{err: event.Error})
		}
	})

	// Start listening for messages
	a.client.Listen()

	return nil
}

// setupHandlers registers all message type handlers
func (a *WebSocketAdapter) setupHandlers() {
	// Connected message
	a.client.OnMessage(protocol.MessageTypeConnected, func(msg protocol.Message) error {
		// Event handler will handle this
		return nil
	})

	// Chat response
	a.client.OnMessage(protocol.MessageTypeChatResponse, func(msg protocol.Message) error {
		a.handleChatResponse(msg)
		return nil
	})

	// Agent events (lifecycle, assistant, tool)
	a.client.OnMessage(protocol.MessageTypeAgentEvent, func(msg protocol.Message) error {
		a.handleAgentEvent(msg)
		return nil
	})

	// Execution failure: the turn died and will produce nothing more. Without
	// this handler the broadcast was dropped and the activity bar counted a
	// dead run forever (live, 2026-08-31).
	a.client.OnMessage("execution.failed", func(msg protocol.Message) error {
		errText := "unknown error"
		if inner, ok := msg.Data["data"].(map[string]interface{}); ok {
			if e, ok := inner["error"].(string); ok && e != "" {
				errText = e
			}
		}
		a.program.Send(executionFailedMsg{errText: errText})
		return nil
	})

	// A scheduled check answering (gateway: postCronAnswer broadcasts to the
	// session that scheduled it). The payload rides as {type, data:{job_id,
	// text}} like execution.failed above, but read it either way: a dropped
	// field here is a silent hole, as the turn-death broadcast once was.
	a.client.OnMessage("cron_answer", func(msg protocol.Message) error {
		data := msg.Data
		if inner, ok := msg.Data["data"].(map[string]interface{}); ok {
			data = inner
		}
		text, _ := data["text"].(string)
		jobID, _ := data["job_id"].(string)
		if text != "" && a.program != nil {
			a.program.Send(cronAnswerMsg{jobID: jobID, text: text})
		}
		return nil
	})

	// A workflow run's transitions (gateway: workflow_runs.go). The payload
	// carries the run's whole state, so the graph is redrawn from the truth.
	a.client.OnMessage("workflow_event", func(msg protocol.Message) error {
		data := msg.Data
		if inner, ok := msg.Data["data"].(map[string]interface{}); ok {
			data = inner
		}
		ev := workflowEventMsg{}
		ev.runID, _ = data["run"].(string)
		ev.task, _ = data["task"].(string)
		ev.state, _ = data["state"].(string)
		ev.text, _ = data["text"].(string)
		// A run reaches every window of its workspace; this one shows it only when it
		// is in the project the window was launched from (an older gateway
		// sends no dir: its events were addressed to this window already).
		if dir, _ := data["dir"].(string); dir != "" && !sameDir(dir, launchDir()) {
			return nil
		}
		if st, ok := data["status"]; ok && st != nil {
			if raw, err := json.Marshal(st); err == nil {
				var run WorkflowRun
				if json.Unmarshal(raw, &run) == nil && run.ID != "" {
					ev.status = &run
				}
			}
		}
		if a.program != nil && (ev.text != "" || ev.status != nil) {
			a.program.Send(ev)
		}
		return nil
	})

	// Todo events (using string literal - not in protocol constants)
	a.client.OnMessage("todo", func(msg protocol.Message) error {
		a.handleTodoEvent(msg)
		return nil
	})

	// Note: Context events come as agent_event messages with stream="context"
	// They're handled by handleAgentEvent -> eventAdapter.OnAgentEvent
}

// SetProgram sets the Bubble Tea program for sending messages
func (a *WebSocketAdapter) SetProgram(program *tea.Program) {
	if program == nil {
		a.SetSink(nil)
		return
	}
	a.SetSink(program)
}

// SetSink sets where messages go: the program, or a page's tagging sender.
func (a *WebSocketAdapter) SetSink(s msgSender) {
	a.program = s
	if a.eventAdapter != nil {
		a.eventAdapter.SetSink(s)
	}
}

// SendMessage sends a chat message
// SendMessage sends a chat turn with the current Claude-style permission mode
// ("default"|"acceptEdits"|"plan"; empty = default).
func (a *WebSocketAdapter) SendMessage(text, permissionMode string) error {
	return a.client.SendChat(text, permissionMode)
}

// SendAnswer replies to an interactive ask_user_question.
func (a *WebSocketAdapter) SendAnswer(questionID, answer string) error {
	return a.client.SendAnswer(questionID, answer)
}

// SendCancel asks the gateway to interrupt the session's running turn.
func (a *WebSocketAdapter) SendCancel() error {
	return a.client.SendMessage(protocol.Message{Type: protocol.MessageTypeCancel})
}

// Close closes the WebSocket connection
func (a *WebSocketAdapter) Close() error {
	return a.client.Close()
}

// Message handlers - convert protocol messages to Bubble Tea messages

func (a *WebSocketAdapter) handleChatResponse(msg protocol.Message) {
	if a.program == nil {
		return
	}

	data, ok := msg.Data["data"].(map[string]interface{})
	if !ok {
		data = msg.Data // Try top-level data
	}

	// Check if queued
	if status, _ := data["status"].(string); status == "queued" {
		return
	}

	// Check for error
	if errMsg, ok := data["error"].(string); ok {
		a.program.Send(assistantResponseMsg{
			content: fmt.Sprintf("Error: %s", errMsg),
		})
		return
	}

	// Get response text
	if text, ok := data["text"].(string); ok && text != "" {
		a.program.Send(assistantResponseMsg{
			content: text,
		})
	}
}

func (a *WebSocketAdapter) handleAgentEvent(msg protocol.Message) {
	// Extract fields directly from msg.Data
	// Note: WebSocket sends timestamp as Unix milliseconds (int64), not RFC3339 string
	runID, _ := msg.Data["run_id"].(string)
	sessionID, _ := msg.Data["session_id"].(string)
	seq, _ := msg.Data["seq"].(float64)
	stream, _ := msg.Data["stream"].(string)
	timestamp, _ := msg.Data["timestamp"].(float64)
	eventData, _ := msg.Data["data"].(map[string]interface{})

	// Interactive ask_user_question: the agent blocked on a question — surface it
	// as a picker so the user can answer (the choice is sent back via SendAnswer).
	if stream == "question" && a.program != nil {
		qid, _ := eventData["question_id"].(string)
		question, _ := eventData["question"].(string)
		var options []string
		if raw, ok := eventData["options"].([]interface{}); ok {
			for _, o := range raw {
				if s, ok := o.(string); ok {
					options = append(options, s)
				}
			}
		}
		if qid != "" && question != "" && len(options) > 0 {
			a.program.Send(questionMsg{id: qid, question: question, options: options})
			return
		}
	}

	// exit_plan_mode: the agent finished planning and presented a plan. Render it as
	// a clean proposal block (the raw tool JSON never reaches the chat now).
	if stream == "plan" && a.program != nil {
		if plan, _ := eventData["plan"].(string); strings.TrimSpace(plan) != "" {
			a.program.Send(planProposalMsg{plan: plan})
			return
		}
	}

	// Create infra.AgentEvent for the consumer
	event := infra.AgentEvent{
		RunID:     runID,
		SessionID: sessionID,
		Seq:       int(seq),
		Stream:    infra.AgentEventStream(stream),
		Timestamp: int64(timestamp),
		Data:      eventData,
	}

	// Forward to event adapter for processing
	if a.eventAdapter != nil {
		a.eventAdapter.OnAgentEvent(event)
	}
}

func (a *WebSocketAdapter) handleTodoEvent(msg protocol.Message) {
	if a.program == nil {
		return
	}

	data, ok := msg.Data["data"].(map[string]interface{})
	if !ok {
		return
	}

	// Extract todos array
	todosData, ok := data["todos"].([]interface{})
	if !ok {
		return
	}

	// Parse todos
	var todos []TodoItem
	for _, todoData := range todosData {
		todoMap, ok := todoData.(map[string]interface{})
		if !ok {
			continue
		}

		content, _ := todoMap["content"].(string)
		activeForm, _ := todoMap["activeForm"].(string)
		status, _ := todoMap["status"].(string)

		todos = append(todos, TodoItem{
			Content:    content,
			ActiveForm: activeForm,
			Status:     status,
		})
	}

	// Send todo update to TUI
	a.program.Send(todoUpdateMsg{todos: todos})
}

// handleContextEvent is no longer needed - context events come as agent_event messages
// and are handled by handleAgentEvent -> eventAdapter.OnAgentEvent

// launchDir is the directory this window was started from: the project the
// coder works on, and the one whose workflow runs it shows.
func launchDir() string {
	wd, _ := os.Getwd()
	return wd
}

// sameDir compares two directories after resolving symlinks (/tmp is
// /private/tmp on macOS).
func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	if err1 != nil || err2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

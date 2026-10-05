package gateway

import (
	"context"
	"log/slog"
	"time"

	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/pkg/authorization"
	"memdoor/pkg/platform"
	"memdoor/pkg/shared"
)

// AgentExecutorAdapter adapts AgentRuntime to platform.AgentExecutor interface
//
// This adapter allows AgentRuntime (internal implementation) to be used
// through the Published Interface (platform.AgentExecutor) by downstream
// consumers like MessageService.
//
// Pattern: Adapter (Gang of Four)
type AgentExecutorAdapter struct {
	runtime     *AgentRuntime
	eventStream *inMemoryEventStream
	// Cancel hooks let a message-path run be interrupted (Esc): they register
	// the run's cancel func under its session key, where the ws "cancel" handler
	// (s.cancelRun(sessionKey)) finds it. Without this, Esc only stopped the UI
	// while the model kept churning. Injected by the gateway; nil = no-op.
	registerCancel func(sessionKey string, cancel context.CancelFunc)
	clearCancel    func(sessionKey string)
	// routes finds the REGISTERED session for a key. A message-path turn
	// runs on a fresh Session built from the message service's context
	// (sessionContextToGatewaySession), so anything set on the registered
	// one — the rung a person pinned with /route (route_handler.go) — has
	// to be carried over here or the turn never sees it (found with the
	// first pin, 2026-09-26). Required at construction: no lazy setter,
	// nothing silently not carried (Greg: "fail fast constructor").
	routes sessionLookup
}

// sessionLookup is the one method the adapter needs of the session registry.
type sessionLookup interface {
	GetSession(id string) (*Session, error)
}

// routeKeys are the session metadata that describe a conversation's rung
// (turn_tier.go, route_handler.go).
var routeKeys = []string{sessionTierKey, sessionPinnedKey, sessionReasonKey, sessionModelKey, sessionSortKey, sessionOrderKey, sessionEffortKey}

// carryRoute copies the registered session's rung onto the turn's session.
func (a *AgentExecutorAdapter) carryRoute(session *Session) {
	if session == nil || session.ID == "" {
		return
	}
	reg, err := a.routes.GetSession(session.ID)
	if err != nil || reg == nil || reg == session {
		return
	}
	for _, k := range routeKeys {
		if v, ok := reg.GetMetadataValue(k); ok {
			if session.Metadata == nil {
				session.Metadata = map[string]interface{}{}
			}
			session.SetMetadata(k, v)
		}
	}
}

// routeSink is the callback a turn reports its route results through (the
// effort it runs at and why): the turn runs on a copy of the session, so
// they go straight to the registered session /api/route reads, the moment
// the turn knows them — the footer showed "effort auto" after a turn at high
// while they sat on the copy (2026-09-30).
func (a *AgentExecutorAdapter) routeSink(session *Session) func(key string, v any) {
	if session == nil || session.ID == "" || a.routes == nil {
		return nil
	}
	reg, err := a.routes.GetSession(session.ID)
	if err != nil || reg == nil {
		return nil
	}
	return func(key string, v any) { reg.SetMetadata(key, v) }
}

type routeSinkKey struct{}

func withRouteSink(ctx context.Context, sink func(key string, v any)) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, routeSinkKey{}, sink)
}

// reportRoute hands a route result to the turn's sink, when it has one.
func reportRoute(ctx context.Context, key string, v any) {
	if sink, ok := ctx.Value(routeSinkKey{}).(func(key string, v any)); ok {
		sink(key, v)
	}
}

// SetCancelHooks wires run-cancellation so Esc actually stops a message-path run.
func (a *AgentExecutorAdapter) SetCancelHooks(reg func(string, context.CancelFunc), clr func(string)) {
	a.registerCancel = reg
	a.clearCancel = clr
}

// NewAgentExecutorAdapter creates an adapter that wraps AgentRuntime
func NewAgentExecutorAdapter(runtime *AgentRuntime, routes sessionLookup) *AgentExecutorAdapter {
	if runtime == nil || routes == nil {
		panic("NewAgentExecutorAdapter: runtime and session registry are required")
	}
	adapter := &AgentExecutorAdapter{
		runtime:     runtime,
		eventStream: newInMemoryEventStream(),
		routes:      routes,
	}

	// Bridge runtime events to Published Interface EventStream
	// Pattern: Subscribe to AgentRuntime.events and forward to adapter.eventStream
	// This enables Web UI to receive all events (including TodoWrite) via the Published Interface
	runtime.events.OnEvent(func(agentEvent infra.AgentEvent) {
		// Heartbeats are STATUS, not results. Every tool event on this bridge
		// becomes a tool.call.completed (below), so forwarding a beat every
		// five seconds would give the web UI an empty completed tool call
		// every five seconds. They drive the live status line, which reads
		// the broadcaster directly.
		if agentEvent.Stream == infra.EventStreamTool {
			switch agentEvent.Data["event"] {
			case toolEventProgress, subagentEvent:
				return
			}
		}

		// Convert infra.AgentEvent → shared.Event
		sharedEvent := adapter.convertAgentEventToSharedEvent(agentEvent)

		// Forward to Published Interface EventStream
		adapter.eventStream.Emit(agentEvent.RunID, sharedEvent)
	})

	return adapter
}

// ProcessMessage implements platform.AgentExecutor interface
//
// Adapts the call to AgentRuntime.ProcessMessage by:
// - Converting platform.SessionContext (interface) to *Session (concrete type)
// - Converting *AgentResponse to *platform.ExecutionResult
func (a *AgentExecutorAdapter) ProcessMessage(
	ctx context.Context,
	userMessage string,
	sessionCtx platform.SessionContext,
	runID string,
	extraSystemPrompt string,
) (*platform.ExecutionResult, error) {
	// Convert platform.SessionContext to gateway.Session
	session := a.sessionContextToGatewaySession(sessionCtx)
	a.carryRoute(session)
	if !sessionRouted(session) {
		// A conversation with no route of its own starts on the kept pin,
		// on the turn's copy and on the registered session the footer
		// reads (persistent_pin.go).
		var reg *Session
		if a.routes != nil && session.ID != "" {
			reg, _ = a.routes.GetSession(session.ID)
		}
		applyPersistentPin(string(authorization.GetActorID(ctx)), session, reg)
	}
	ctx = withRouteSink(ctx, a.routeSink(session))

	// Make this run cancellable via Esc: register its cancel under the session
	// key the websocket subscribes to, so the ws "cancel" message stops it.
	if a.registerCancel != nil && session.ID != "" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		a.registerCancel(session.ID, cancel)
		defer func() {
			a.clearCancel(session.ID)
			cancel()
		}()
	}

	// Call actual runtime
	response, err := a.runtime.ProcessMessage(ctx, userMessage, session, runID, extraSystemPrompt)
	if err != nil {
		return &platform.ExecutionResult{
			Text:  "",
			Error: err.Error(),
			Metadata: map[string]interface{}{
				"error": err.Error(),
			},
		}, err
	}

	// Convert AgentResponse to platform.ExecutionResult
	result := a.agentResponseToExecutionResult(response)
	return result, nil
}

// EventStream implements platform.AgentExecutor interface
func (a *AgentExecutorAdapter) EventStream() platform.EventStream {
	return a.eventStream
}

// sessionContextToGatewaySession converts Published Interface type to internal type
func (a *AgentExecutorAdapter) sessionContextToGatewaySession(ctx platform.SessionContext) *Session {
	if ctx == nil {
		// Return minimal session
		return &Session{
			ID:       "default",
			Type:     "default",
			Messages: []SessionMessage{},
			Metadata: map[string]interface{}{},
		}
	}

	// Convert platform.SessionMessage[] to gateway.SessionMessage[]
	messages := make([]SessionMessage, 0, len(ctx.GetMessages()))
	for _, msg := range ctx.GetMessages() {
		// Convert Unix milliseconds (int64) to time.Time
		timestamp := time.Unix(0, msg.Timestamp*int64(time.Millisecond))
		messages = append(messages, SessionMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: timestamp,
		})
	}

	// Create gateway.Session
	metadata := ctx.GetMetadata()
	session := &Session{
		ID:       ctx.GetID(),
		Type:     ctx.GetType(),
		Messages: messages,
		Metadata: metadata,
	}

	// DEBUG: Log metadata transfer
	log := logs.New("AgentAdapter")
	log.Debug("Converting SessionContext to gateway.Session",
		slog.String("session_id", session.ID),
		slog.Any("metadata", metadata),
		slog.Int("metadata_count", len(metadata)))

	return session
}

// agentResponseToExecutionResult converts internal type to Published Interface type
func (a *AgentExecutorAdapter) agentResponseToExecutionResult(response *AgentResponse) *platform.ExecutionResult {
	if response == nil {
		return &platform.ExecutionResult{
			Text:          "",
			ToolsExecuted: []platform.ToolExecution{},
			Error:         "nil response",
			Metadata:      map[string]interface{}{},
		}
	}

	// Convert ToolExecutionInfo[] to platform.ToolExecution[]
	tools := make([]platform.ToolExecution, 0, len(response.ToolsExecuted))
	for _, tool := range response.ToolsExecuted {
		tools = append(tools, platform.ToolExecution{
			Name:       tool.Name,
			Input:      tool.Input,
			Output:     tool.Output,
			Error:      tool.Error,
			DurationMs: tool.DurationMs,
		})
	}

	return &platform.ExecutionResult{
		Text:          response.Text,
		ToolsExecuted: tools,
		Error:         response.Error,
		Model:         response.Model,
		Metadata: map[string]interface{}{
			"model":       response.Model,
			"tools_count": len(tools),
		},
	}
}

// inMemoryEventStream implements platform.EventStream with in-memory pub/sub
type inMemoryEventStream struct {
	subscribers map[string][]func(event shared.Event)
}

func newInMemoryEventStream() *inMemoryEventStream {
	return &inMemoryEventStream{
		subscribers: make(map[string][]func(event shared.Event)),
	}
}

// Subscribe implements platform.EventStream.Subscribe
func (s *inMemoryEventStream) Subscribe(runID string, listener func(event shared.Event)) func() {
	// Register listener
	s.subscribers[runID] = append(s.subscribers[runID], listener)

	// Return unsubscribe function
	unsubscribed := false
	return func() {
		if !unsubscribed {
			// Remove this listener (simplified - removes all for runID)
			delete(s.subscribers, runID)
			unsubscribed = true
		}
	}
}

// Emit implements platform.EventStream.Emit
func (s *inMemoryEventStream) Emit(runID string, event shared.Event) {
	// Call all listeners for this runID
	for _, listener := range s.subscribers[runID] {
		listener(event)
	}
}

// convertAgentEventToSharedEvent converts infra.AgentEvent → shared.Event
// This bridges the internal event system to the Published Interface event system
func (a *AgentExecutorAdapter) convertAgentEventToSharedEvent(agentEvent infra.AgentEvent) shared.Event {
	// Map infra.AgentEventStream → shared.EventType + EventCategory
	var eventType shared.EventType
	var category shared.EventCategory

	switch agentEvent.Stream {
	case infra.EventStreamLifecycle:
		category = shared.EventCategoryLifecycle
		// Check data for specific lifecycle event type
		if status, ok := agentEvent.Data["status"].(string); ok {
			switch status {
			case "started":
				eventType = shared.EventExecutionStarted
			case "completed":
				eventType = shared.EventExecutionCompleted
			case "failed":
				eventType = shared.EventExecutionFailed
			default:
				eventType = shared.EventExecutionStarted // Default
			}
		} else {
			eventType = shared.EventExecutionStarted // Default
		}

	case infra.EventStreamTool:
		category = shared.EventCategoryTool
		// Check data for todo type (special handling for TodoWrite)
		if eventTypeStr, ok := agentEvent.Data["type"].(string); ok && eventTypeStr == "todo" {
			// This is a TodoWrite event - use a custom event type
			eventType = shared.EventToolCallCompleted
			// Keep the "type": "todo" in data so subscribers can identify it
		} else {
			// Generic tool event
			eventType = shared.EventToolCallCompleted
		}

	case infra.EventStreamAssistant:
		category = shared.EventCategoryResponse
		eventType = shared.EventResponseGenerated

	case infra.EventStreamError:
		category = shared.EventCategoryError
		eventType = shared.EventErrorOccurred

	case infra.EventStreamContext:
		category = shared.EventCategoryThinking
		eventType = shared.EventThinkingStarted

	default:
		category = shared.EventCategoryLifecycle
		eventType = shared.EventExecutionStarted
	}

	// Create shared.Event with converted type and original data
	return shared.Event{
		Type:      eventType,
		Category:  category,
		Data:      agentEvent.Data, // Forward all data unchanged
		Timestamp: time.Unix(0, agentEvent.Timestamp*int64(time.Millisecond)),
	}
}

// Ensure AgentExecutorAdapter implements platform.AgentExecutor at compile time
var _ platform.AgentExecutor = (*AgentExecutorAdapter)(nil)

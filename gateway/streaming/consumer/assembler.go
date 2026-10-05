package consumer

import (
	"strings"
	"sync"
	"time"

	"memdoor/gateway/infra"
)

// StreamAssembler builds text incrementally from delta events and tracks tool execution state
// Pattern: OpenClaw's TUIStreamAssembler - maintains per-run state
type StreamAssembler struct {
	runStates map[string]*RunState
	mu        sync.RWMutex
	maxStates int
	verbose   bool
}

// NewStreamAssembler creates a new stream assembler
func NewStreamAssembler(maxStates int, verbose bool) *StreamAssembler {
	if maxStates <= 0 {
		maxStates = 100 // Default limit
	}

	return &StreamAssembler{
		runStates: make(map[string]*RunState),
		maxStates: maxStates,
		verbose:   verbose,
	}
}

// ProcessEvent processes an agent event and updates run state
// Returns UpdateResult describing what changed
func (sa *StreamAssembler) ProcessEvent(event infra.AgentEvent, parsedData ParsedEventData) (*UpdateResult, error) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	runID := event.RunID
	sessionID := event.SessionID

	// Get or create run state
	state, exists := sa.runStates[runID]
	if !exists {
		state = &RunState{
			RunID:       runID,
			SessionID:   sessionID,
			Text:        "",
			ToolEvents:  make(map[string]*ToolState),
			IsThinking:  false,
			LastUpdated: time.Now(),
			Finalized:   false,
		}
		sa.runStates[runID] = state

		// Enforce max states limit
		if len(sa.runStates) > sa.maxStates {
			sa.pruneOldest()
		}
	}

	// Process based on event type
	result := &UpdateResult{
		RunID:      runID,
		TextDelta:  "",
		FullText:   state.Text,
		ToolUpdate: nil,
		EventType:  GetEventType(parsedData),
	}

	switch data := parsedData.(type) {
	case AssistantEventData:
		sa.processAssistantEvent(state, data, result)

	case ToolEventData:
		sa.processToolEvent(state, data, result)

	case LifecycleEventData:
		sa.processLifecycleEvent(state, data, result)

	case ErrorEventData:
		sa.processErrorEvent(state, data, result)
	}

	// Update state timestamp
	state.LastUpdated = time.Now()

	// Set finalized flag in result
	result.Finalized = state.Finalized
	result.ThinkingState = state.IsThinking

	return result, nil
}

// processAssistantEvent handles assistant stream events
func (sa *StreamAssembler) processAssistantEvent(state *RunState, data AssistantEventData, result *UpdateResult) {
	switch data.Event {
	case "thinking":
		state.IsThinking = true
		result.ThinkingState = true

	case "text_start":
		state.IsThinking = false
		result.ThinkingState = false

	case "text_delta":
		// Append delta to accumulated text. Chunk() reads whichever key the
		// emitter used — "delta" today, "text" historically.
		chunk := data.Chunk()
		state.Text += chunk
		result.TextDelta = chunk
		result.FullText = state.Text
		state.IsThinking = false
		result.ThinkingState = false

	case "text_end", "text":
		// Final text: the whole reply, or (append) only what did not stream.
		if data.Text != "" {
			if data.Append && state.Text != "" {
				state.Text = strings.TrimRight(state.Text, "\n") + "\n\n" + data.Text
			} else {
				state.Text = data.Text
			}
			result.TextDelta = data.Text
			result.FullText = data.Text
			result.TextAppend = data.Append
		}
		state.IsThinking = false
		result.ThinkingState = false
	}
}

// processToolEvent handles tool stream events
func (sa *StreamAssembler) processToolEvent(state *RunState, data ToolEventData, result *UpdateResult) {
	toolName := data.Tool
	if toolName == "" {
		return
	}

	switch data.Event {
	case "start":
		// Tool execution started
		toolState := &ToolState{
			Name:      toolName,
			Status:    "running",
			Input:     data.Input,
			Output:    "",
			Error:     "",
			StartTime: time.Now(),
			EndTime:   nil,
		}
		state.ToolEvents[toolName] = toolState
		result.ToolUpdate = toolState

	case "update":
		// Tool execution update (progress)
		if toolState, exists := state.ToolEvents[toolName]; exists {
			toolState.Output = data.Output
			result.ToolUpdate = toolState
		}

	case "complete", "result":
		// Tool execution completed
		if toolState, exists := state.ToolEvents[toolName]; exists {
			toolState.Status = "complete"
			toolState.Output = data.Output
			toolState.Error = data.Error
			now := time.Now()
			toolState.EndTime = &now

			// If there's an error, mark as error status
			if data.Error != "" {
				toolState.Status = "error"
			}

			result.ToolUpdate = toolState
		}
	}
}

// processLifecycleEvent handles lifecycle stream events
func (sa *StreamAssembler) processLifecycleEvent(state *RunState, data LifecycleEventData, result *UpdateResult) {
	switch data.Event {
	case "start":
		// Run started - state already initialized

	case "complete", "end":
		// Run completed successfully
		state.Finalized = true
		result.Finalized = true

	case "error":
		// Run failed
		state.Finalized = true
		result.Finalized = true
	}
}

// processErrorEvent handles error stream events
func (sa *StreamAssembler) processErrorEvent(state *RunState, data ErrorEventData, result *UpdateResult) {
	// Mark run as finalized on error
	state.Finalized = true
	result.Finalized = true
}

// GetRunState returns the current state for a run (read-only copy)
func (sa *StreamAssembler) GetRunState(runID string) *RunState {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	state, exists := sa.runStates[runID]
	if !exists {
		return nil
	}

	// Return a copy to avoid concurrent modification
	stateCopy := &RunState{
		RunID:       state.RunID,
		SessionID:   state.SessionID,
		Text:        state.Text,
		ToolEvents:  make(map[string]*ToolState),
		IsThinking:  state.IsThinking,
		LastUpdated: state.LastUpdated,
		Finalized:   state.Finalized,
	}

	// Deep copy tool events
	for k, v := range state.ToolEvents {
		toolCopy := *v
		stateCopy.ToolEvents[k] = &toolCopy
	}

	return stateCopy
}

// ClearRun removes a run from the assembler state
func (sa *StreamAssembler) ClearRun(runID string) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	delete(sa.runStates, runID)
}

// PruneOldRuns removes runs older than the given duration
func (sa *StreamAssembler) PruneOldRuns(maxAge time.Duration) int {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	now := time.Now()
	pruned := 0

	for runID, state := range sa.runStates {
		if now.Sub(state.LastUpdated) > maxAge {
			delete(sa.runStates, runID)
			pruned++
		}
	}

	return pruned
}

// pruneOldest removes the oldest run state when limit is exceeded
func (sa *StreamAssembler) pruneOldest() {
	if len(sa.runStates) == 0 {
		return
	}

	var oldestID string
	var oldestTime time.Time

	for runID, state := range sa.runStates {
		if oldestID == "" || state.LastUpdated.Before(oldestTime) {
			oldestID = runID
			oldestTime = state.LastUpdated
		}
	}

	if oldestID != "" {
		delete(sa.runStates, oldestID)
	}
}

// GetActiveRuns returns a list of all active (non-finalized) run IDs
func (sa *StreamAssembler) GetActiveRuns() []string {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	var activeRuns []string
	for runID, state := range sa.runStates {
		if !state.Finalized {
			activeRuns = append(activeRuns, runID)
		}
	}

	return activeRuns
}

// GetStats returns statistics about the assembler state
func (sa *StreamAssembler) GetStats() map[string]interface{} {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	activeCount := 0
	finalizedCount := 0
	totalTools := 0

	for _, state := range sa.runStates {
		if state.Finalized {
			finalizedCount++
		} else {
			activeCount++
		}
		totalTools += len(state.ToolEvents)
	}

	return map[string]interface{}{
		"total_runs":     len(sa.runStates),
		"active_runs":    activeCount,
		"finalized_runs": finalizedCount,
		"total_tools":    totalTools,
		"max_states":     sa.maxStates,
	}
}

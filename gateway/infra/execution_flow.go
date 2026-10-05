package infra

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ExecutionFlowNode represents a single tool execution in the flow
type ExecutionFlowNode struct {
	ID        string                 `json:"id"`         // Unique ID for this node
	ToolName  string                 `json:"tool_name"`  // Name of the tool executed
	Seq       int                    `json:"seq"`        // Sequence number in execution
	Input     map[string]interface{} `json:"input"`      // Tool input parameters
	Output    string                 `json:"output"`     // Tool output (truncated)
	Error     string                 `json:"error"`      // Error message if failed
	StartTime int64                  `json:"start_time"` // Unix timestamp (ms)
	EndTime   int64                  `json:"end_time"`   // Unix timestamp (ms)
	Duration  int64                  `json:"duration"`   // Duration in milliseconds
	Success   bool                   `json:"success"`    // Whether execution succeeded
}

// ExecutionFlow represents the complete execution flow for a run
type ExecutionFlow struct {
	RunID     string              `json:"run_id"`
	SessionID string              `json:"session_id"`
	Nodes     []ExecutionFlowNode `json:"nodes"`
	StartTime int64               `json:"start_time"`
	EndTime   int64               `json:"end_time"`
	mu        sync.RWMutex        `json:"-"`
}

// ExecutionFlowTracker tracks tool execution flows
// Pattern: Captures tool execution sequences for visualization
type ExecutionFlowTracker struct {
	flows   map[string]*ExecutionFlow     // Map of runID -> ExecutionFlow
	pending map[string]*ExecutionFlowNode // Map of toolID -> pending node
	mu      sync.RWMutex
	verbose bool
}

// NewExecutionFlowTracker creates a new execution flow tracker
func NewExecutionFlowTracker(verbose bool) *ExecutionFlowTracker {
	return &ExecutionFlowTracker{
		flows:   make(map[string]*ExecutionFlow),
		pending: make(map[string]*ExecutionFlowNode),
		verbose: verbose,
	}
}

// Start begins tracking a new run
func (eft *ExecutionFlowTracker) Start(runID, sessionID string) {
	eft.mu.Lock()
	defer eft.mu.Unlock()

	eft.flows[runID] = &ExecutionFlow{
		RunID:     runID,
		SessionID: sessionID,
		Nodes:     make([]ExecutionFlowNode, 0),
		StartTime: time.Now().UnixMilli(),
	}
}

// RecordToolStart records the start of a tool execution
func (eft *ExecutionFlowTracker) RecordToolStart(runID, toolName string, seq int, input string) {
	eft.mu.Lock()
	defer eft.mu.Unlock()

	// Lazy-register the flow record on first tool start. The flow's
	// Nodes slice isn't populated here — RecordToolComplete moves the
	// finished node out of pending — so we only need the side effect
	// of the map write, never the value.
	if _, exists := eft.flows[runID]; !exists {
		eft.flows[runID] = &ExecutionFlow{
			RunID:     runID,
			SessionID: "",
			Nodes:     make([]ExecutionFlowNode, 0),
			StartTime: time.Now().UnixMilli(),
		}
	}

	// Parse input JSON
	var inputMap map[string]interface{}
	if input != "" {
		if err := json.Unmarshal([]byte(input), &inputMap); err != nil {
			inputMap = map[string]interface{}{"raw": input}
		}
	}

	// Create pending node
	nodeID := fmt.Sprintf("%s-%d", runID, seq)
	node := &ExecutionFlowNode{
		ID:        nodeID,
		ToolName:  toolName,
		Seq:       seq,
		Input:     inputMap,
		StartTime: time.Now().UnixMilli(),
	}

	eft.pending[nodeID] = node
}

// RecordToolComplete records the completion of a tool execution
func (eft *ExecutionFlowTracker) RecordToolComplete(runID, toolName string, seq int, output, errMsg string) {
	eft.mu.Lock()
	defer eft.mu.Unlock()

	nodeID := fmt.Sprintf("%s-%d", runID, seq)
	node, exists := eft.pending[nodeID]
	if !exists {
		// Tool complete without start - create node anyway
		node = &ExecutionFlowNode{
			ID:        nodeID,
			ToolName:  toolName,
			Seq:       seq,
			StartTime: time.Now().UnixMilli(),
		}
	}

	// Complete the node
	node.EndTime = time.Now().UnixMilli()
	node.Duration = node.EndTime - node.StartTime
	node.Output = truncateString(output, 200)
	node.Error = errMsg
	node.Success = errMsg == ""

	// Add to flow
	flow, exists := eft.flows[runID]
	if exists {
		flow.mu.Lock()
		flow.Nodes = append(flow.Nodes, *node)
		flow.mu.Unlock()
	}

	// Remove from pending
	delete(eft.pending, nodeID)
}

// Complete marks a run as complete
func (eft *ExecutionFlowTracker) Complete(runID string) {
	eft.mu.Lock()
	defer eft.mu.Unlock()

	flow, exists := eft.flows[runID]
	if exists {
		flow.mu.Lock()
		flow.EndTime = time.Now().UnixMilli()
		flow.mu.Unlock()
	}
}

// GetFlow retrieves the execution flow for a run
func (eft *ExecutionFlowTracker) GetFlow(runID string) (*ExecutionFlow, error) {
	eft.mu.RLock()
	defer eft.mu.RUnlock()

	flow, exists := eft.flows[runID]
	if !exists {
		return nil, fmt.Errorf("no execution flow found for run %s", runID)
	}

	// Return a copy to prevent concurrent modification
	flow.mu.RLock()
	defer flow.mu.RUnlock()

	flowCopy := &ExecutionFlow{
		RunID:     flow.RunID,
		SessionID: flow.SessionID,
		Nodes:     make([]ExecutionFlowNode, len(flow.Nodes)),
		StartTime: flow.StartTime,
		EndTime:   flow.EndTime,
	}
	copy(flowCopy.Nodes, flow.Nodes)

	return flowCopy, nil
}

// GetLatestFlow retrieves the most recent execution flow
func (eft *ExecutionFlowTracker) GetLatestFlow() (*ExecutionFlow, error) {
	eft.mu.RLock()
	defer eft.mu.RUnlock()

	if len(eft.flows) == 0 {
		return nil, fmt.Errorf("no execution flows recorded")
	}

	// Find the flow with the latest start time
	var latestFlow *ExecutionFlow
	var latestTime int64

	for _, flow := range eft.flows {
		flow.mu.RLock()
		if flow.StartTime > latestTime {
			latestTime = flow.StartTime
			latestFlow = flow
		}
		flow.mu.RUnlock()
	}

	if latestFlow == nil {
		return nil, fmt.Errorf("no execution flows found")
	}

	// Return a copy
	latestFlow.mu.RLock()
	defer latestFlow.mu.RUnlock()

	flowCopy := &ExecutionFlow{
		RunID:     latestFlow.RunID,
		SessionID: latestFlow.SessionID,
		Nodes:     make([]ExecutionFlowNode, len(latestFlow.Nodes)),
		StartTime: latestFlow.StartTime,
		EndTime:   latestFlow.EndTime,
	}
	copy(flowCopy.Nodes, latestFlow.Nodes)

	return flowCopy, nil
}

// Clear removes the flow for a run (called after run completes)
func (eft *ExecutionFlowTracker) Clear(runID string) {
	eft.mu.Lock()
	defer eft.mu.Unlock()

	delete(eft.flows, runID)
}

// ClearAll removes all flows (useful for testing)
func (eft *ExecutionFlowTracker) ClearAll() {
	eft.mu.Lock()
	defer eft.mu.Unlock()

	eft.flows = make(map[string]*ExecutionFlow)
	eft.pending = make(map[string]*ExecutionFlowNode)
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ToMermaidFlowchart converts an execution flow to a Mermaid flowchart
func (flow *ExecutionFlow) ToMermaidFlowchart() string {
	if len(flow.Nodes) == 0 {
		return "flowchart TB\n    Start([No tools executed])\n"
	}

	var mermaid string
	mermaid += "flowchart TB\n"
	mermaid += "    Start([Start])\n"

	// Add nodes
	for i, node := range flow.Nodes {
		nodeID := fmt.Sprintf("N%d", i)
		label := node.ToolName

		// Add duration if available
		if node.Duration > 0 {
			label += fmt.Sprintf("\\n%.1fs", float64(node.Duration)/1000.0)
		}

		// Choose shape based on success
		if node.Success {
			mermaid += fmt.Sprintf("    %s[%s]\n", nodeID, label)
		} else {
			mermaid += fmt.Sprintf("    %s[/%s/]\n", nodeID, label+" ❌")
		}
	}

	mermaid += "    End([End])\n\n"

	// Add edges
	mermaid += "    Start --> N0\n"
	for i := 0; i < len(flow.Nodes)-1; i++ {
		mermaid += fmt.Sprintf("    N%d --> N%d\n", i, i+1)
	}
	if len(flow.Nodes) > 0 {
		mermaid += fmt.Sprintf("    N%d --> End\n", len(flow.Nodes)-1)
	}

	return mermaid
}

// ToJSON converts an execution flow to JSON
func (flow *ExecutionFlow) ToJSON() ([]byte, error) {
	flow.mu.RLock()
	defer flow.mu.RUnlock()

	return json.MarshalIndent(flow, "", "  ")
}

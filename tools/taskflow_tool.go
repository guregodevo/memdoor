package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"memdoor/gateway/logs"
)

// =============================================================================
// task_flow tool - Start a deterministic multi-step managed flow
// =============================================================================
//
// Pattern: OpenClaw Task Flow (managed mode). The calling agent (e.g. the
// planner) decomposes a goal into an ordered list of small, self-contained
// steps ONCE, then hands that plan to task_flow. Deterministic code then drives
// the steps to completion — dispatching each as one isolated doer subagent and
// advancing on completion — so the model never has to hold the multi-step loop.

// FlowStarter starts a managed flow. Duck-typed to avoid importing the gateway
// package (which owns the flow registry + queue).
type FlowStarter interface {
	StartFlow(ownerKey, goal string, steps []string, parentMessageID int64) (flowID string, err error)
}

// TaskFlowTool holds dependencies for the task_flow tool.
type TaskFlowTool struct {
	starter FlowStarter
	log     *logs.EventLogger
}

// NewTaskFlowTool creates a new task_flow tool.
func NewTaskFlowTool(starter FlowStarter) *TaskFlowTool {
	return &TaskFlowTool{
		starter: starter,
		log:     logs.New("Agent"),
	}
}

// TaskFlowInput defines the parameters for starting a managed flow.
type TaskFlowInput struct {
	Goal  string   `json:"goal" jsonschema_description:"The overall goal this flow accomplishes, in one sentence (e.g. 'build a Tetris game')."`
	Steps []string `json:"steps" jsonschema_description:"Ordered list of small, self-contained steps. Each step is handed to a fresh isolated coder subagent, so it MUST reference shared state on disk (e.g. 'read the game file and add X'), not prior conversation. Keep each step to one concrete change."`
}

// TaskFlowInputSchema is the generated JSON schema for the tool input.
var TaskFlowInputSchema = GenerateSchema[TaskFlowInput]()

// TaskFlowDefinition is the tool definition for task_flow.
var TaskFlowDefinition = ToolDefinition{
	Name: "task_flow",
	Description: `Start a deterministic multi-step task flow.

Use this to build something that takes several steps. You decompose the goal into
an ordered list of small steps ONCE; the system then drives them to completion —
running each step as a focused coder subagent and moving to the next automatically.
You do NOT loop or re-dispatch yourself.

Each step:
- Runs in a fresh, isolated coder session (no memory of the other steps)
- So it MUST be self-contained and refer to shared state on disk
  (e.g. "read the game file and add a Piece type"), not "continue from before"
- Should make ONE concrete, verifiable change
- Uses whatever language the task asks for; run each step's file with that
  language's standard command (go run, python3, node, ...)

Example (goal "build Tetris in Python"):
  steps: [
    "Create tetris.py: a Board class holding a 20x10 grid and a main() that prints the empty board",
    "Read tetris.py and add a Piece class plus spawn() that places an I-piece at the top",
    "Read tetris.py and add drop() that moves the active piece down and locks it into the board",
    "Read tetris.py and add clear_lines() that removes full rows"
  ]`,
	InputSchema: TaskFlowInputSchema,
	Function:    nil, // Execution handled specially in executeTool()
}

// ExecuteWithSession starts the managed flow, owned by the calling session.
func (t *TaskFlowTool) ExecuteWithSession(input json.RawMessage, session Session) (string, error) {
	var params TaskFlowInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid task_flow input: %w", err)
	}
	if len(params.Steps) == 0 {
		return "", fmt.Errorf("task_flow requires a non-empty steps list")
	}

	ownerKey := session.GetKey()

	var parentMessageID int64
	if meta := session.GetMetadata(); meta != nil {
		if v, ok := meta["parent_message_id"]; ok {
			switch id := v.(type) {
			case int64:
				parentMessageID = id
			case float64:
				parentMessageID = int64(id)
			case int:
				parentMessageID = int64(id)
			}
		}
	}

	flowID, err := t.starter.StartFlow(ownerKey, params.Goal, params.Steps, parentMessageID)
	if err != nil {
		return "", fmt.Errorf("failed to start flow: %w", err)
	}

	t.log.Info("task_flow started", slog.String("flow_id", flowID), slog.Int("steps", len(params.Steps)))

	return fmt.Sprintf("Started task flow %s with %d steps. The steps will run in sequence automatically; I do not need to dispatch them myself. I'll report the result when the flow completes.", flowID, len(params.Steps)), nil
}

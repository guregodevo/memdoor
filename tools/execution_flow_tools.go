package tools

import (
	"encoding/json"
	"fmt"
)

// ExecutionFlowInput defines the input for the execution_flow tool
type ExecutionFlowInput struct {
	RunID  string `json:"run_id"` // Optional: specific run ID to visualize
	Format string `json:"format"` // mermaid, json (default: mermaid)
}

var ExecutionFlowInputSchema = GenerateSchema[ExecutionFlowInput]()

// ExecutionFlowDefinition defines the execution_flow tool
var ExecutionFlowDefinition = ToolDefinition{
	Name: "execution_flow",
	Description: `Visualize the execution flow of tool calls from an agent run.

Shows the sequence of tools that were executed, their timing, and success/failure status.
Useful for understanding what the agent did and debugging complex workflows.

Output Formats:
- mermaid: Generates a Mermaid flowchart showing execution sequence (default)
- json: Returns detailed JSON with all execution data

Parameters:
- run_id (optional): Specific run ID to visualize. If not provided, shows the most recent run.
- format: Output format - "mermaid" or "json" (default: "mermaid")

Example:
{
  "format": "mermaid"
}

The Mermaid output shows:
- Each tool as a node in sequence
- Execution duration next to tool name
- Failed tools marked with ❌
- Complete execution flow from Start to End

Example Mermaid output:
flowchart TB
    Start([Start])
    N0[Read\n0.5s]
    N1[Grep\n1.2s]
    N2[Edit\n0.8s]
    End([End])

    Start --> N0
    N0 --> N1
    N1 --> N2
    N2 --> End`,
	InputSchema: ExecutionFlowInputSchema,
	Function:    nil, // Will be set at runtime with access to flowTracker
}

// CreateExecutionFlowFunction creates the execution flow function with runtime dependencies
// This is called during agent runtime initialization
func CreateExecutionFlowFunction(getFlowTracker func() interface{}) func(json.RawMessage) (string, error) {
	return func(input json.RawMessage) (string, error) {
		var params ExecutionFlowInput

		if err := json.Unmarshal(input, &params); err != nil {
			return "", fmt.Errorf("invalid input: %w", err)
		}

		// Default format to mermaid
		if params.Format == "" {
			params.Format = "mermaid"
		}

		// Get flow tracker from runtime
		tracker := getFlowTracker()
		if tracker == nil {
			return "", fmt.Errorf("execution flow tracker not available")
		}

		// Type assert to ExecutionFlowTracker interface
		type FlowGetter interface {
			GetFlow(runID string) (interface{}, error)
			GetLatestFlow() (interface{}, error)
		}

		flowGetter, ok := tracker.(FlowGetter)
		if !ok {
			return "", fmt.Errorf("invalid flow tracker type")
		}

		// Get flow
		var flow interface{}
		var err error

		if params.RunID != "" {
			flow, err = flowGetter.GetFlow(params.RunID)
		} else {
			flow, err = flowGetter.GetLatestFlow()
		}

		if err != nil {
			return "", fmt.Errorf("failed to get execution flow: %w", err)
		}

		// Format output based on requested format
		switch params.Format {
		case "json":
			// Convert to JSON
			type JSONFlow interface {
				ToJSON() ([]byte, error)
			}
			jsonFlow, ok := flow.(JSONFlow)
			if !ok {
				// Fallback to standard JSON marshaling
				data, err := json.MarshalIndent(flow, "", "  ")
				if err != nil {
					return "", fmt.Errorf("failed to marshal flow: %w", err)
				}
				return string(data), nil
			}
			data, err := jsonFlow.ToJSON()
			if err != nil {
				return "", fmt.Errorf("failed to serialize flow: %w", err)
			}
			return string(data), nil

		case "mermaid":
			// Convert to Mermaid flowchart
			type MermaidFlow interface {
				ToMermaidFlowchart() string
			}
			mermaidFlow, ok := flow.(MermaidFlow)
			if !ok {
				return "", fmt.Errorf("flow does not support Mermaid export")
			}
			mermaid := mermaidFlow.ToMermaidFlowchart()
			return fmt.Sprintf("Execution Flow Diagram:\n\n%s", mermaid), nil

		default:
			return "", fmt.Errorf("unsupported format: %s (use 'mermaid' or 'json')", params.Format)
		}
	}
}

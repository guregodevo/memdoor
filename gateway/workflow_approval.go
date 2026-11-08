package gateway

import (
	"encoding/json"
	"fmt"
	"strings"
)

// workflowApproval describes the authority delegated by one workflow call.
// Execution approval covers that invocation, not later runs or human gates.
type workflowApproval struct {
	summary string
	gate    bool
}

func workflowApprovalFor(input json.RawMessage) (workflowApproval, error) {
	var in struct {
		Action string `json:"action"`
		Name   string `json:"name"`
		RunID  string `json:"run_id"`
		Task   string `json:"task"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return workflowApproval{}, fmt.Errorf("invalid workflow input: %w", err)
	}
	label := func(s string) string {
		s = strings.Join(strings.Fields(s), " ")
		runes := []rune(s)
		if len(runes) > 160 {
			return string(runes[:160]) + "..."
		}
		return s
	}
	switch strings.ToLower(strings.TrimSpace(in.Action)) {
	case "list", "status", "stop", "cancel":
		return workflowApproval{}, nil
	case "run", "start":
		if strings.TrimSpace(in.Name) == "" {
			return workflowApproval{}, fmt.Errorf("name is required: which workflow?")
		}
		return workflowApproval{summary: fmt.Sprintf("Run workflow %q: its steps may execute commands, change files, and make model calls", label(in.Name))}, nil
	case "resume", "continue":
		if strings.TrimSpace(in.RunID) == "" {
			return workflowApproval{}, fmt.Errorf("run_id is required: which workflow run?")
		}
		return workflowApproval{summary: fmt.Sprintf("Resume workflow run %q: its remaining steps may execute commands, change files, and make model calls", label(in.RunID))}, nil
	case "approve":
		if strings.TrimSpace(in.RunID) == "" || strings.TrimSpace(in.Task) == "" {
			return workflowApproval{}, fmt.Errorf("run_id and task are required: which workflow review gate?")
		}
		return workflowApproval{
			summary: fmt.Sprintf("Approve review gate %q on workflow run %q: dependent steps may then execute commands, change files, and make model calls", label(in.Task), label(in.RunID)),
			gate:    true,
		}, nil
	default:
		// New actions must be classified before an agent can invoke them.
		return workflowApproval{}, fmt.Errorf("unknown workflow action %q", in.Action)
	}
}

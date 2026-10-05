package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"memdoor/pkg/authorization"
	"memdoor/pkg/llm"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

// The workflow tool: the agent runs, watches, stops and approves DAGs the
// way the TUI and the shell do (Greg, 2026-10-02: "the coder agent can stop
// a workflow … gracefully"). Same calls, no logic of its own — mario
// orchestrates, the gateway reports.

const workflowToolName = "workflow"

// workflowOps is what the tool needs from the server, declared here so the
// runtime does not hold the server.
type workflowOps interface {
	workflowList(dir string) ([]workflowRunJSON, []string)
	startWorkflow(dir, name, session, workspace, actor, partition string, timeout time.Duration) (*workflowRun, error)
	workflowStatus(id string) (*workflowRun, error)
	stopWorkflow(id string) (*workflowRun, error)
	resumeWorkflow(id, session, dir, workspace, actor string) (*workflowRun, error)
	approveWorkflowTask(id, task string) (*workflowRun, error)
}

type workflowToolInput struct {
	Action    string `json:"action"`
	Name      string `json:"name"`
	RunID     string `json:"run_id"`
	Task      string `json:"task"`
	Partition string `json:"partition"`
}

// turnWorkflowTool is the workflow tool for THIS turn — it runs in this
// turn's project and reports into this turn's window — or nil when the
// palette does not carry it.
func (ar *AgentRuntime) turnWorkflowTool(ctx context.Context) *tools.ToolDefinition {
	if ar.workflows == nil || !ar.paletteHas(ctx, workflowToolName) {
		return nil
	}
	workdir, _ := ctx.Value(sharedctx.WorkdirKey).(string)
	session, _ := ctx.Value(sharedctx.SessionIDKey).(string)
	t := ar.workflowTool(session, workdir, string(authorization.GetActorID(ctx)))
	return &t
}

func (ar *AgentRuntime) workflowTool(session, workdir, actor string) tools.ToolDefinition {
	return tools.ToolDefinition{
		Name: workflowToolName,
		Description: "A DAG of agent tasks in this project, one YAML per task at .memdoor/workflows/<name>/<group>/<task>.yaml " +
			"(or any directory of task YAML you wrote). `run` is a fresh run; `resume` continues a failed or stopped run — what it finished is skipped. Before WRITING one, load the `workflow` skill: it has the file " +
			"format, the rules (a target is a check; a person's approval is an external requirement, never a file) and an example. " +
			"`list` the workflows and runs; `run` one (it goes on in the " +
			"background and reports here); `status` a run; `stop` a run gracefully — nothing more starts, the running " +
			"task is told; `approve` an external task of a run that is waiting on it.\nA task's `target` is a CHECK (a file that must exist, " +
			"a command that must exit 0), not the work: a target already true is skipped without a turn. A task with no target keeps the agent's answer as its proof.",
		InputSchema: llm.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"action":    map[string]interface{}{"type": "string", "enum": []string{"list", "run", "status", "stop", "approve", "resume"}},
				"name":      map[string]interface{}{"type": "string", "description": "run: the workflow (.memdoor/workflows/<name>) or a path to a directory of task YAML"},
				"partition": map[string]interface{}{"type": "string", "description": "run: a run is fresh by default. Name an earlier run's partition (its status shows it) to resume it — what it finished is skipped; \"today\" for a DAG run once a day."},
				"run_id":    map[string]interface{}{"type": "string", "description": "status, stop, approve, resume: which run"},
				"task":      map[string]interface{}{"type": "string", "description": "approve: the external task"},
			},
			Required: []string{"action"},
		},
		Function: func(input json.RawMessage) (string, error) {
			var in workflowToolInput
			if err := json.Unmarshal(input, &in); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			ops := ar.workflows
			if ops == nil {
				return "", fmt.Errorf("workflows are not available on this gateway")
			}
			switch strings.ToLower(strings.TrimSpace(in.Action)) {
			case "list":
				runs, files := ops.workflowList(workdir)
				return describeWorkflowList(workdir, runs, files), nil
			case "run", "start":
				if strings.TrimSpace(in.Name) == "" {
					return "", fmt.Errorf("name is required: which workflow? (workflow(action:\"list\") shows them)")
				}
				run, err := ops.startWorkflow(workdir, strings.TrimSpace(in.Name), session, "", actor, in.Partition, 0)
				if err != nil {
					return "", err
				}
				snap := run.snapshot()
				return fmt.Sprintf("▶ %s · %d tasks · %s. It reports here on its own — do not wait, sleep or poll; say it is running and end your turn.\n%s",
					snap.Workflow, snap.Total, snap.ID, describeWorkflowRun(snap)), nil
			case "status":
				run, err := ops.workflowStatus(strings.TrimSpace(in.RunID))
				if err != nil {
					return "", err
				}
				return describeWorkflowRun(run.snapshot()), nil
			case "stop", "cancel":
				run, err := ops.stopWorkflow(strings.TrimSpace(in.RunID))
				if err != nil {
					return "", err
				}
				snap := run.snapshot()
				return fmt.Sprintf("■ %s %s — nothing more starts; a running task is told and ends on its own.", snap.ID, snap.State), nil
			case "resume", "continue":
				run, err := ops.resumeWorkflow(strings.TrimSpace(in.RunID), session, workdir, "", actor)
				if err != nil {
					return "", err
				}
				snap := run.snapshot()
				return fmt.Sprintf("▶ %s resumed as %s — what it finished is skipped. It reports here on its own; do not wait.\n%s", snap.Workflow, snap.ID, describeWorkflowRun(snap)), nil
			case "approve":
				if strings.TrimSpace(in.Task) == "" {
					return "", fmt.Errorf("task is required: which external task?")
				}
				run, err := ops.approveWorkflowTask(strings.TrimSpace(in.RunID), strings.TrimSpace(in.Task))
				if err != nil {
					return "", err
				}
				return "✓ approved\n" + describeWorkflowRun(run.snapshot()), nil
			default:
				return "", fmt.Errorf("action must be list, run, status, stop, approve or resume")
			}
		},
	}
}

func describeWorkflowList(dir string, runs []workflowRunJSON, files []string) string {
	var b strings.Builder
	if len(files) == 0 {
		fmt.Fprintf(&b, "No workflows in %s — add .memdoor/workflows/<name>/<group>/<task>.yaml, or pass a directory as name.\n", dir)
	} else {
		fmt.Fprintf(&b, "Workflows in %s: %s\n", dir, strings.Join(files, ", "))
	}
	for _, r := range runs {
		fmt.Fprintf(&b, "%s  %s  %s  %d/%d\n", r.ID, r.Workflow, r.State, r.Done, r.Total)
	}
	return strings.TrimRight(b.String(), "\n")
}

func describeWorkflowRun(r workflowRunJSON) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s · %s · %d/%d done", r.ID, r.State, r.Done, r.Total)
	if r.Error != "" {
		fmt.Fprintf(&b, " — %s", r.Error)
	}
	for _, t := range r.Tasks {
		fmt.Fprintf(&b, "\n  %-8s %s", t.State, t.Name)
		if len(t.Requires) > 0 {
			fmt.Fprintf(&b, "  ← %s", strings.Join(t.Requires, ", "))
		}
		if t.External && t.State != "done" && t.State != "skipped" {
			b.WriteString("  (external)")
		}
		if t.Error != "" && t.State != "done" {
			fmt.Fprintf(&b, "  — %s", oneLine(t.Error, 160))
		}
	}
	return b.String()
}

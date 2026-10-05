package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"memdoor/cmd/tui/ui"
)

// memdoor workflow: a DAG of agent tasks, run on mario (ADR-0019), from the
// shell. The gateway runs it; this is the surface, and the ops the TUI panel
// is handed.

type workflowTaskStatus struct {
	Name     string     `json:"name"`
	Agent    string     `json:"agent"`
	Requires []string   `json:"requires"`
	Target   string     `json:"target"`
	External bool       `json:"external"`
	State    string     `json:"state"`
	Error    string     `json:"error"`
	Attempt  int        `json:"attempt"`
	Took     string     `json:"took"`
	Started  *time.Time `json:"started,omitempty"`
	Receipt  string     `json:"receipt,omitempty"`
}

type workflowRunStatus struct {
	ID        string               `json:"id"`
	Workflow  string               `json:"workflow"`
	Dir       string               `json:"dir"`
	Partition string               `json:"partition"`
	State     string               `json:"state"`
	Started   time.Time            `json:"started"`
	Error     string               `json:"error"`
	Tasks     []workflowTaskStatus `json:"tasks"`
	Done      int                  `json:"done"`
	Total     int                  `json:"total"`
	Review    string               `json:"review,omitempty"`
	Results   []string             `json:"results,omitempty"`
}

type workflowListResponse struct {
	Runs    []workflowRunStatus `json:"runs"`
	Files   []string            `json:"files"`
	Entries []struct {
		Name, Source, Description string
	} `json:"entries"`
}

func workflowPost(action string, body map[string]interface{}, v interface{}) error {
	// The run's workspace decides whose windows see it (gateway
	// BroadcastToWorkspace): a shell-started run names the one this
	// directory resolves to, as the TUI names its own.
	if _, ok := body["workspace"]; !ok {
		if ws, _, found := resolveWorkspaceSlug(); found {
			body["workspace"] = ws
		}
	}
	return NewClient().PostJSON("/api/workflow/"+action, body, v)
}

// workflowDir is the project a workflow command is about, as an absolute
// path: --dir as given ("." included), else here. The gateway refuses a
// relative one, and `--dir .` is how a Makefile says "this repo" (2026-10-04).
func workflowDir(flag string) (string, error) {
	if flag != "" {
		return filepath.Abs(flag)
	}
	return os.Getwd()
}

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Run a DAG of agent tasks from .memdoor/workflows/<name>/",
	Long: `A workflow is a directory, .memdoor/workflows/<name>/ in a project (or in
~/.memdoor/workflows for every project): one YAML file per task, each one
turn of an agent, a model call or a command, with the tasks it requires and a
TARGET that proves it —
a file that must exist, or a command that must exit 0. A task is done when
its target exists, never when the model says so; a re-run skips what exists.

  memdoor workflow                       runs here, and the project's workflows
  memdoor workflow run <name>            start .memdoor/workflows/<name>/ — a fresh run
  memdoor workflow resume <run-id>       continue a failed or stopped run: what it finished is skipped
  memdoor workflow status <run-id>       every task's state
  memdoor workflow stop <run-id>         cancel a run
  memdoor workflow approve <run-id> <task>   complete an external task
  memdoor workflow history <name>        what a workflow did: every run, newest first

In the TUI, /workflow draws the graph as it runs. Every run is kept in the
project's run table (.memdoor/runs.db), so history answers for runs that
ended before this gateway started.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := workflowDir(workflowDirFlag)
		if err != nil {
			return err
		}
		var out workflowListResponse
		if err := workflowPost("list", map[string]interface{}{"dir": dir}, &out); err != nil {
			return err
		}
		if len(out.Files) > 0 {
			fmt.Println("Workflows in " + dir + " (and ~/.memdoor/workflows):")
			desc := map[string]string{}
			src := map[string]string{}
			for _, e := range out.Entries {
				desc[e.Name], src[e.Name] = e.Description, e.Source
			}
			for _, f := range out.Files {
				where := ""
				if src[f] == "library" {
					where = " · library"
				}
				fmt.Printf("  %-20s %s%s\n", f, desc[f], where)
			}
		} else {
			fmt.Println("No workflows in " + dir + " — add .memdoor/workflows/<name>/ (one YAML file per task), or ask the coder for one.")
		}
		if len(out.Runs) > 0 {
			fmt.Println()
			fmt.Printf("%-26s %-16s %-9s %-6s %s\n", "RUN", "WORKFLOW", "STATE", "DONE", "STARTED")
			for _, r := range out.Runs {
				fmt.Printf("%-26s %-16s %-9s %d/%-4d %s\n", r.ID, r.Workflow, r.State, r.Done, r.Total, r.Started.Local().Format("15:04:05"))
			}
		}
		return nil
	},
}

var workflowDirFlag, workflowTimeoutFlag, workflowPartitionFlag string
var workflowHistoryLimit int

var workflowRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Start .memdoor/workflows/<name>/ in this project — a fresh run",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := workflowDir(workflowDirFlag)
		if err != nil {
			return err
		}
		var out workflowRunStatus
		body := map[string]interface{}{"dir": dir, "name": args[0]}
		if workflowTimeoutFlag != "" {
			body["timeout"] = workflowTimeoutFlag
		}
		if workflowPartitionFlag != "" {
			body["partition"] = workflowPartitionFlag
		}
		if err := workflowPost("run", body, &out); err != nil {
			return err
		}
		fmt.Printf("▶ %s started as %s — %d tasks\n", out.Workflow, out.ID, out.Total)
		printWorkflowTasks(out)
		fmt.Println("\n  memdoor workflow status " + out.ID)
		return nil
	},
}

var workflowStatusCmd = &cobra.Command{
	Use:   "status <run-id>",
	Short: "Every task's state in a run",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var out workflowRunStatus
		if err := workflowPost("status", map[string]interface{}{"run_id": args[0]}, &out); err != nil {
			return err
		}
		fmt.Printf("%s · run %s · partition %s · %s · %d/%d done\n", out.Workflow, out.ID, out.Partition, out.State, out.Done, out.Total)
		if out.Error != "" {
			fmt.Println("  " + out.Error)
		}
		printWorkflowTasks(out)
		if len(out.Results) > 0 {
			fmt.Println("\nResults:")
			for _, f := range out.Results {
				fmt.Println("  " + f)
			}
		}
		if out.Review != "" {
			fmt.Println("\nTo review before approving (memdoor workflow diff " + out.ID + " for every line):")
			for _, l := range strings.Split(out.Review, "\n") {
				fmt.Println("  " + l)
			}
		}
		return nil
	},
}

var workflowStopCmd = &cobra.Command{
	Use:   "stop <run-id>",
	Short: "Cancel a run: nothing more starts, the running task is told",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var out workflowRunStatus
		if err := workflowPost("stop", map[string]interface{}{"run_id": args[0]}, &out); err != nil {
			return err
		}
		fmt.Printf("■ %s %s\n", out.ID, out.State)
		return nil
	},
}

var workflowResumeCmd = &cobra.Command{
	Use:   "resume <run-id>",
	Short: "Continue a failed or stopped run: what it finished is skipped",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := workflowDir(workflowDirFlag)
		if err != nil {
			return err
		}
		var out workflowRunStatus
		if err := workflowPost("resume", map[string]interface{}{"run_id": args[0], "dir": dir}, &out); err != nil {
			return err
		}
		fmt.Printf("▶ %s resumed as %s\n", out.Workflow, out.ID)
		printWorkflowTasks(out)
		return nil
	},
}

var workflowApproveCmd = &cobra.Command{
	Use:   "approve <run-id> <task>",
	Short: "Complete an external task: your yes, written where the DAG looks for it",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var out workflowRunStatus
		if err := workflowPost("approve", map[string]interface{}{"run_id": args[0], "task": args[1]}, &out); err != nil {
			return err
		}
		fmt.Printf("✓ %s.%s approved\n", out.Workflow, args[1])
		printWorkflowTasks(out)
		return nil
	},
}

var workflowChangesCmd = &cobra.Command{
	Use:   "changes <run-id> <task> <comment>",
	Short: "Request changes at a gate: the task reruns with your comment, then the steps after it, and the run comes back to the gate",
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		var out workflowRunStatus
		comment := strings.Join(args[2:], " ")
		if err := workflowPost("changes", map[string]interface{}{"run_id": args[0], "task": args[1], "comment": comment}, &out); err != nil {
			return err
		}
		fmt.Printf("↺ %s.%s reruns with your comment; the run comes back to its gate\n", out.Workflow, args[1])
		return nil
	},
}

var workflowDiffCmd = &cobra.Command{
	Use:   "diff <run-id>",
	Short: "The full diff a run made since it started, committed or not: what you approve at its gate",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var out struct {
			Diff string `json:"diff"`
		}
		if err := workflowPost("diff", map[string]interface{}{"run_id": args[0]}, &out); err != nil {
			return err
		}
		if out.Diff == "" {
			fmt.Println("No changes since the run started.")
			return nil
		}
		fmt.Print(out.Diff)
		return nil
	},
}

func printWorkflowTasks(r workflowRunStatus) {
	w := 0
	for _, t := range r.Tasks {
		if len(t.Name) > w {
			w = len(t.Name)
		}
	}
	for _, t := range r.Tasks {
		glyph := map[string]string{"running": "▶", "done": "✓", "skipped": "✓", "failed": "✗", "stopped": "■", "retrying": "↻"}[t.State]
		if glyph == "" {
			glyph = "○"
		}
		line := fmt.Sprintf("  %s %-*s  %s", glyph, w, t.Name, t.State)
		if t.Took != "" {
			line += " " + t.Took
		}
		if len(t.Requires) > 0 {
			line += "  ← " + strings.Join(t.Requires, ", ")
		}
		if t.External && t.State != "done" {
			line += "  (external)"
		}
		fmt.Println(line)
		if t.Receipt != "" {
			// What proved the step, not only that it is done.
			fmt.Println("      " + t.Receipt)
		}
		if t.Error != "" && t.State != "done" {
			fmt.Println("      " + t.Error)
		}
	}
}

func init() {
	workflowCmd.PersistentFlags().StringVar(&workflowDirFlag, "dir", "", "the project directory (default: here)")
	workflowRunCmd.Flags().StringVar(&workflowTimeoutFlag, "timeout", "", "a clock on the whole run, e.g. 90m (default 2h, at most 24h)")
	workflowRunCmd.Flags().StringVar(&workflowPartitionFlag, "partition", "", "resume an earlier run by its partition (status shows it), or \"today\" for a DAG run once a day; a run is fresh by default")
	workflowHistoryCmd.Flags().IntVar(&workflowHistoryLimit, "limit", 0, "history: how many runs back to show (default 20)")
	workflowCmd.AddCommand(workflowRunCmd, workflowStatusCmd, workflowStopCmd, workflowApproveCmd, workflowChangesCmd, workflowDiffCmd, workflowResumeCmd, workflowHistoryCmd)
	rootCmd.AddCommand(workflowCmd)
}

// tuiWorkflowOps is what the TUI panel is handed: the same gateway calls,
// with the conversation named so the run's events land in that window.
func tuiWorkflowOps(dir func() string, workspace, channel string) ui.WorkflowOps {
	conv := func(r workflowRunStatus) ui.WorkflowRun {
		out := ui.WorkflowRun{ID: r.ID, Workflow: r.Workflow, Dir: r.Dir, State: r.State, Started: r.Started, Error: r.Error, Done: r.Done, Total: r.Total, Review: r.Review, Results: r.Results}
		for _, t := range r.Tasks {
			out.Tasks = append(out.Tasks, ui.WorkflowTask{Name: t.Name, Agent: t.Agent, Requires: t.Requires, Target: t.Target,
				External: t.External, State: t.State, Error: t.Error, Attempt: t.Attempt, Took: t.Took, Started: t.Started, Receipt: t.Receipt})
		}
		return out
	}
	return ui.WorkflowOps{
		List: func() ([]ui.WorkflowRun, []ui.WorkflowEntry, error) {
			var out workflowListResponse
			if err := workflowPost("list", map[string]interface{}{"dir": dir()}, &out); err != nil {
				return nil, nil, err
			}
			runs := make([]ui.WorkflowRun, 0, len(out.Runs))
			for _, r := range out.Runs {
				runs = append(runs, conv(r))
			}
			entries := make([]ui.WorkflowEntry, 0, len(out.Files))
			byName := map[string]ui.WorkflowEntry{}
			for _, e := range out.Entries {
				byName[e.Name] = ui.WorkflowEntry{Name: e.Name, Source: e.Source, Description: e.Description}
			}
			for _, f := range out.Files { // the names are the order; an older gateway sends no entries
				e, ok := byName[f]
				if !ok {
					e = ui.WorkflowEntry{Name: f}
				}
				entries = append(entries, e)
			}
			return runs, entries, nil
		},
		Run: func(name, partition string) (ui.WorkflowRun, error) {
			var out workflowRunStatus
			err := workflowPost("run", map[string]interface{}{"dir": dir(), "name": name, "partition": partition, "workspace": workspace, "channel": channel}, &out)
			return conv(out), err
		},
		Status: func(id string) (ui.WorkflowRun, error) {
			var out workflowRunStatus
			err := workflowPost("status", map[string]interface{}{"run_id": id}, &out)
			return conv(out), err
		},
		Stop: func(id string) (ui.WorkflowRun, error) {
			var out workflowRunStatus
			err := workflowPost("stop", map[string]interface{}{"run_id": id}, &out)
			return conv(out), err
		},
		Resume: func(id string) (ui.WorkflowRun, error) {
			var out workflowRunStatus
			err := workflowPost("resume", map[string]interface{}{"run_id": id, "dir": dir(), "workspace": workspace, "channel": channel}, &out)
			return conv(out), err
		},
		Approve: func(id, task string) (ui.WorkflowRun, error) {
			var out workflowRunStatus
			err := workflowPost("approve", map[string]interface{}{"run_id": id, "task": task}, &out)
			return conv(out), err
		},
		Changes: func(id, task, comment string) (ui.WorkflowRun, error) {
			var out workflowRunStatus
			err := workflowPost("changes", map[string]interface{}{"run_id": id, "task": task, "comment": comment}, &out)
			return conv(out), err
		},
		Diff: func(id string) (string, error) {
			var out struct {
				Diff string `json:"diff"`
			}
			err := workflowPost("diff", map[string]interface{}{"run_id": id}, &out)
			return out.Diff, err
		},
	}
}

// Workflow history is the run table (.memdoor/runs.db), not this gateway's
// memory: it answers for runs that ended before the gateway started.
type workflowHistoryResponse struct {
	Runs []struct {
		Workflow  string `json:"workflow"`
		Partition string `json:"partition"`
		State     string `json:"state"`
		Error     string `json:"error"`
		Started   string `json:"started"`
		Ended     string `json:"ended"`
		Tasks     int    `json:"tasks"`
	} `json:"runs"`
}

// workflowHistoryCmd is what a workflow did, from the run table: the runs
// outlive the gateway that ran them (.memdoor/runs.db in the project).
var workflowHistoryCmd = &cobra.Command{
	Use:   "history <name>",
	Short: "What a workflow did before: every run, newest first (survives a restart)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := workflowDir(workflowDirFlag)
		if err != nil {
			return err
		}
		body := map[string]interface{}{"dir": dir, "name": args[0]}
		if workflowHistoryLimit > 0 {
			body["limit"] = workflowHistoryLimit
		}
		var out workflowHistoryResponse
		if err := workflowPost("history", body, &out); err != nil {
			return err
		}
		if len(out.Runs) == 0 {
			fmt.Printf("No runs of %s yet in this project.\n", args[0])
			return nil
		}
		fmt.Printf("Runs of %s (newest first):\n", args[0])
		for _, r := range out.Runs {
			line := fmt.Sprintf("  %s  %-8s  %d task(s)", r.Partition, r.State, r.Tasks)
			if r.Started != "" {
				if t, err := time.Parse(time.RFC3339, r.Started); err == nil {
					line += "  " + t.Local().Format("2006-01-02 15:04")
				}
			}
			if r.Error != "" {
				line += "  — " + oneLineText(r.Error, 80)
			}
			fmt.Println(line)
		}
		return nil
	},
}

// oneLineText folds a multi-line error into one readable line.
func oneLineText(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}

package gateway

import (
	"context"
	"os"
	"strings"
	"time"

	"memdoor/pkg/workflow"
)

// A DRY RUN walks a workflow and runs nothing (mario's tasks.Base.DryRun):
// each task renders what it would run, no command runs, no model is called,
// a gate is not waited at. It is `workflow check --dry-run`, the step after
// the compile: a changed workflow walked end to end before a run pays for a
// step (#333). Its answers are kept under .memdoor/runs/ on a partition of
// its own; its state lives in memory only, so no run table row is written
// and no window sees a run.

// dryRunTimeout bounds a walk that renders text and runs nothing.
const dryRunTimeout = 2 * time.Minute

type workflowDryTask struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Error  string `json:"error,omitempty"`
	Answer string `json:"answer,omitempty"` // what it would have run, rendered
}

type workflowDryRunJSON struct {
	Partition string            `json:"partition"`
	Tasks     []workflowDryTask `json:"tasks"`
}

// dryRunWorkflow loads the workflow as checkWorkflow does, then walks it.
func (s *Server) dryRunWorkflow(dir, name string) (workflowCheckJSON, error) {
	wf, err := s.checkWorkflow(dir, name)
	if err != nil {
		return workflowCheckJSON{}, err
	}
	types := s.workflowTypes(nil)
	for i := range types {
		types[i].DryRun = true
	}
	partition := "dry-run-" + time.Now().Format("2006-01-02T150405")
	res, err := workflow.Run(context.Background(), wf, types, workflow.Options{Workdir: dir, Partition: partition, Timeout: dryRunTimeout})
	if err != nil {
		return workflowCheckJSON{}, err
	}
	out := workflowDryRunJSON{Partition: partition, Tasks: []workflowDryTask{}}
	for _, t := range wf.Tasks {
		tr := res.Tasks[t.Name]
		dt := workflowDryTask{Name: t.Name, State: string(tr.Status), Error: workflow.ErrorText(tr.Error)}
		if !t.External {
			if b, err := os.ReadFile(workflow.OutputPath(dir, partition, t.Full)); err == nil {
				dt.Answer = strings.TrimRight(string(b), "\n")
			}
		}
		out.Tasks = append(out.Tasks, dt)
	}
	check := checkJSON(wf)
	check.DryRun = &out
	return check, nil
}

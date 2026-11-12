package gateway

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sharedctx "memdoor/pkg/shared/context"
)

// fakeWorkflowOps records what the tool asked of the gateway.
type fakeWorkflowOps struct {
	run     *workflowRun
	calls   []string
	stopErr error
}

func (f *fakeWorkflowOps) workflowList(dir string) ([]workflowRunJSON, []string) {
	f.calls = append(f.calls, "list "+dir)
	return []workflowRunJSON{f.run.snapshot()}, []string{"release"}
}
func (f *fakeWorkflowOps) startWorkflow(dir, name, session, _, _, _, _, prompt string, _ time.Duration) (*workflowRun, error) {
	call := "run " + name + " in " + dir + " for " + session
	if prompt != "" {
		call += " with " + prompt
	}
	f.calls = append(f.calls, call)
	return f.run, nil
}
func (f *fakeWorkflowOps) workflowStatus(id string) (*workflowRun, error) {
	f.calls = append(f.calls, "status "+id)
	return f.run, nil
}
func (f *fakeWorkflowOps) stopWorkflow(id string) (*workflowRun, error) {
	f.calls = append(f.calls, "stop "+id)
	if f.stopErr != nil {
		return nil, f.stopErr
	}
	f.run.state = "stopping"
	return f.run, nil
}
func (f *fakeWorkflowOps) resumeWorkflow(id, session, dir, _, _ string) (*workflowRun, error) {
	f.calls = append(f.calls, "resume "+id)
	return f.run, nil
}
func (f *fakeWorkflowOps) approveWorkflowTask(id, task string) (*workflowRun, error) {
	f.calls = append(f.calls, "approve "+id+" "+task)
	return f.run, nil
}

func workflowToolForTest(t *testing.T) (*AgentRuntime, *fakeWorkflowOps, context.Context) {
	t.Helper()
	_, run, _ := workflowServerWithRun(t)
	ops := &fakeWorkflowOps{run: run}
	ar := &AgentRuntime{workflows: ops}
	ctx := context.WithValue(context.Background(), sharedctx.BuddyToolsKey, []string{"bash", "workflow"})
	ctx = context.WithValue(ctx, sharedctx.WorkdirKey, "/proj")
	ctx = context.WithValue(ctx, sharedctx.SessionIDKey, "ws:chan")
	return ar, ops, ctx
}

func callWorkflowTool(t *testing.T, ar *AgentRuntime, ctx context.Context, input string) (string, error) {
	t.Helper()
	def := ar.turnWorkflowTool(ctx)
	if def == nil {
		t.Fatal("the turn's palette carries workflow, so the tool must be there")
	}
	return def.Function(json.RawMessage(input))
}

func TestTheWorkflowToolRunsInTheTurnsProjectAndReportsToItsWindow(t *testing.T) {
	ar, ops, ctx := workflowToolForTest(t)
	out, err := callWorkflowTool(t, ar, ctx, `{"action":"run","name":"release"}`)
	if err != nil || !strings.Contains(out, "r1") || !strings.Contains(out, "do not wait") {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if got := strings.Join(ops.calls, ";"); got != "run release in /proj for ws:chan" {
		t.Fatalf("calls = %q", got)
	}
}

// #107: a turn hands its task to team-solve. The tool's prompt goes to the run
// as it is; team-solve without one is refused, since it would solve its sample.
func TestTheWorkflowToolHandsItsTaskToTeamSolve(t *testing.T) {
	ar, ops, ctx := workflowToolForTest(t)
	if _, err := callWorkflowTool(t, ar, ctx, `{"action":"run","name":"team-solve","prompt":"Make scan.sh skip vendor/."}`); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ops.calls, ";"); got != "run team-solve in /proj for ws:chan with Make scan.sh skip vendor/." {
		t.Fatalf("calls = %q", got)
	}
	if _, err := callWorkflowTool(t, ar, ctx, `{"action":"run","name":"team-solve"}`); err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("team-solve with no prompt: err = %v", err)
	}
	if len(ops.calls) != 1 {
		t.Fatalf("a refused run started anyway: %q", ops.calls)
	}
	def := ar.turnWorkflowTool(ctx)
	if _, ok := def.InputSchema.Properties.(map[string]interface{})["prompt"]; !ok || !strings.Contains(def.Description, "`team-solve` with the task as `prompt`") {
		t.Fatal("the tool must offer prompt and say team-solve takes the task there")
	}
}

// The prompt reaches the run's folder on a real gateway: prompt.md, which
// team-solve's first step reads as the task.
func TestTheWorkflowToolsPromptIsTheRunsPromptFile(t *testing.T) {
	t.Setenv("MEMDOOR_BILLING_TOKEN", "seat-token")
	dir := t.TempDir()
	writeReleaseWorkflow(t, dir)
	s := runServer(t, &Server{})
	ar := &AgentRuntime{workflows: s}
	ctx := context.WithValue(context.Background(), sharedctx.BuddyToolsKey, []string{"workflow"})
	ctx = context.WithValue(ctx, sharedctx.WorkdirKey, dir)
	// Quotes, a $ and a second line reach the file as they were sent.
	if _, err := callWorkflowTool(t, ar, ctx, `{"action":"run","name":"release","prompt":"Ship 1.2: \"notes\" cost $5.\nThen tag it."}`); err != nil {
		t.Fatal(err)
	}
	runs, _ := s.workflowList(dir)
	if len(runs) != 1 {
		t.Fatalf("runs = %+v", runs)
	}
	t.Cleanup(func() { _, _ = s.stopWorkflow(runs[0].ID) })
	got, err := os.ReadFile(filepath.Join(dir, ".memdoor", "runs", "release-"+runs[0].Partition, "prompt.md"))
	if err != nil || string(got) != "Ship 1.2: \"notes\" cost $5.\nThen tag it.\n" {
		t.Fatalf("prompt.md = %q, %v", got, err)
	}

	// A second run while the first goes on is refused, and the refusal tells
	// the agent the run is the person's: a live turn read "stop it or wait for
	// it" as leave to stop it, and did.
	_, err = callWorkflowTool(t, ar, ctx, `{"action":"run","name":"release","prompt":"Ship 1.3."}`)
	if err == nil || !strings.Contains(err.Error(), "already running here as "+runs[0].ID) || !strings.Contains(err.Error(), "stop it only if they ask") {
		t.Fatalf("second run: err = %v", err)
	}
	if now, _ := s.workflowList(dir); len(now) != 1 || now[0].ID != runs[0].ID || (now[0].State != "running" && now[0].State != "waiting") {
		t.Fatalf("the first run must go on, alone: %+v", now)
	}
}

func TestTheWorkflowToolStopsGracefullyAndSaysSo(t *testing.T) {
	ar, _, ctx := workflowToolForTest(t)
	out, err := callWorkflowTool(t, ar, ctx, `{"action":"stop","run_id":"r1"}`)
	if err != nil || !strings.Contains(out, "r1 stopping") || !strings.Contains(out, "running task is told") {
		t.Fatalf("stop: %v\n%s", err, out)
	}
}

func TestTheWorkflowToolStatusAndApproveReadTheGraph(t *testing.T) {
	ar, ops, ctx := workflowToolForTest(t)
	out, err := callWorkflowTool(t, ar, ctx, `{"action":"status","run_id":"r1"}`)
	if err != nil || !strings.Contains(out, "waiting  deploy  ← approve, changelog") || !strings.Contains(out, "waiting  approve  (external)") {
		t.Fatalf("status: %v\n%s", err, out)
	}
	out, err = callWorkflowTool(t, ar, ctx, `{"action":"approve","run_id":"r1","task":"approve"}`)
	if err != nil || !strings.HasPrefix(out, "✓ approved") || ops.calls[len(ops.calls)-1] != "approve r1 approve" {
		t.Fatalf("approve: %v\n%s", err, out)
	}
}

func TestTheWorkflowToolRefusesInWords(t *testing.T) {
	ar, _, ctx := workflowToolForTest(t)
	for _, c := range []struct{ in, want string }{
		{`{"action":"run"}`, "name is required"},
		{`{"action":"approve","run_id":"r1"}`, "task is required"},
		{`{"action":"dance"}`, "action must be"},
	} {
		if _, err := callWorkflowTool(t, ar, ctx, c.in); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: err = %v, want %q", c.in, err, c.want)
		}
	}
}

func TestTheWorkflowToolIsAbsentWithoutThePaletteOrTheServer(t *testing.T) {
	ar, _, ctx := workflowToolForTest(t)
	bare := context.WithValue(context.Background(), sharedctx.BuddyToolsKey, []string{"bash"})
	if ar.turnWorkflowTool(bare) != nil {
		t.Fatal("a palette without workflow gets no workflow tool")
	}
	if (&AgentRuntime{}).turnWorkflowTool(ctx) != nil {
		t.Fatal("a runtime with no server behind it gets no workflow tool")
	}
}

package gateway

import (
	"context"
	"encoding/json"
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
func (f *fakeWorkflowOps) startWorkflow(dir, name, session, _, _, _ string, _ time.Duration) (*workflowRun, error) {
	f.calls = append(f.calls, "run "+name+" in "+dir+" for "+session)
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

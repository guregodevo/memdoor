package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"memdoor/gateway/broadcast"
	"memdoor/pkg/shared"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	mario "github.com/guregodevo/mario/workflow"
	"memdoor/pkg/workflow"
)

// releaseTasks is the DAG at workflows/release/steps/<task>.yaml, mario's layout.
var releaseTasks = map[string]string{
	"tests":     "type: agent\nprompt: run the tests\ntarget:\n  command: \"true\"\n",
	"changelog": "type: agent\nprompt: write the changelog\nrequires:\n  - table_pattern: tests\ntarget:\n  file: CHANGELOG.md\n",
	"deploy":    "type: agent\nagent: runner\nprompt: deploy\nrequires:\n  - table_pattern: approve\n    external: true\n  - table_pattern: changelog\ntarget:\n  command: \"true\"\n",
}

func writeReleaseWorkflow(t *testing.T, dir string) {
	t.Helper()
	steps := filepath.Join(dir, ".memdoor", "workflows", "release", "steps")
	if err := os.MkdirAll(steps, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, yaml := range releaseTasks {
		if err := os.WriteFile(filepath.Join(steps, name+".yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func workflowServerWithRun(t *testing.T) (*Server, *workflowRun, string) {
	t.Helper()
	t.Setenv("MEMDOOR_BILLING_TOKEN", "seat-token")
	dir := t.TempDir()
	writeReleaseWorkflow(t, dir)
	s := &Server{}
	wf, err := workflow.Load(filepath.Join(dir, ".memdoor", "workflows", "release"), workflow.Registry(s.workflowTypes(nil)...))
	if err != nil {
		t.Fatal(err)
	}
	run := &workflowRun{ID: "r1", Workflow: wf.Name, Dir: dir, Partition: "2026-10-02", WF: wf, state: "running", started: time.Now()}
	w := s.workflows()
	w.runs[run.ID] = run
	return s, run, dir
}

func TestWorkflowFilesListsTheProjectsWorkflows(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // the list includes the library under ~/.memdoor/workflows
	dir := t.TempDir()
	if got := workflowFiles(dir); len(got) != 0 {
		t.Fatalf("an empty project has no workflows, got %v", got)
	}
	writeReleaseWorkflow(t, dir)
	os.MkdirAll(filepath.Join(dir, ".memdoor", "workflows", "empty"), 0o755)
	os.WriteFile(filepath.Join(dir, ".memdoor", "workflows", "notes.txt"), []byte("x"), 0o644)
	if got := strings.Join(workflowFiles(dir), ","); got != "release" {
		t.Fatalf("workflowFiles = %q, want release", got)
	}
}

func TestApproveWritesTheExternalTarget(t *testing.T) {
	s, _, dir := workflowServerWithRun(t)
	if _, err := s.approveWorkflowTask("r1", "approve"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(workflow.OutputPath(dir, "2026-10-02", "release.steps.approve"))
	if err != nil || !strings.HasPrefix(string(b), "approved ") {
		t.Fatalf("approval must land where the DAG looks: %v %q", err, b)
	}
	// What the window shows after approval is mario's record of the trigger
	// the approval starts (TestApproveTriggersAWaitingRunAgain); here the
	// proof is the file where the DAG looks for it.
}

func TestApproveRefusesWhatIsNotYoursToApprove(t *testing.T) {
	s, _, _ := workflowServerWithRun(t)
	for _, c := range []struct{ run, task, want string }{
		{"nope", "approve", "no run nope"},
		{"r1", "ship", "has no task ship"},
		{"r1", "changelog", "is not a gate"},
	} {
		_, err := s.approveWorkflowTask(c.run, c.task)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("approve %s.%s: err = %v, want %q", c.run, c.task, err, c.want)
		}
	}
}

func TestSnapshotIsMariosRecord(t *testing.T) {
	_, run, _ := workflowServerWithRun(t)
	run.last = workflow.Result{Tasks: map[string]workflow.TaskResult{
		"tests":     {Status: workflow.StatusRunning, Started: time.Now().Add(-3 * time.Second), Took: 3 * time.Second, Attempt: 1},
		"changelog": {Status: workflow.StatusWaiting},
		"approve":   {Status: workflow.StatusWaiting, External: true},
		"deploy":    {Status: workflow.StatusWaiting},
	}}
	snap := run.snapshot()
	byName := func(s workflowRunJSON, name string) workflowTaskJSON {
		for _, tj := range s.Tasks {
			if tj.Name == name {
				return tj
			}
		}
		t.Fatalf("no task %s in %+v", name, s.Tasks)
		return workflowTaskJSON{}
	}
	if tests := byName(snap, "tests"); snap.Total != 4 || tests.State != "running" || tests.Took != "3s" || tests.Attempt != 1 {
		t.Fatalf("running task not in the snapshot: %+v", tests)
	}
	targets := map[string]string{}
	for _, tj := range snap.Tasks {
		targets[tj.Name] = tj.Target
	}
	if targets["changelog"] != "file CHANGELOG.md" || targets["tests"] != "command: true" || targets["approve"] != "output" {
		t.Fatalf("targets must read in the snapshot: %v", targets)
	}
	run.last = workflow.Result{Tasks: map[string]workflow.TaskResult{
		"tests": {Status: workflow.StatusSkipped}, "changelog": {Status: workflow.StatusDone, Took: time.Second},
		"approve": {Status: workflow.StatusWaiting, External: true}, "deploy": {Status: workflow.StatusFailed, Error: "exit 1"},
	}}
	snap = run.snapshot()
	states := map[string]string{}
	for _, tj := range snap.Tasks {
		states[tj.Name] = tj.State
	}
	if states["tests"] != "skipped" || states["changelog"] != "done" || states["approve"] != "waiting" || states["deploy"] != "failed" {
		t.Fatalf("record not reflected: %v", states)
	}
	if deploy := byName(snap, "deploy"); snap.Done != 2 || deploy.Error != "exit 1" {
		t.Fatalf("done = %d (want 2), deploy error = %q", snap.Done, deploy.Error)
	}
}

// A run that ended waiting at the gate is triggered again by the approval;
// the gate itself is never run, and the broadcast carries the fresh state.
func TestApproveTriggersAWaitingRunAgain(t *testing.T) {
	s, run, dir := workflowServerWithRun(t)
	run.state = "waiting"
	os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte("v0.1.0\n"), 0o644)
	if _, err := s.approveWorkflowTask("r1", "approve"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		snap := run.snapshot()
		if snap.State != "running" && snap.State != "waiting" {
			// Every target exists (deploy's command is "true"), so the
			// re-triggered run ends done without a single turn — no agent
			// serves this test server, so a turn would have failed.
			if snap.State != "done" {
				t.Fatalf("re-triggered run ended %q, want done: %+v", snap.State, snap)
			}
			states := map[string]string{}
			for _, tj := range snap.Tasks {
				states[tj.Name] = tj.State
			}
			if (states["approve"] != "done" && states["approve"] != "skipped") || states["changelog"] != "skipped" || states["deploy"] != "skipped" {
				t.Fatalf("after approval the gate is done and what exists is skipped: %v", states)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("re-triggered run never ended")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestStopAWaitingRunEndsIt(t *testing.T) {
	s, run, _ := workflowServerWithRun(t)
	run.state = "waiting"
	if _, err := s.stopWorkflow("r1"); err != nil {
		t.Fatal(err)
	}
	if run.snapshot().State != "stopped" {
		t.Fatalf("a waiting run stops at once, got %q", run.snapshot().State)
	}
	if _, err := s.stopWorkflow("r1"); err == nil {
		t.Fatal("stopping a stopped run must say it is already stopped")
	}
}

func TestStopUnknownRunIsAnError(t *testing.T) {
	s, _, _ := workflowServerWithRun(t)
	if _, err := s.stopWorkflow("ghost"); err == nil {
		t.Fatal("stopping a run that does not exist must say so")
	}
}

func TestStartWorkflowRefusesARelativeDirAndAMissingFile(t *testing.T) {
	s := &Server{}
	if _, err := s.startWorkflow("relative", "x", "", "", "", "", 0); err == nil {
		t.Fatal("a relative dir must be refused")
	}
	dir := t.TempDir()
	if _, err := s.startWorkflow(dir, "missing", "", "", "", "", 0); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("a missing workflow must be named: %v", err)
	}
}

func TestRunIDsAreUniqueWithinASecond(t *testing.T) {
	w := &workflowRuns{runs: map[string]*workflowRun{}}
	a := w.newID("adhoc")
	w.runs[a] = &workflowRun{}
	b := w.newID("adhoc")
	w.runs[b] = &workflowRun{}
	c := w.newID("slow")
	if a == b || !strings.HasPrefix(a, "adhoc-") || !strings.HasPrefix(c, "slow-") {
		t.Fatalf("ids = %q %q %q", a, b, c)
	}
}

func TestTheRelayMapsATaskSessionBackToItsTask(t *testing.T) {
	s, run, _ := workflowServerWithRun(t)
	if got := run.taskOfSession(run.taskSession("changelog")); got != "changelog" {
		t.Fatalf("round trip = %q", got)
	}
	if run.taskOfSession("ws:chan") != "" {
		t.Fatal("another session is nobody's task")
	}
	if off := s.listen(run); off == nil {
		t.Fatal("a server with no window to report to still answers a stopper")
	}
}

func TestARunIsFreshUnlessAPartitionIsNamed(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	if f := workflowPartition(""); !strings.HasPrefix(f, today+"T") {
		t.Fatalf("default = %q, want a fresh partition of this moment", f)
	}
	if workflowPartition("today") != today {
		t.Fatalf("today = %q", workflowPartition("today"))
	}
	if workflowPartition("2026-10-02T130438") != "2026-10-02T130438" {
		t.Fatal("an earlier run's partition is kept as said: it resumes that run")
	}
}

func TestResumeIsANewRunOnTheSamePartition(t *testing.T) {
	s, run, _ := workflowServerWithRun(t)
	run.state = "running"
	if _, err := s.resumeWorkflow("r1", "", "", "", ""); err == nil || !strings.Contains(err.Error(), "still running") {
		t.Fatalf("a running run is not resumed: %v", err)
	}
	run.state = "waiting"
	if _, err := s.resumeWorkflow("r1", "", "", "", ""); err == nil || !strings.Contains(err.Error(), "approve") {
		t.Fatalf("a waiting run is approved, not resumed: %v", err)
	}
	run.state = "failed"
	again, err := s.resumeWorkflow("r1", "win", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID == "r1" || again.Partition != "2026-10-02" || again.Session != "win" {
		t.Fatalf("resume = new run %q on partition %q for %q", again.ID, again.Partition, again.Session)
	}
	s.stopWorkflow(again.ID)
}

// The HTTP surface the TUI, the CLI and the tool share, action by action.
func TestHandleWorkflowHTTP(t *testing.T) {
	s, run, dir := workflowServerWithRun(t)
	call := func(action string, body map[string]interface{}) (int, map[string]interface{}) {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/workflow/"+action, bytes.NewReader(b))
		rec := httptest.NewRecorder()
		s.handleWorkflow(rec, req)
		var out map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	if code, out := call("status", map[string]interface{}{"run_id": "r1"}); code != 200 || out["id"] != "r1" || out["total"] != float64(4) {
		t.Fatalf("status: %d %v", code, out)
	}
	if code, _ := call("status", map[string]interface{}{"run_id": "ghost"}); code != 404 {
		t.Fatalf("an unknown run is 404, got %d", code)
	}
	if code, out := call("list", map[string]interface{}{"dir": dir}); code != 200 || len(out["runs"].([]interface{})) != 1 || out["files"].([]interface{})[0] != "release" {
		t.Fatalf("list: %d %v", code, out)
	}
	if code, _ := call("run", map[string]interface{}{"dir": "relative", "name": "x"}); code != 400 {
		t.Fatalf("a relative dir is refused with 400, got %d", code)
	}
	if code, _ := call("run", map[string]interface{}{"dir": dir, "name": "release", "timeout": "not-a-duration"}); code != 400 {
		t.Fatalf("a bad timeout is refused with 400, got %d", code)
	}
	if code, _ := call("resume", map[string]interface{}{"run_id": "r1"}); code != 400 {
		t.Fatalf("resuming a running run is refused with 400, got %d", code)
	}
	if code, _ := call("dance", nil); code != 404 {
		t.Fatalf("an unknown action is 404, got %d", code)
	}
	run.state = "waiting"
	if code, out := call("stop", map[string]interface{}{"run_id": "r1"}); code != 200 || out["state"] != "stopped" {
		t.Fatalf("stop a waiting run: %d %v", code, out)
	}
}

func TestTheToolDescribesAListAndARun(t *testing.T) {
	_, run, dir := workflowServerWithRun(t)
	out := describeWorkflowList(dir, []workflowRunJSON{run.snapshot()}, []string{"release", "nightly"})
	if !strings.Contains(out, "release, nightly") || !strings.Contains(out, "r1  release  running  0/4") {
		t.Fatalf("list:\n%s", out)
	}
	out = describeWorkflowList(dir, nil, nil)
	if !strings.Contains(out, "No workflows in") {
		t.Fatalf("empty list:\n%s", out)
	}
	run.last = workflow.Result{Tasks: map[string]workflow.TaskResult{"deploy": {Status: workflow.StatusFailed, Error: "exit 1\nmore"}, "approve": {Status: workflow.StatusWaiting, External: true}}}
	out = describeWorkflowRun(run.snapshot())
	if !strings.Contains(out, "failed   deploy  ← approve, changelog  — exit 1 more") || !strings.Contains(out, "waiting  approve  (external)") {
		t.Fatalf("run:\n%s", out)
	}
}

// Greg, 2026-10-02: "workflow is for Pro only". Without a seat every way of
// starting a run is refused in words that say where Pro is; with one it runs.
// Local runs are free: no seat, the run starts (2026-10-04).
func TestLocalWorkflowsNeedNoSeat(t *testing.T) {
	s, _, dir := workflowServerWithRun(t)
	t.Setenv("MEMDOOR_BILLING_TOKEN", "")
	t.Setenv("HOME", t.TempDir())
	if _, err := s.startWorkflow(dir, "release", "", "", "", "", 0); err != nil && strings.Contains(err.Error(), "Pro") {
		t.Fatalf("a local run must not ask for a seat: %v", err)
	}
}

// THE POINT OF THE RUN TABLE: a run recorded by one gateway is readable by
// the next one — a restart does not forget what a workflow did. The test
// writes a run into a project's table with one Server, throws that Server
// away, and reads the run back through a brand-new one, exactly as a
// restarted gateway would.
func TestWorkflowHistorySurvivesAGateway(t *testing.T) {
	dir := t.TempDir()
	writeReleaseWorkflow(t, dir)

	first := &Server{}
	repo, _ := first.runRepository(dir)
	if repo == nil {
		t.Fatal("a project must get a run table")
	}
	// Two executions of one run: the DAG's tasks, sharing the partition.
	for _, task := range []string{"tests", "changelog"} {
		execution := mario.WorkflowExecution{
			ExecutionId: task + "-2026-10-03T090000",
			WorkflowInstanceId: mario.WorkflowInstanceId{
				WorkflowId: mario.WorkflowId{DName: "release", DVersion: "release@2026-10-03T090000", DComponent: "memdoor"},
				Partition:  "2026-10-03T090000",
			},
			StartDate: time.Now(),
			Status:    mario.Done,
		}
		if err := repo.Upsert(execution); err != nil {
			t.Fatalf("Upsert: %v", err)
		}
	}
	// The table is on disk in the project, where a restart would find it.
	if _, err := os.Stat(filepath.Join(workflow.ProjectRoot(dir), ".memdoor", "runs.db")); err != nil {
		t.Fatalf("the run table is not in the project: %v", err)
	}

	// A different gateway — this process never ran anything.
	second := &Server{}
	history, _ := second.workflowHistory(dir, "release", 10)
	if len(history) != 1 {
		t.Fatalf("a restarted gateway sees %d runs, want 1: %+v", len(history), history)
	}
	if history[0].State != "done" {
		t.Fatalf("the run reads as %q, want done", history[0].State)
	}
	if history[0].Tasks != 2 {
		t.Fatalf("the run has %d tasks, want 2", history[0].Tasks)
	}
	if history[0].Partition != "2026-10-03T090000" {
		t.Fatalf("the run's partition is %q", history[0].Partition)
	}
}

// History must list a run the gateway actually ran — the hand-written rows
// of the test above have the workflow's name where a real run writes the
// task's, which is how an empty history shipped (2026-10-03). A real run
// of the release workflow, then history on a brand-new Server.
func TestWorkflowHistoryListsARealRun(t *testing.T) {
	t.Setenv("MEMDOOR_BILLING_TOKEN", "seat-token")
	dir := t.TempDir()
	writeReleaseWorkflow(t, dir)
	first := &Server{}
	run, err := first.startWorkflow(dir, "release", "", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		run.mu.Lock()
		state := run.state
		run.mu.Unlock()
		if state == "waiting" || state == "done" || state == "failed" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	second := &Server{}
	runs, _ := second.workflowHistory(dir, "release", 0)
	if len(runs) != 1 || runs[0].Workflow != "release" || runs[0].Partition != run.Partition || runs[0].Tasks == 0 {
		t.Fatalf("a real run, read by a new gateway: %+v", runs)
	}
	if none, _ := second.workflowHistory(dir, "nothing-by-this-name", 0); len(none) != 0 {
		t.Fatal("another name lists nothing")
	}
}

// RESUME AFTER A RESTART (2026-10-04: a run resumed by id after `make up`
// answered "no run …"). A real run on one Server; a brand-new Server — the
// restarted gateway, its memory empty — resumes it by id from the project's
// run table, on the same partition. An id the table never saw stays refused.
func TestResumeAfterARestartFindsTheRunInTheTable(t *testing.T) {
	t.Setenv("MEMDOOR_BILLING_TOKEN", "seat-token")
	dir := t.TempDir()
	writeReleaseWorkflow(t, dir)
	first := &Server{}
	run, err := first.startWorkflow(dir, "release", "", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		run.mu.Lock()
		state := run.state
		run.mu.Unlock()
		if state == "waiting" || state == "done" || state == "failed" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	first.stopWorkflow(run.ID)

	restarted := &Server{}
	again, err := restarted.resumeWorkflow(run.ID, "", dir, "", "")
	if err != nil {
		t.Fatalf("resume after a restart: %v", err)
	}
	// A new run (the test runs inside one second, so its id may repeat the
	// stamp) on the old partition: what is done is skipped.
	if again == run || again.Partition != run.Partition {
		t.Fatalf("resumed as %q on %q, want a new run on %q", again.ID, again.Partition, run.Partition)
	}
	restarted.stopWorkflow(again.ID)

	// A RERUN's id (a later trigger of the same partition) is found by the
	// span its own movement left in the table.
	rerun, err := (&Server{}).startWorkflow(dir, "release", "", "", "", run.Partition, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		rerun.mu.Lock()
		st := rerun.state
		rerun.mu.Unlock()
		if st == "waiting" || st == "done" || st == "failed" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, partition, ok := (&Server{}).runFromTable(dir, rerun.ID); !ok || partition != run.Partition {
		t.Fatalf("a rerun's id maps to its partition: %q %v", partition, ok)
	}

	// NEVER A GUESS (2026-10-04: a jeff run resumed building-effective-agents).
	// An id whose stamp lies in no partition's span is refused with the
	// partitions named — not given the latest one started before it.
	stray := "release-" + time.Now().Add(1*time.Hour).Format("20060102-150405")
	if _, err := (&Server{}).resumeWorkflow(stray, "", dir, "", ""); err == nil || !strings.Contains(err.Error(), "--partition") || !strings.Contains(err.Error(), run.Partition) {
		t.Fatalf("an undecidable id is refused with the partitions named: %v", err)
	}
	if _, err := (&Server{}).resumeWorkflow("release-20200101-000000", "", dir, "", ""); err == nil || !strings.Contains(err.Error(), "run table") {
		t.Fatalf("an id the table never saw is refused, naming where it looked: %v", err)
	}
	if _, err := (&Server{}).resumeWorkflow(run.ID, "", "", "", ""); err == nil {
		t.Fatal("no project named, nothing to look in")
	}
}

// A FINISHED RUN SHOWS WHAT IT MADE (2026-10-04: "we dont see the result"):
// the done line lists the file targets that exist, in task order, and opens
// the final task's file — capped, with what is left counted.
func TestADoneRunShowsItsResults(t *testing.T) {
	dir := t.TempDir()
	steps := filepath.Join(dir, ".memdoor", "workflows", "w", "steps")
	os.MkdirAll(steps, 0o755)
	os.WriteFile(filepath.Join(steps, "a.yaml"), []byte("type: command\ncommand: echo a\ntarget:\n  file: reports/a.json\n"), 0o644)
	os.WriteFile(filepath.Join(steps, "b.yaml"), []byte("type: command\ncommand: echo b\nrequires:\n  - table_pattern: a\ntarget:\n  file: FINAL-{{.partition}}.md\n"), 0o644)
	os.WriteFile(filepath.Join(steps, "c.yaml"), []byte("type: command\ncommand: echo c\ntarget:\n  file: never.md\n"), 0o644)
	s := &Server{}
	wf, err := workflow.Load(filepath.Join(dir, ".memdoor", "workflows", "w"), workflow.Registry(s.workflowTypes(nil)...))
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, "reports"), 0o755)
	os.WriteFile(filepath.Join(dir, "reports", "a.json"), []byte("{}"), 0o644)
	var final strings.Builder
	for i := 1; i <= 25; i++ {
		fmt.Fprintf(&final, "line %d\n", i)
	}
	os.WriteFile(filepath.Join(dir, "FINAL-p1.md"), []byte(final.String()), 0o644)

	run := &workflowRun{ID: "w-1", Workflow: "w", Dir: dir, Partition: "p1", WF: wf}
	note := doneNote(run)
	if !strings.Contains(note, "results: reports/a.json · FINAL-p1.md") || strings.Contains(note, "never.md") {
		t.Fatalf("the files the run made, and only those: %q", note)
	}
	if !strings.Contains(note, "FINAL-p1.md:\n    line 1\n    line 2") || strings.Contains(note, "line 21") || !strings.Contains(note, "5 more lines") {
		t.Fatalf("the final file opens, capped: %q", note)
	}
}

// A RUN REACHES ITS WORKSPACE'S WINDOWS, AND ONLY THOSE (review, 2026-10-04:
// "every window" leaked one user's runs to another's on a shared gateway). A
// reopened window of the same workspace — a new channel — still sees the run;
// a run with neither a workspace nor a window reaches nobody.
type namedScreen struct {
	id     string
	mu     sync.Mutex
	frames int
}

func (n *namedScreen) GetID() string { return n.id }
func (n *namedScreen) Send([]byte) error {
	n.mu.Lock()
	n.frames++
	n.mu.Unlock()
	return nil
}
func (n *namedScreen) count() int { n.mu.Lock(); defer n.mu.Unlock(); return n.frames }

func TestARunReachesItsWorkspacesWindowsOnly(t *testing.T) {
	sm := broadcast.NewSubscriptionManager(false)
	reopened, colleague := &namedScreen{id: "mine-2"}, &namedScreen{id: "theirs"}
	for sub, session := range map[*namedScreen]string{
		reopened:  shared.NewChannelSessionID("ws-a", "new-channel"),
		colleague: shared.NewChannelSessionID("ws-b", "their-channel"),
	} {
		sm.AddSubscriber(sub)
		sm.SubscribeToSession(sub.GetID(), session)
	}
	s := &Server{broadcaster: sm}
	gone := &workflowRun{ID: "w-1", Dir: "/p", Session: shared.NewChannelSessionID("ws-a", "closed-channel"), Workspace: runWorkspace("", shared.NewChannelSessionID("ws-a", "closed-channel"))}
	s.sendRunEvent(gone, map[string]interface{}{"run": gone.ID, "state": "note", "text": "■ w done"})
	if reopened.count() != 1 || colleague.count() != 0 {
		t.Fatalf("the workspace's reopened window sees it, the other workspace does not: %d %d", reopened.count(), colleague.count())
	}
	nobody := &workflowRun{ID: "w-2", Dir: "/p"}
	s.sendRunEvent(nobody, map[string]interface{}{"run": nobody.ID, "state": "note", "text": "x"})
	if reopened.count() != 1 || colleague.count() != 0 {
		t.Fatal("a run with no workspace and no window reaches nobody")
	}
}

// A STOPPED RUN'S KILLED STEP WAS STOPPED, NOT FAILED (2026-10-04: a stopped
// run read "✗ long failed — signal: killed"). A failure in a run that was not
// stopped stays a failure.
func TestAStoppedRunsKilledStepReadsStopped(t *testing.T) {
	_, run, _ := workflowServerWithRun(t)
	run.state = "stopped"
	run.last = workflow.Result{Tasks: map[string]workflow.TaskResult{"tests": {Status: workflow.StatusFailed, Error: "signal: killed"}}}
	for _, tj := range run.snapshot().Tasks {
		if tj.Name == "tests" && (tj.State != "stopped" || tj.Error != "") {
			t.Fatalf("a stopped run's killed step: %q %q", tj.State, tj.Error)
		}
	}
	run.state = "failed"
	for _, tj := range run.snapshot().Tasks {
		if tj.Name == "tests" && tj.State != "failed" {
			t.Fatalf("a real failure stays failed: %q", tj.State)
		}
	}
}

// A project's panel lists its own runs, not another project's (dogfood
// 2026-10-04: the campaign window listed a scratch selftest's runs).
func TestTheRunListIsThisProjectsOnly(t *testing.T) {
	s := &Server{}
	here, there := t.TempDir(), t.TempDir()
	w := s.workflows()
	w.runs["a"] = &workflowRun{ID: "a", Workflow: "w", Dir: here, started: time.Now()}
	w.runs["b"] = &workflowRun{ID: "b", Workflow: "w", Dir: there, started: time.Now()}
	runs, _ := s.workflowList(here)
	if len(runs) != 1 || runs[0].ID != "a" {
		t.Fatalf("this project's runs only: %+v", runs)
	}
}

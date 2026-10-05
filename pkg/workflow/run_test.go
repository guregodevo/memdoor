package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/guregodevo/mario/tasks"
)

// writeWorkflow lays out <dir>/<name>/steps/<task>.yaml the way mario reads it.
func writeWorkflow(t *testing.T, root, name string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(root, name, "steps")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for task, yaml := range files {
		if err := os.WriteFile(filepath.Join(dir, task+".yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(root, name)
}

type fakeRunner struct {
	mu       sync.Mutex
	order    []string
	dirs     []string
	failures map[string]int // task -> failures still to produce
	hang     bool
	started  chan struct{}
	once     sync.Once
}

func (r *fakeRunner) Run(ctx context.Context, run TaskRun) (string, error) {
	if r.hang {
		if r.started != nil {
			r.once.Do(func() { close(r.started) })
		}
		<-ctx.Done()
		return "", ctx.Err()
	}
	r.mu.Lock()
	r.order = append(r.order, run.Task.Name)
	r.dirs = append(r.dirs, run.Workdir)
	left := r.failures[run.Task.Name]
	if left > 0 {
		r.failures[run.Task.Name] = left - 1
	}
	r.mu.Unlock()
	if left > 0 {
		return "", fmt.Errorf("%s: simulated failure", run.Task.Name)
	}
	if strings.HasPrefix(run.Task.Target, "file ") {
		return "", os.WriteFile(filepath.Join(run.Workdir, strings.TrimPrefix(run.Task.Target, "file ")), []byte("done\n"), 0o644)
	}
	return "answer to: " + run.Prompt, nil
}

func (r *fakeRunner) workdir() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.dirs) == 0 {
		return ""
	}
	return r.dirs[len(r.dirs)-1]
}

func (r *fakeRunner) ran() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.order...)
}

type echoModel struct{}

func (echoModel) Complete(_ context.Context, model, prompt string) (string, error) {
	return model + " says: " + prompt, nil
}

// types is what a test gateway offers: the agent turn on a fake runner,
// mario's command, mario's llm on an echo model.
func types(r *fakeRunner) []tasks.Base {
	return []tasks.Base{Agent(r), tasks.Command(), tasks.LLM(echoModel{})}
}

func load(t *testing.T, dir string, r *fakeRunner) Workflow {
	t.Helper()
	wf, err := Load(dir, Registry(types(r)...))
	if err != nil {
		t.Fatal(err)
	}
	return wf
}

type recorder struct {
	mu  sync.Mutex
	seq []string
}

func (r *recorder) add(s string)                { r.mu.Lock(); r.seq = append(r.seq, s); r.mu.Unlock() }
func (r *recorder) TaskStarted(wf, task string) { r.add("started " + task) }
func (r *recorder) TaskDone(wf, task string)    { r.add("done " + task) }
func (r *recorder) TaskSkipped(wf, task string) { r.add("skipped " + task) }
func (r *recorder) TaskFailed(wf, task string, err error, retry bool) {
	r.add(fmt.Sprintf("failed %s retry=%v", task, retry))
}
func (r *recorder) joined() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.seq, " | ")
}

const release = `type: agent
prompt: write the changelog for {{.version}}
args:
  version: v0.1.0
requires:
  - table_pattern: tests
`

func TestLoadReadsMariosLayout(t *testing.T) {
	dir := writeWorkflow(t, t.TempDir(), "release", map[string]string{
		"tests":     "type: agent\nagent: verifier\nprompt: run the tests\ntimeout: 20m\ntarget:\n  command: go test ./...\n",
		"changelog": release,
		"ship":      "type: agent\nprompt: ship it\nrequires:\n  - table_pattern: changelog\n  - table_pattern: approve\n    external: true\n",
	})
	wf := load(t, dir, &fakeRunner{})
	if wf.Name != "release" || len(wf.Defs) != 3 || len(wf.Tasks) != 4 {
		t.Fatalf("name=%q defs=%d tasks=%d", wf.Name, len(wf.Defs), len(wf.Tasks))
	}
	tests, _ := wf.Task("tests")
	if tests.Type != "agent" || tests.Agent != "verifier" || tests.Timeout != 20*time.Minute || tests.Target != "command: go test ./..." || tests.Full != "release.steps.tests" {
		t.Fatalf("tests = %+v", tests)
	}
	changelog, _ := wf.Task("changelog")
	if changelog.Agent != defaultAgent || strings.Join(changelog.Requires, ",") != "tests" || changelog.Target != "output" {
		t.Fatalf("changelog = %+v", changelog)
	}
	approve, ok := wf.Task("approve")
	if !ok || !approve.External || approve.Full != "release.steps.approve" {
		t.Fatalf("the external a task requires is a task of the DAG: %+v %v", approve, ok)
	}
	if got := strings.Join(wf.Sinks(), ","); got != "release.steps.ship" {
		t.Fatalf("sinks = %q", got)
	}
}

func TestLoadRefusesWhatTheSchemasRefuse(t *testing.T) {
	for _, c := range []struct{ name, yaml, want string }{
		{"unknown field", "type: agent\nprompt: x\ncolour: red\n", "colour"},
		{"no prompt", "type: agent\n", "prompt"},
		{"unknown type", "type: browser\nprompt: x\n", "must be one of"},
		{"command without a command", "type: command\nprompt: x\n", "command"},
		{"missing require", "type: agent\nprompt: x\nrequires:\n  - table_pattern: ghost\n", "ghost"},
	} {
		dir := writeWorkflow(t, t.TempDir(), "w", map[string]string{"a": c.yaml})
		_, err := Load(dir, Registry(types(&fakeRunner{})...))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: err = %v, want it to mention %q", c.name, err, c.want)
		}
	}
	if _, err := Load(filepath.Join(t.TempDir(), "nope"), Registry(types(&fakeRunner{})...)); err == nil {
		t.Fatal("a missing directory must be refused")
	}
}

// An external requirement whose task has a file of its own would be RUN,
// bypassing the gate: mario refuses it when the files are read.
func TestLoadRefusesAnExternalThatIsDefined(t *testing.T) {
	dir := writeWorkflow(t, t.TempDir(), "w", map[string]string{
		"approve": "type: agent\nprompt: wait for a person\n",
		"ship":    "type: agent\nprompt: ship\nrequires:\n  - table_pattern: approve\n    external: true\n",
	})
	_, err := Load(dir, Registry(types(&fakeRunner{})...))
	if err == nil || !strings.Contains(err.Error(), "approve") || !strings.Contains(err.Error(), "external") {
		t.Fatalf("err = %v", err)
	}
}

// A DAG of three types: a command's stdout, an llm's answer over it, an
// agent's turn after both — each built by the factory of its type, each
// output kept where the next can read it.
func TestADAGMixesTypesByTheYAMLsType(t *testing.T) {
	root := t.TempDir()
	dir := writeWorkflow(t, root, "mix", map[string]string{
		"log":     "type: command\ncommand: printf 'c1\\nc2\\n'\n",
		"summary": "type: llm\nmodel: fast\nprompt: summarize the log for {{.partition}}\nrequires:\n  - table_pattern: log\n",
		"notes":   "type: agent\nprompt: write the notes\nrequires:\n  - table_pattern: summary\ntarget:\n  file: NOTES.md\n",
	})
	r := &fakeRunner{}
	wf := load(t, dir, r)
	rec := &recorder{}
	res, err := Run(context.Background(), wf, types(r), Options{Workdir: root, Partition: "2026-10-02", Events: rec, Retry: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if res.Done() != 3 || res.Failed() {
		t.Fatalf("\n%s", res.Summary())
	}
	if got := rec.joined(); got != "started log | done log | started summary | done summary | started notes | done notes" {
		t.Fatalf("events in order = %q", got)
	}
	if b, _ := os.ReadFile(OutputPath(root, "2026-10-02", "mix.steps.log")); string(b) != "c1\nc2\n" {
		t.Fatalf("command output = %q", b)
	}
	if b, _ := os.ReadFile(OutputPath(root, "2026-10-02", "mix.steps.summary")); string(b) != "fast says: summarize the log for 2026-10-02" {
		t.Fatalf("llm output = %q", b)
	}
	if got := strings.Join(r.ran(), ","); got != "notes" {
		t.Fatalf("the agent ran only its own task: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "NOTES.md")); err != nil {
		t.Fatal("the agent task's target was made")
	}
}

func TestRunDoesTheChainAndSkipsOnARerun(t *testing.T) {
	root := t.TempDir()
	dir := writeWorkflow(t, root, "release", map[string]string{
		"tests":     "type: agent\nprompt: run the tests\ntarget:\n  file: tested\n",
		"changelog": release,
	})
	r := &fakeRunner{}
	wf := load(t, dir, r)
	opts := Options{Workdir: root, Partition: "2026-10-02", Retry: 20 * time.Millisecond}
	res, err := Run(context.Background(), wf, types(r), opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.ran(), ","); got != "tests,changelog" || res.Tasks["changelog"].Status != StatusDone {
		t.Fatalf("ran %q\n%s", got, res.Summary())
	}
	if b, _ := os.ReadFile(OutputPath(root, "2026-10-02", "release.steps.changelog")); string(b) != "answer to: write the changelog for v0.1.0" {
		t.Fatalf("the answer is the proof, rendered from args: %q", b)
	}
	res, _ = Run(context.Background(), wf, types(r), opts)
	if len(r.ran()) != 2 || res.Tasks["changelog"].Status != StatusSkipped || res.Tasks["tests"].Status != StatusSkipped {
		t.Fatalf("a re-run skips what exists:\n%s\nran %v", res.Summary(), r.ran())
	}
}

func TestRunRetriesThenFails(t *testing.T) {
	root := t.TempDir()
	dir := writeWorkflow(t, root, "w", map[string]string{
		"flaky": "type: agent\nprompt: do x\nmax_retries: 1\n",
		"after": "type: agent\nprompt: do y\nrequires:\n  - table_pattern: flaky\n",
	})
	r := &fakeRunner{failures: map[string]int{"flaky": 2}}
	wf := load(t, dir, r)
	res, err := Run(context.Background(), wf, types(r), Options{Workdir: root, Retry: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if res.Tasks["flaky"].Status != StatusFailed || !res.Failed() || res.Tasks["after"].Status != StatusWaiting {
		t.Fatalf("\n%s", res.Summary())
	}
	if n := len(r.ran()); n != 2 {
		t.Fatalf("max_retries: 1 means two attempts, got %d", n)
	}
}

// An external task is made outside the workflow: the run builds everything
// else, ends WAITING at the missing target, and a second trigger after the
// target exists finishes the rest, skipping what is already there.
func TestExternalTargetBlocksTheRunUntilTriggeredAgain(t *testing.T) {
	root := t.TempDir()
	dir := writeWorkflow(t, root, "release", map[string]string{
		"tests":     "type: agent\nprompt: run the tests\n",
		"changelog": release,
		"ship":      "type: agent\nprompt: ship it\nrequires:\n  - table_pattern: changelog\n  - table_pattern: approve\n    external: true\n",
	})
	r := &fakeRunner{}
	wf := load(t, dir, r)
	rec := &recorder{}
	opts := Options{Workdir: root, Partition: "2026-10-02", Retry: 20 * time.Millisecond, Events: rec}
	res, err := Run(context.Background(), wf, types(r), opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(res.Blocked(), ","); got != "approve" {
		t.Fatalf("blocked = %q, want approve\n%s", got, res.Summary())
	}
	if res.Tasks["tests"].Status != StatusDone || res.Tasks["changelog"].Status != StatusDone || res.Tasks["ship"].Status != StatusWaiting || res.Failed() {
		t.Fatalf("first trigger builds what it can and stops at the gate:\n%s", res.Summary())
	}
	p := OutputPath(root, "2026-10-02", "release.steps.approve")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("approved\n"), 0o644)
	res, err = Run(context.Background(), wf, types(r), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Blocked()) != 0 || res.Tasks["ship"].Status != StatusDone || res.Tasks["approve"].Status != StatusSkipped || res.Tasks["changelog"].Status != StatusSkipped {
		t.Fatalf("second trigger finishes past the gate:\n%s", res.Summary())
	}
	if got := strings.Join(r.ran(), ","); got != "tests,changelog,ship" {
		t.Fatalf("ran %q: existing targets skipped, the gate never run", got)
	}
	if !strings.Contains(rec.joined(), "skipped changelog") {
		t.Fatalf("a target found there is announced as skipped: %v", rec.joined())
	}
}

// A requirement on another workflow of the same project (project_id) is
// external here, and its proof is THAT task's target — a file, a command —
// not the output convention: the sibling definition is read for it.
func TestCrossWorkflowRequirementUsesTheSiblingsTarget(t *testing.T) {
	root := t.TempDir()
	writeWorkflow(t, root, "release", map[string]string{"changelog": "type: agent\nprompt: log it\ntarget:\n  file: CHANGELOG.md\n"})
	dir := writeWorkflow(t, root, "ship", map[string]string{"announce": "type: agent\nprompt: announce\nrequires:\n  - project_id: release\n    table_pattern: changelog\n    external: true\n"})
	r := &fakeRunner{}
	wf := load(t, dir, r)
	dep, ok := wf.Task("changelog")
	if !ok || !dep.External || dep.Target != "file CHANGELOG.md" {
		t.Fatalf("sibling target not read: %+v %v", dep, ok)
	}
	opts := Options{Workdir: root, Partition: "2026-10-02", Retry: 10 * time.Millisecond}
	res, err := Run(context.Background(), wf, types(r), opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(res.Blocked(), ","); got != "changelog" || len(r.ran()) != 0 {
		t.Fatalf("without the file the run waits: blocked=%q ran=%v", got, r.ran())
	}
	os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("# v1\n"), 0o644)
	res, _ = Run(context.Background(), wf, types(r), opts)
	if res.Tasks["announce"].Status != StatusDone || res.Tasks["changelog"].Status != StatusSkipped {
		t.Fatalf("with the file it goes:\n%s", res.Summary())
	}
}

func TestRunIsCancelled(t *testing.T) {
	root := t.TempDir()
	dir := writeWorkflow(t, root, "w", map[string]string{"slow": "type: agent\nprompt: do x\n"})
	r := &fakeRunner{hang: true, started: make(chan struct{})}
	wf := load(t, dir, r)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-r.started; cancel() }()
	res, err := Run(ctx, wf, types(r), Options{Workdir: root, Retry: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if res.Tasks["slow"].Status != StatusFailed || !strings.Contains(res.Tasks["slow"].Error, "cancel") {
		t.Fatalf("\n%s", res.Summary())
	}
}

// Start answers an Execution whose Status reads mario's record while it
// runs: a task whose turn is going is "running", with its attempt and time.
func TestStatusReadsMariosRecordWhileRunning(t *testing.T) {
	root := t.TempDir()
	dir := writeWorkflow(t, root, "w", map[string]string{"slow": "type: agent\nprompt: do x\n"})
	r := &fakeRunner{hang: true, started: make(chan struct{})}
	wf := load(t, dir, r)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exe, err := Start(ctx, wf, types(r), Options{Workdir: root, Retry: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	<-r.started
	time.Sleep(20 * time.Millisecond)
	st := exe.Status().Tasks["slow"]
	if st.Status != StatusRunning || st.Attempt != 1 || st.Took <= 0 || exe.Done() {
		t.Fatalf("mid-run status = %+v done=%v", st, exe.Done())
	}
	cancel()
	res := exe.Wait()
	if res.Tasks["slow"].Status != StatusFailed || !exe.Done() {
		t.Fatalf("after cancel: %+v", res.Tasks["slow"])
	}
}

// A workflow in the shared library (~/.memdoor/workflows) runs in any
// project; the project's own of the same name wins; a path still works.
func TestTheSharedLibraryIsFoundAfterTheProjectsOwn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := t.TempDir()
	writeWorkflow(t, filepath.Join(home, ".memdoor", "workflows"), "release", map[string]string{"a": "type: agent\nprompt: shared\n"})
	writeWorkflow(t, filepath.Join(home, ".memdoor", "workflows"), "nightly", map[string]string{"a": "type: agent\nprompt: shared\n"})
	writeWorkflow(t, filepath.Join(project, ".memdoor", "workflows"), "release", map[string]string{"a": "type: agent\nprompt: own\n"})
	if got := strings.Join(Names(project), ","); got != "release,nightly" {
		t.Fatalf("names = %q: the project's first, then the library's it does not shadow", got)
	}
	p, err := Find(project, "release")
	if err != nil || !strings.HasPrefix(p, project) {
		t.Fatalf("the project's own wins: %q %v", p, err)
	}
	p, err = Find(project, "nightly")
	if err != nil || !strings.HasPrefix(p, home) {
		t.Fatalf("the library's is found: %q %v", p, err)
	}
	if _, err := Find(project, "ghost"); err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("a missing one is named: %v", err)
	}
	adhoc := writeWorkflow(t, t.TempDir(), "adhoc", map[string]string{"a": "type: agent\nprompt: x\n"})
	if p, err := Find(project, adhoc); err != nil || p != adhoc {
		t.Fatalf("a path still works: %q %v", p, err)
	}
}

// A TUI opened in a subdirectory still sees the project's workflows, and
// the project's .memdoor/.gitignore keeps run outputs out of commits.
func TestWorkflowsAreFoundFromASubdirectoryAndOutputsAreIgnored(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	dir := writeWorkflow(t, filepath.Join(project, ".memdoor", "workflows"), "digest", map[string]string{"a": "type: agent\nprompt: do x\n"})
	sub := filepath.Join(project, "cmd", "deep")
	os.MkdirAll(sub, 0o755)
	if ProjectRoot(sub) != project {
		t.Fatalf("root of %s = %q", sub, ProjectRoot(sub))
	}
	if p, err := Find(sub, "digest"); err != nil || p != dir {
		t.Fatalf("found %q %v", p, err)
	}
	if got := strings.Join(Names(sub), ","); got != "digest" {
		t.Fatalf("names from a subdirectory = %q", got)
	}
	wf := load(t, dir, &fakeRunner{})
	r := &fakeRunner{}
	if _, err := Run(context.Background(), wf, types(r), Options{Workdir: sub, Retry: 10 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if got := r.workdir(); got != project {
		t.Fatalf("the agent's turn runs at the project root, got %q", got)
	}
	b, err := os.ReadFile(filepath.Join(project, ".memdoor", ".gitignore"))
	if err != nil || !strings.Contains(string(b), "runs/") {
		t.Fatalf(".memdoor/.gitignore = %q %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(project, ".memdoor", "runs", "digest")); err != nil {
		t.Fatal("outputs live under the project's .memdoor/runs, not the subdirectory's")
	}
}

// A workflow's README beside its steps is its description — the first
// paragraph, headings skipped, one line — and where it comes from is said.
func TestDescribeReadsTheReadmeLead(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := t.TempDir()
	own := writeWorkflow(t, filepath.Join(project, ".memdoor", "workflows"), "digest", map[string]string{"a": "type: agent\nprompt: do x\n"})
	os.WriteFile(filepath.Join(own, "README.md"), []byte("# digest\n\nLists the last commits\nand writes DIGEST.md.\n\nMore below, not shown.\n"), 0o644)
	writeWorkflow(t, filepath.Join(home, ".memdoor", "workflows"), "nightly", map[string]string{"a": "type: agent\nprompt: do y\n"})
	entries := Describe(project)
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].Name != "digest" || entries[0].Source != "project" || entries[0].Description != "Lists the last commits and writes DIGEST.md." {
		t.Fatalf("own = %+v", entries[0])
	}
	if entries[1].Name != "nightly" || entries[1].Source != "library" || entries[1].Description != "" {
		t.Fatalf("library = %+v", entries[1])
	}
	// The README is not a task: the DAG still has one task.
	wf := load(t, own, &fakeRunner{})
	if len(wf.Tasks) != 1 {
		t.Fatalf("tasks = %d: README.md beside the steps is not a task", len(wf.Tasks))
	}
}

func TestEnsureIgnoredLeavesAnExistingFileAlone(t *testing.T) {
	project := t.TempDir()
	os.MkdirAll(filepath.Join(project, ".memdoor", "workflows"), 0o755)
	os.WriteFile(filepath.Join(project, ".memdoor", ".gitignore"), []byte("# mine\n*\n"), 0o644)
	EnsureIgnored(project)
	if b, _ := os.ReadFile(filepath.Join(project, ".memdoor", ".gitignore")); string(b) != "# mine\n*\n" {
		t.Fatalf("a person's ignore file is theirs: %q", b)
	}
	fresh := t.TempDir()
	EnsureIgnored(fresh)
	if b, _ := os.ReadFile(filepath.Join(fresh, ".memdoor", ".gitignore")); !strings.Contains(string(b), "runs/") {
		t.Fatalf("a project without one gets runs/ ignored: %q", b)
	}
}

func TestNamesWithNoLibraryAndNoProject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := Names(t.TempDir()); len(got) != 0 {
		t.Fatalf("names = %v", got)
	}
	if OutputPath("/p", "2026-10-02", "wf.steps.task") != filepath.Join("/p", ".memdoor", "runs", "wf", "steps", "task", "2026-10-02") {
		t.Fatal("outputs live under the project's .memdoor/runs by workflow, group, task, partition")
	}
}

// Sinks are the tasks nothing requires; an external is never a sink.
func TestSinksSkipExternalsAndRequiredTasks(t *testing.T) {
	dir := writeWorkflow(t, t.TempDir(), "w", map[string]string{
		"a": "type: agent\nprompt: a\n",
		"b": "type: agent\nprompt: b\nrequires:\n  - table_pattern: a\n  - table_pattern: gate\n    external: true\n",
		"c": "type: command\ncommand: \"true\"\n",
	})
	wf := load(t, dir, &fakeRunner{})
	if got := strings.Join(wf.Sinks(), ","); got != "w.steps.b,w.steps.c" {
		t.Fatalf("sinks = %q", got)
	}
}

// A result reads for a person: one line per task, name order, with what failed.
func TestResultSummaryAndDone(t *testing.T) {
	r := Result{Tasks: map[string]TaskResult{
		"b":    {Status: StatusFailed, Error: "exit 1", Took: 1500 * time.Millisecond},
		"a":    {Status: StatusDone, Took: 2 * time.Second},
		"c":    {Status: StatusSkipped},
		"gate": {Status: StatusWaiting, External: true},
	}}
	if r.Done() != 2 || !r.Failed() || strings.Join(r.Blocked(), ",") != "gate" {
		t.Fatalf("done=%d failed=%v blocked=%v", r.Done(), r.Failed(), r.Blocked())
	}
	want := "done     a  (2s)\nfailed   b  (1.5s)  — exit 1\nskipped  c\nwaiting  gate"
	if got := r.Summary(); got != want {
		t.Fatalf("summary:\n%s\nwant:\n%s", got, want)
	}
}

// Creating a workflow, the way the coder does it from a sentence: a README
// and one file per step under the project's .memdoor/workflows/<name>/, then
// it is listed, found, loaded, run, and its outputs kept — nothing else.
func TestCreatingAWorkflowFromFilesToARun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	dir := filepath.Join(project, ".memdoor", "workflows", "digest")
	os.MkdirAll(filepath.Join(dir, "steps"), 0o755)
	files := map[string]string{
		"README.md":          "# digest\n\nThe last commits, summarized, written to DIGEST.md.\n",
		"steps/log.yaml":     "type: command\ncommand: printf 'c1\\nc2\\n'\n",
		"steps/summary.yaml": "type: llm\nprompt: |\n  Summarize:\n  {{ output \"log\" }}\nrequires:\n  - table_pattern: log\n",
		"steps/write.yaml":   "type: agent\nprompt: write DIGEST.md from {{ output \"summary\" }}\nrequires:\n  - table_pattern: summary\ntarget:\n  file: DIGEST.md\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Join(Names(project), ","); got != "digest" {
		t.Fatalf("listed as %q", got)
	}
	if e := Describe(project)[0]; e.Description != "The last commits, summarized, written to DIGEST.md." || e.Source != "project" {
		t.Fatalf("described as %+v", e)
	}
	found, err := Find(project, "digest")
	if err != nil || found != dir {
		t.Fatalf("found %q %v", found, err)
	}
	r := &fakeRunner{}
	wf := load(t, found, r)
	if got := strings.Join(wf.Sinks(), ","); got != "digest.steps.write" {
		t.Fatalf("sinks = %q", got)
	}
	res, err := Run(context.Background(), wf, types(r), Options{Workdir: project, Partition: "2026-10-02", Retry: 10 * time.Millisecond})
	if err != nil || res.Done() != 3 {
		t.Fatalf("run: %v\n%s", err, res.Summary())
	}
	if b, _ := os.ReadFile(OutputPath(project, "2026-10-02", "digest.steps.summary")); !strings.Contains(string(b), "c1") {
		t.Fatalf("the llm read the command's output: %q", b)
	}
	if _, err := os.Stat(filepath.Join(project, "DIGEST.md")); err != nil {
		t.Fatal("the agent's target was made")
	}
	if b, _ := os.ReadFile(filepath.Join(project, ".memdoor", ".gitignore")); !strings.Contains(string(b), "runs/") {
		t.Fatal("the outputs are ignored")
	}
}

// A git root is a project root too: from a subdirectory of a repo with no
// .memdoor yet, the first run's outputs land at the root, not in src/.
func TestAGitRootIsAProjectRoot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	os.MkdirAll(filepath.Join(project, ".git"), 0o755)
	sub := filepath.Join(project, "src", "deep")
	os.MkdirAll(sub, 0o755)
	if ProjectRoot(sub) != project {
		t.Fatalf("root of %s = %q", sub, ProjectRoot(sub))
	}
	if OutputsDir(sub) != filepath.Join(project, ".memdoor", "runs") {
		t.Fatalf("outputs = %q", OutputsDir(sub))
	}
	loose := t.TempDir() // no .git, no .memdoor: the directory itself
	if ProjectRoot(loose) != loose {
		t.Fatalf("a loose directory is its own root, got %q", ProjectRoot(loose))
	}
}

// A cycle is refused when the files are read, naming the loop.
func TestLoadRefusesACycle(t *testing.T) {
	dir := writeWorkflow(t, t.TempDir(), "w", map[string]string{
		"a": "type: command\ncommand: echo a\nrequires:\n  - table_pattern: b\n",
		"b": "type: command\ncommand: echo b\nrequires:\n  - table_pattern: a\n",
	})
	_, err := Load(dir, Registry(types(&fakeRunner{})...))
	if err == nil || !strings.Contains(err.Error(), "cycle") || !strings.Contains(err.Error(), "w.steps.a") {
		t.Fatalf("err = %v", err)
	}
}

// `model:` on an agent task is part of the task: the runner pins the
// task's session with it (a workflow on Groq, Greg 2026-10-02).
func TestAnAgentTaskCarriesItsModel(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "steps", "write.yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("type: agent\nmodel: groq:qwen/qwen3.8-27b\nbudget: 300000\noutput_schema:\n  type: object\n  required: [ticker]\nprompt: Write DIGEST.md.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wf, err := Load(dir, Registry(Agent(nil)))
	if err != nil {
		t.Fatal(err)
	}
	task, ok := wf.Task("write")
	if !ok || task.Model != "groq:qwen/qwen3.8-27b" || task.Budget != 300_000 || !strings.Contains(task.OutputSchema, `"required":["ticker"]`) {
		t.Fatalf("the model and the budget travel with the task: %+v %v", task, ok)
	}
}

// The run table is never committed: a fresh .memdoor/.gitignore lists it,
// an older one that only had runs/ gains it, and a project that ignores
// everything is left alone.
func TestEnsureIgnoredCoversTheRunTable(t *testing.T) {
	fresh := t.TempDir()
	EnsureIgnored(fresh)
	b, _ := os.ReadFile(filepath.Join(fresh, ".memdoor", ".gitignore"))
	for _, want := range []string{"runs/", "runs.db", "runs.db-*"} {
		if !strings.Contains(string(b), want+"\n") {
			t.Fatalf("fresh file lacks %q:\n%s", want, b)
		}
	}
	old := t.TempDir()
	os.MkdirAll(filepath.Join(old, ".memdoor"), 0o755)
	os.WriteFile(filepath.Join(old, ".memdoor", ".gitignore"), []byte("# mine\nruns/\n"), 0o644)
	EnsureIgnored(old)
	b, _ = os.ReadFile(filepath.Join(old, ".memdoor", ".gitignore"))
	if !strings.HasPrefix(string(b), "# mine\nruns/\n") || !strings.Contains(string(b), "runs.db\n") || strings.Count(string(b), "runs/\n") != 1 {
		t.Fatalf("an older file keeps its lines and gains the table's:\n%s", b)
	}
	// The bare "*" an image paste used to write hid workflows/: it becomes
	// the standard list. A person's own "*" among other lines stays theirs.
	pasted := t.TempDir()
	os.MkdirAll(filepath.Join(pasted, ".memdoor"), 0o755)
	os.WriteFile(filepath.Join(pasted, ".memdoor", ".gitignore"), []byte("*\n"), 0o644)
	EnsureIgnored(pasted)
	b, _ = os.ReadFile(filepath.Join(pasted, ".memdoor", ".gitignore"))
	if string(b) != ignoreFile {
		t.Fatalf("the paste's bare * becomes the standard list, workflows committable:\n%s", b)
	}
	all := t.TempDir()
	os.MkdirAll(filepath.Join(all, ".memdoor"), 0o755)
	os.WriteFile(filepath.Join(all, ".memdoor", ".gitignore"), []byte("# mine\n*\n"), 0o644)
	EnsureIgnored(all)
	b, _ = os.ReadFile(filepath.Join(all, ".memdoor", ".gitignore"))
	if string(b) != "# mine\n*\n" {
		t.Fatalf("a person's own file that ignores everything is left alone: %q", b)
	}
}

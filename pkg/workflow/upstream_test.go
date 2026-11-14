package workflow

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/guregodevo/mario/templates"
)

// A requirement on another workflow (project_id) that is not external pulls that workflow's task, and what it
// requires, into this run as upstream. The sibling's other tasks stay out.
func TestARequirementOnAnotherWorkflowRunsItUpstream(t *testing.T) {
	root := t.TempDir()
	writeWorkflow(t, root, "lower", map[string]string{
		"base":   "type: command\ncommand: echo base\n",
		"make":   "type: command\ncommand: echo made\nrequires:\n  - table_pattern: base\n",
		"unused": "type: command\ncommand: echo unused\n",
	})
	dir := writeWorkflow(t, root, "upper", map[string]string{
		"use": "type: command\ncommand: echo used\nrequires:\n  - project_id: lower\n    dataset_id: steps\n    table_pattern: make\n",
	})
	r := &fakeRunner{}
	wf := load(t, dir, r)
	if _, ok := wf.Defs["lower.steps.make"]; !ok {
		t.Fatalf("lower.steps.make not pulled: %v", keys(wf.Defs))
	}
	if _, ok := wf.Defs["lower.steps.base"]; !ok {
		t.Fatalf("what make requires was not pulled: %v", keys(wf.Defs))
	}
	if _, ok := wf.Defs["lower.steps.unused"]; ok {
		t.Fatalf("an unrequired sibling task came in: %v", keys(wf.Defs))
	}
	if got := strings.Join(wf.Sinks(), ","); got != "upper.steps.use" {
		t.Fatalf("sinks %q", got)
	}
	rec := &recorder{}
	res, err := Run(context.Background(), wf, types(r), Options{Workdir: root, Partition: "p", Events: rec, Retry: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if res.Done() != 3 || res.Failed() {
		t.Fatalf("\n%s", res.Summary())
	}
	got := rec.joined()
	if !(strings.Index(got, "done base") < strings.Index(got, "started make") && strings.Index(got, "done make") < strings.Index(got, "started use")) {
		t.Fatalf("not in dependency order: %s", got)
	}
}

// An external requirement on another workflow stays a read of its target: nothing of it is pulled in.
func TestAnExternalRequirementOnAnotherWorkflowIsNotPulledIn(t *testing.T) {
	root := t.TempDir()
	writeWorkflow(t, root, "lower", map[string]string{"make": "type: command\ncommand: echo made\ntarget:\n  file: made.txt\n"})
	dir := writeWorkflow(t, root, "upper", map[string]string{
		"use": "type: command\ncommand: echo used\nrequires:\n  - project_id: lower\n    dataset_id: steps\n    table_pattern: make\n    external: true\n",
	})
	wf := load(t, dir, &fakeRunner{})
	if _, ok := wf.Defs["lower.steps.make"]; ok {
		t.Fatalf("an external requirement pulled the sibling in: %v", keys(wf.Defs))
	}
}

// A requirement on a workflow that is not there is still refused, with the loader's own message.
func TestARequirementOnAMissingWorkflowIsRefused(t *testing.T) {
	root := t.TempDir()
	dir := writeWorkflow(t, root, "upper", map[string]string{
		"use": "type: command\ncommand: echo used\nrequires:\n  - project_id: nowhere\n    dataset_id: steps\n    table_pattern: make\n",
	})
	if _, err := Load(dir, Registry(types(&fakeRunner{})...)); err == nil || !strings.Contains(err.Error(), "has no definition and is not external") {
		t.Fatalf("err %v", err)
	}
}

func keys(m map[string]*templates.YamlTaskDefinition) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// The pulled workflow runs after what the requiring task requires here: the sibling's first task waits for it.
func TestAPulledWorkflowRunsAfterWhatTheRequiringTaskRequires(t *testing.T) {
	root := t.TempDir()
	writeWorkflow(t, root, "lower", map[string]string{
		"base": "type: command\ncommand: echo base\n",
		"make": "type: command\ncommand: echo made\nrequires:\n  - table_pattern: base\n",
	})
	dir := writeWorkflow(t, root, "upper", map[string]string{
		"prep": "type: command\ncommand: echo prep\n",
		"use":  "type: command\ncommand: echo used\nrequires:\n  - table_pattern: prep\n  - project_id: lower\n    dataset_id: steps\n    table_pattern: make\n",
	})
	r := &fakeRunner{}
	wf := load(t, dir, r)
	rec := &recorder{}
	res, err := Run(context.Background(), wf, types(r), Options{Workdir: root, Partition: "p", Events: rec, Retry: 20 * time.Millisecond})
	if err != nil || res.Failed() || res.Done() != 4 {
		t.Fatalf("err %v\n%s", err, res.Summary())
	}
	got := rec.joined()
	if !(strings.Index(got, "done prep") < strings.Index(got, "started base") && strings.Index(got, "done make") < strings.Index(got, "started use")) {
		t.Fatalf("prep must finish before the pulled workflow starts: %s", got)
	}
}

// A task that needs a sibling's task and also its own upstream, which comes from the same sibling, is not a
// cycle: what its own requirements need stays ahead of it, and only the rest of the sibling waits.
func TestAPulledWorkflowDoesNotWaitOnWhatItsOwnRequirementsPulled(t *testing.T) {
	root := t.TempDir()
	writeWorkflow(t, root, "lower", map[string]string{
		"first":  "type: command\ncommand: echo first\n",
		"second": "type: command\ncommand: echo second\nrequires:\n  - table_pattern: first\n",
		"side":   "type: command\ncommand: echo side\n",
	})
	dir := writeWorkflow(t, root, "upper", map[string]string{
		"hand": "type: command\ncommand: echo hand\nrequires:\n  - project_id: lower\n    dataset_id: steps\n    table_pattern: second\n",
		"end":  "type: command\ncommand: echo end\nrequires:\n  - table_pattern: hand\n  - project_id: lower\n    dataset_id: steps\n    table_pattern: side\n",
	})
	r := &fakeRunner{}
	wf := load(t, dir, r)
	rec := &recorder{}
	res, err := Run(context.Background(), wf, types(r), Options{Workdir: root, Partition: "p", Events: rec, Retry: 20 * time.Millisecond})
	if err != nil || res.Failed() || res.Done() != 5 {
		t.Fatalf("err %v\n%s", err, res.Summary())
	}
	got := rec.joined()
	if !(strings.Index(got, "done second") < strings.Index(got, "started hand") && strings.Index(got, "done hand") < strings.Index(got, "started side")) {
		t.Fatalf("hand must follow second, side must follow hand: %s", got)
	}
}

// with: binds the pulled workflow's args to this one's outputs, as Go templates: the pulled task runs
// with the value the requiring workflow produced.
func TestWithBindsAPulledWorkflowsArgsToThisOnesOutputs(t *testing.T) {
	root := t.TempDir()
	writeWorkflow(t, root, "lower", map[string]string{
		"make": "type: command\nargs:\n  dir: .\ncommand: echo made in {{.dir}}\n",
	})
	dir := writeWorkflow(t, root, "upper", map[string]string{
		"hand": "type: command\ncommand: printf /tmp/wt-7\n",
		"use":  "type: command\ncommand: echo used\nrequires:\n  - table_pattern: hand\n  - project_id: lower\n    dataset_id: steps\n    table_pattern: make\n    with:\n      dir: '{{ output \"upper.steps.hand\" }}'\n",
	})
	r := &fakeRunner{}
	wf := load(t, dir, r)
	res, err := Run(context.Background(), wf, types(r), Options{Workdir: root, Partition: "p", Retry: 20 * time.Millisecond})
	if err != nil || res.Failed() {
		t.Fatalf("err %v\n%s", err, res.Summary())
	}
	if b, _ := os.ReadFile(OutputPath(root, "p", "lower.steps.make")); strings.TrimSpace(string(b)) != "made in /tmp/wt-7" {
		t.Fatalf("lower.make ran with %q", b)
	}
}

// A with: key no pulled task declares is refused when the files are read.
func TestWithOnAnUndeclaredArgIsRefused(t *testing.T) {
	root := t.TempDir()
	writeWorkflow(t, root, "lower", map[string]string{"make": "type: command\ncommand: echo made\n"})
	dir := writeWorkflow(t, root, "upper", map[string]string{
		"use": "type: command\ncommand: echo used\nrequires:\n  - project_id: lower\n    dataset_id: steps\n    table_pattern: make\n    with:\n      dri: x\n",
	})
	if _, err := Load(dir, Registry(types(&fakeRunner{})...)); err == nil || !strings.Contains(err.Error(), "binds dri, which no task of lower.steps.make declares") {
		t.Fatalf("err %v", err)
	}
}

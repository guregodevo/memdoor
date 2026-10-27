package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A workflow's command steps run with shell.Env: the `when`, the command and
// its command target never see a model key the gateway holds (2026-10-27,
// "dont share key"; mario Base.Env). Each of the three holds only when the
// key is absent, so a leak fails the run.
func TestACommandStepNeverSeesAModelKey(t *testing.T) {
	t.Setenv("OPEN_ROUTER_API_KEY", "sk-or-must-not-leak")
	t.Setenv("MEMDOOR_ENV_TEST_KEPT", "kept")
	root := t.TempDir()
	dir := writeWorkflow(t, root, "probe", map[string]string{
		"probe": "type: command\n" +
			"when: test -z \"$OPEN_ROUTER_API_KEY\"\n" +
			"command: printf '[%s][%s]' \"$OPEN_ROUTER_API_KEY\" \"$MEMDOOR_ENV_TEST_KEPT\" > probe.txt\n" +
			"target:\n  command: test -f probe.txt && test -z \"$OPEN_ROUTER_API_KEY\"\n",
	})
	r := &fakeRunner{}
	wf := load(t, dir, r)
	res, err := Run(context.Background(), wf, types(r), Options{Workdir: root, Partition: "2026-10-27", Retry: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed() || res.Tasks["probe"].Status != StatusDone {
		t.Fatalf("the step did not run clean:\n%s", res.Summary())
	}
	b, _ := os.ReadFile(filepath.Join(root, "probe.txt"))
	if string(b) != "[][kept]" {
		t.Fatalf("the command saw %q, want no key and the rest of the environment", b)
	}
}

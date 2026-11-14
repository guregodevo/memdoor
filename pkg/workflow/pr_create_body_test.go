package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// prCreateBody runs the built-in pr-create's real body step beside a stand-in pr-description and a stand-in
// checks step, and returns the body it wrote. issue is the body step's issue arg; checks is what the checks
// step prints (empty: it printed nothing, as when no check is asked for).
func prCreateBody(t *testing.T, issue, checks string) string {
	t.Helper()
	real, err := os.ReadFile(filepath.Join("..", "..", "workflows", "pr-create", "steps", "body.yaml"))
	if err != nil {
		t.Skip("no built-in pr-create beside this package")
	}
	root := t.TempDir()
	writeWorkflow(t, root, "pr-description", map[string]string{
		"describe":      "type: command\ncommand: mkdir -p .memdoor/runs && printf 'Fix the estimate\\n\\n**What**\\nIt uses the window.\\n' > .memdoor/runs/pr-description-{{.partition}}.md\n",
		"describe-lean": "type: command\nwhen: 'false'\ncommand: echo lean\n",
	})
	dir := writeWorkflow(t, root, "pr-create", map[string]string{
		"checks": "type: command\ncommand: printf '" + checks + "'\n",
		"body":   strings.Replace(string(real), `issue: ""`, `issue: "`+issue+`"`, 1),
	})
	r := &fakeRunner{}
	wf := load(t, dir, r)
	res, err := Run(context.Background(), wf, types(r), Options{Workdir: root, Partition: "p", Retry: 20 * time.Millisecond})
	if err != nil || res.Failed() {
		t.Fatalf("err %v\n%s", err, res.Summary())
	}
	b, err := os.ReadFile(filepath.Join(root, ".memdoor", "runs", "pr-create-p", "body.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The body is the description pr-description wrote, the issue it fixes, and the checks: a ticked box when
// their output ends `exit 0`, with the output under it.
func TestPRCreateBodyTicksTheChecksThatPassedAndLinksTheIssue(t *testing.T) {
	body := prCreateBody(t, "27", `ok  pkg\nexit 0\n`)
	for _, want := range []string{"Fix the estimate", "It uses the window.", "Fixes #27", "- [x] the checks pass", "ok  pkg"} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %q:\n%s", want, body)
		}
	}
}

// Failed checks leave the box unticked with their output; no issue writes no Fixes line; no check run says so.
func TestPRCreateBodyReportsFailedChecksAndInventsNothing(t *testing.T) {
	failed := prCreateBody(t, "", `FAIL pkg\nexit 1\n`)
	if !strings.Contains(failed, "- [ ] the checks pass") || !strings.Contains(failed, "FAIL pkg") || strings.Contains(failed, "Fixes #") {
		t.Errorf("failed checks:\n%s", failed)
	}
	none := prCreateBody(t, "", ``)
	if !strings.Contains(none, "None run.") || strings.Contains(none, "[x]") || strings.Contains(none, "[ ]") {
		t.Errorf("no checks:\n%s", none)
	}
}

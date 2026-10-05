package ui

import (
	"github.com/charmbracelet/lipgloss"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWorkflowLayersFollowRequirements(t *testing.T) {
	tasks := []WorkflowTask{
		{Name: "deploy", Requires: []string{"approve", "changelog"}},
		{Name: "tests"},
		{Name: "changelog", Requires: []string{"tests"}},
		{Name: "approve", Requires: []string{"changelog"}, External: true},
		{Name: "lint"},
	}
	layers := workflowLayers(tasks)
	want := [][]string{{"tests", "lint"}, {"changelog"}, {"approve"}, {"deploy"}}
	if len(layers) != len(want) {
		t.Fatalf("layers = %d, want %d", len(layers), len(want))
	}
	for i, l := range layers {
		var names []string
		for _, x := range l {
			names = append(names, x.Name)
		}
		if strings.Join(names, ",") != strings.Join(want[i], ",") {
			t.Fatalf("layer %d = %v, want %v", i, names, want[i])
		}
	}
}

func TestWorkflowLayersSurviveACycle(t *testing.T) {
	layers := workflowLayers([]WorkflowTask{{Name: "a", Requires: []string{"b"}}, {Name: "b", Requires: []string{"a"}}})
	n := 0
	for _, l := range layers {
		n += len(l)
	}
	if n != 2 {
		t.Fatalf("every task must land in a layer, got %d of 2", n)
	}
}

func TestWorkflowEventRedrawsTheOpenRun(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.workflowPanel = &workflowPanelState{screen: workflowScreenRun,
		run:  &WorkflowRun{ID: "r1", Workflow: "release", Tasks: []WorkflowTask{{Name: "tests", State: "running"}}},
		runs: []WorkflowRun{{ID: "r1", State: "running"}}}
	fresh := &WorkflowRun{ID: "r1", Workflow: "release", State: "running", Done: 1, Total: 1,
		Tasks: []WorkflowTask{{Name: "tests", State: "done", Took: "4s"}}}
	handled, _ := m.updateWorkflowPanel(workflowEventMsg{runID: "r1", task: "tests", state: "done", status: fresh})
	if !handled {
		t.Fatal("a workflow event is the panel's to handle")
	}
	if got := m.workflowPanel.run.Tasks[0].State; got != "done" {
		t.Fatalf("open run not redrawn from the event: state = %q", got)
	}
	if got := m.workflowPanel.runs[0].Done; got != 1 {
		t.Fatalf("list not updated from the event: done = %d", got)
	}
	out := m.renderWorkflowPanel()
	if !strings.Contains(out, "✓") || !strings.Contains(out, "tests") {
		t.Fatalf("rendered panel lacks the done task:\n%s", out)
	}
}

func TestWorkflowEventForAnotherRunLeavesThePanelAlone(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.workflowPanel = &workflowPanelState{screen: workflowScreenRun,
		run: &WorkflowRun{ID: "r1", Tasks: []WorkflowTask{{Name: "tests", State: "running"}}}}
	m.updateWorkflowPanel(workflowEventMsg{runID: "r2", status: &WorkflowRun{ID: "r2", Tasks: []WorkflowTask{{Name: "tests", State: "done"}}}})
	if got := m.workflowPanel.run.Tasks[0].State; got != "running" {
		t.Fatalf("another run's event changed this one: %q", got)
	}
}

func TestWorkflowRunErrorReadsInThePanel(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.workflowPanel = &workflowPanelState{loading: "Starting …"}
	m.updateWorkflowPanel(workflowRunMsg{err: errString("no workflows/x.yaml")})
	if m.workflowPanel.loading != "" || !strings.Contains(m.workflowPanel.err, "no workflows/x.yaml") {
		t.Fatalf("refused run must read in words: loading=%q err=%q", m.workflowPanel.loading, m.workflowPanel.err)
	}
}

func TestWorkflowKeysWithoutAPanelAreNotTaken(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	handled, _ := m.updateWorkflowPanel(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if handled {
		t.Fatal("with no panel open a key belongs to the input")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestEscStopsThisWindowsRun(t *testing.T) {
	stopped := ""
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.workflow.Stop = func(id string) (WorkflowRun, error) { stopped = id; return WorkflowRun{ID: id, State: "stopped"}, nil }
	m.updateWorkflowPanel(workflowEventMsg{status: &WorkflowRun{ID: "r1", State: "running"}})
	if m.workflowRunning != "r1" {
		t.Fatalf("a running event names the window's run, got %q", m.workflowRunning)
	}
	next, cmd := m.update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc with a run going must act")
	}
	cmd()
	m = next.(Model)
	if stopped != "r1" || m.workflowRunning != "" {
		t.Fatalf("Esc stopped %q (want r1), running=%q", stopped, m.workflowRunning)
	}
	m.updateWorkflowPanel(workflowEventMsg{status: &WorkflowRun{ID: "r1", State: "stopped"}})
	if m.workflowRunning != "" {
		t.Fatal("a stopped run is no longer the window's")
	}
}

// The run's DAG is one block in the conversation, redrawn in place.
func TestWorkflowGraphIsRedrawnInPlaceInTheConversation(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.note("before")
	m.updateWorkflowPanel(workflowEventMsg{status: &WorkflowRun{ID: "r1", Workflow: "echo", State: "running", Total: 2,
		Tasks: []WorkflowTask{{Name: "b", State: "running"}, {Name: "a", State: "waiting", Requires: []string{"b"}}}}})
	m.updateWorkflowPanel(workflowEventMsg{status: &WorkflowRun{ID: "r1", Workflow: "echo", State: "done", Done: 2, Total: 2,
		Tasks: []WorkflowTask{{Name: "b", State: "done", Took: "10s"}, {Name: "a", State: "done", Took: "11s", Requires: []string{"b"}}}}})
	blocks := 0
	var last string
	for _, msg := range m.messages {
		if msg.ToolID == "workflow:r1" {
			blocks++
			last = msg.Content
		}
	}
	if blocks != 1 {
		t.Fatalf("one block per run, got %d", blocks)
	}
	if !strings.Contains(last, "done · 2/2 done") || !strings.Contains(last, "✓ a") || !strings.Contains(last, "← b") {
		t.Fatalf("block not redrawn from the latest state:\n%s", last)
	}
	m.updateWorkflowPanel(workflowEventMsg{status: &WorkflowRun{ID: "r2", Workflow: "other", State: "running", Total: 1, Tasks: []WorkflowTask{{Name: "x", State: "running"}}}})
	if n := len(m.messages); n != 4 { // the welcome note, "before", r1, r2
		var kinds []string
		for _, x := range m.messages {
			kinds = append(kinds, x.Role+":"+x.ToolID+":"+truncateEnd(x.Content, 30))
		}
		t.Fatalf("a second run gets its own block: %d messages, want 4: %v", n, kinds)
	}
}

func TestTheDAGBlockCopiesWithoutColours(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.updateWorkflowPanel(workflowEventMsg{status: &WorkflowRun{ID: "r1", Workflow: "digest", State: "done", Done: 1, Total: 1, Tasks: []WorkflowTask{{Name: "log", State: "done"}}}})
	for _, msg := range m.messages {
		if msg.Role == "workflow" {
			if plain := plainText(msg.Content); strings.Contains(plain, "\x1b[") || !strings.Contains(plain, "✓ log") {
				t.Fatalf("copy text = %q", plain)
			}
		}
	}
}

// The project's workflows are slash commands: the dropdown completes
// "/workflow:" with what workflows/ holds, and the command runs it.
func TestTheProjectsWorkflowsAreSlashCommands(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // the person's own library is not this test's
	dir := t.TempDir()
	for _, w := range []string{"release", "digest"} {
		os.MkdirAll(filepath.Join(dir, ".memdoor", "workflows", w, "steps"), 0o755)
		os.WriteFile(filepath.Join(dir, ".memdoor", "workflows", w, "steps", "a.yaml"), []byte("type: agent\nprompt: x\n"), 0o644)
	}
	os.MkdirAll(filepath.Join(dir, ".memdoor", "workflows", "empty"), 0o755)
	if got := strings.Join(workflowNames(dir), ","); got != "digest,release" {
		t.Fatalf("workflowNames = %q", got)
	}
	got := filterCommands([]string{"/workflow", "/workflow:digest", "/workflow:release", "/model"}, "/workflow:r")
	if strings.Join(got, ",") != "/workflow:release" {
		t.Fatalf("completion = %v", got)
	}
	started := ""
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.workflow.Run = func(name, partition string) (WorkflowRun, error) {
		started = name + "@" + partition
		return WorkflowRun{ID: "r"}, nil
	}
	if cmd := m.runNetworkCmd([]string{"/workflow:release"}); cmd == nil {
		t.Fatal("the command runs the workflow")
	} else {
		cmd()
	}
	if started != "release@" {
		t.Fatalf("started = %q", started)
	}
}

// The panel shows what a workflow is and where it comes from, from its README.
func TestThePanelShowsAWorkflowsDescriptionAndSource(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.width = 120
	m.workflowPanel = &workflowPanelState{files: []WorkflowEntry{
		{Name: "digest", Source: "library", Description: "Lists the last commits and writes DIGEST.md."},
		{Name: "echo"},
	}}
	out := plainText(m.renderWorkflowPanel())
	if !strings.Contains(out, "digest") || !strings.Contains(out, "library · Lists the last commits") || !strings.Contains(out, "enter runs it") {
		t.Fatalf("panel:\n%s", out)
	}
}

func TestContinueKeyOnlyOnAFailedOrStoppedRun(t *testing.T) {
	resumed := ""
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.workflow.Resume = func(id string) (WorkflowRun, error) { resumed = id; return WorkflowRun{ID: id}, nil }
	for _, state := range []string{"running", "waiting", "done"} {
		m.workflowPanel = &workflowPanelState{screen: workflowScreenRun, run: &WorkflowRun{ID: "r1", State: state}}
		if cmd := m.workflowKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}); cmd != nil {
			t.Fatalf("c on a %s run must do nothing", state)
		}
	}
	for _, state := range []string{"failed", "stopped"} {
		m.workflowPanel = &workflowPanelState{screen: workflowScreenRun, run: &WorkflowRun{ID: "r1", State: state}}
		cmd := m.workflowKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
		if cmd == nil {
			t.Fatalf("c on a %s run continues it", state)
		}
		cmd()
		if resumed != "r1" {
			t.Fatalf("resumed %q", resumed)
		}
		resumed = ""
	}
}

func TestWorkflowRunCommandPassesThePartition(t *testing.T) {
	got := ""
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.workflow.List = func() ([]WorkflowRun, []WorkflowEntry, error) { return nil, nil, nil }
	m.workflow.Run = func(name, partition string) (WorkflowRun, error) {
		got = name + "@" + partition
		return WorkflowRun{ID: "r"}, nil
	}
	if cmd := m.workflowCommand([]string{"run", "digest", "today"}); cmd == nil {
		t.Fatal("run is a command")
	} else {
		cmd()
	}
	if got != "digest@today" {
		t.Fatalf("got %q", got)
	}
	if cmd := m.runNetworkCmd([]string{"/workflow:digest", "2026-10-02"}); cmd != nil {
		cmd()
	}
	if got != "digest@2026-10-02" {
		t.Fatalf("/workflow:<name> <partition> → %q", got)
	}
}

func TestTheClockStopsWithTheRun(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.updateWorkflowPanel(workflowEventMsg{runID: "r1", status: &WorkflowRun{ID: "r1", State: "running", Total: 1, Tasks: []WorkflowTask{{Name: "a", State: "running"}}}})
	if _, cmd := m.updateWorkflowPanel(workflowTickMsg(time.Now())); cmd == nil {
		t.Fatal("while a run goes, a tick schedules the next")
	}
	m.updateWorkflowPanel(workflowEventMsg{runID: "r1", status: &WorkflowRun{ID: "r1", State: "done", Done: 1, Total: 1, Tasks: []WorkflowTask{{Name: "a", State: "done"}}}})
	if _, cmd := m.updateWorkflowPanel(workflowTickMsg(time.Now())); cmd != nil {
		t.Fatal("with no run going, the clock stops")
	}
}

func TestThePanelListsRunsAfterTheWorkflows(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.width = 120
	m.workflowPanel = &workflowPanelState{files: []WorkflowEntry{{Name: "digest"}}, runs: []WorkflowRun{{ID: "digest-1", Workflow: "digest", State: "failed", Done: 1, Total: 3}}}
	out := plainText(m.renderWorkflowPanel())
	if !strings.Contains(out, "▷ digest") || !strings.Contains(out, "✗ digest") || !strings.Contains(out, "1/3") || !strings.Contains(out, "digest-1") {
		t.Fatalf("panel:\n%s", out)
	}
}

// A refusal ("already running") went stale on the screen after the run it
// named ended, and stayed under every later key.
func TestAPanelErrorDoesNotOutliveItsCause(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.workflowPanel = &workflowPanelState{}
	m.updateWorkflowPanel(workflowRunMsg{err: errString("release is already running here as r1")})
	m.updateWorkflowPanel(workflowEventMsg{runID: "r1", status: &WorkflowRun{ID: "r1", State: "done", Done: 1, Total: 1, Tasks: []WorkflowTask{{Name: "a", State: "done"}}}})
	if m.workflowPanel.err != "" {
		t.Fatalf("the run ended; the refusal is stale: %q", m.workflowPanel.err)
	}
	m.workflowPanel.err = "nothing is waiting for approval"
	m.updateWorkflowPanel(tea.KeyMsg{Type: tea.KeyDown})
	if m.workflowPanel.err != "" {
		t.Fatalf("the next key replaces the last error: %q", m.workflowPanel.err)
	}
}

// At a gate, d puts every changed line in the conversation (which scrolls),
// coloured by what it is; the panel closes so it can be read.
func TestTheGateShowsTheFullDiff(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	asked := ""
	m.workflow.Diff = func(id string) (string, error) {
		asked = id
		return "diff --git a/a.go b/a.go\n@@ -1 +1,3 @@\n package a\n+func F() {}\n-func Old() {}\n", nil
	}
	m.workflowPanel = &workflowPanelState{screen: workflowScreenRun, run: &WorkflowRun{ID: "fix-20261004-101010", State: "waiting"}}
	_, cmd := m.updateWorkflowPanel(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if cmd == nil || m.workflowPanel != nil {
		t.Fatal("d asks for the diff and closes the panel")
	}
	m.updateWorkflowPanel(cmd())
	if asked != "fix-20261004-101010" {
		t.Fatalf("diff asked for %q", asked)
	}
	last := m.messages[len(m.messages)-1]
	if last.Role != "workflow" || !strings.Contains(last.Content, "func F() {}") || !strings.Contains(last.Content, "func Old() {}") {
		t.Fatalf("the diff is not in the conversation: %+v", last)
	}
	m.workflow.Diff = func(string) (string, error) { return "", nil }
	m.workflow.List = func() ([]WorkflowRun, []WorkflowEntry, error) { return nil, nil, nil }
	m.updateWorkflowPanel(m.workflowCommand([]string{"diff", "r2"})())
	if got := m.messages[len(m.messages)-1].Content; !strings.Contains(got, "No changes in r2") {
		t.Fatalf("an empty diff says so: %q", got)
	}
}

// An unowned run is shown where it runs: the same directory, through a
// symlink too (/tmp is /private/tmp on macOS), and nowhere else.
func TestSameDirResolvesSymlinks(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skip(err)
	}
	if !sameDir(real, link) || !sameDir(real, real+"/") {
		t.Fatal("the same project through a symlink or a trailing slash is the same project")
	}
	if sameDir(real, t.TempDir()) || sameDir("", real) {
		t.Fatal("another project, or none, is not this one")
	}
}

// A RUNNING GRAPH IS NEVER FROZEN (2026-10-04: a done run's header still read
// "running" in the scrollback). While its run goes the block stays live —
// a turn ending does not settle it — and a block already printed (a run that
// waited at a gate) is drawn anew below instead of edited out of sight.
func TestARunningGraphStaysLiveAndAFrozenOneIsRedrawn(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	base := len(m.messages)
	m.upsertWorkflowGraph(WorkflowRun{ID: "r1", Workflow: "w", State: "running", Total: 1, Tasks: []WorkflowTask{{Name: "a", State: "running"}}})
	if settled(m.messages[len(m.messages)-1]) {
		t.Fatal("a running graph must not be frozen into scrollback")
	}
	m.settleStreaming()
	if settled(m.messages[len(m.messages)-1]) {
		t.Fatal("a turn ending does not settle a running graph")
	}
	m.upsertWorkflowGraph(WorkflowRun{ID: "r1", Workflow: "w", State: "waiting", Total: 1, Tasks: []WorkflowTask{{Name: "a", State: "done"}}})
	if !settled(m.messages[len(m.messages)-1]) || len(m.messages) != base+1 {
		t.Fatal("a waiting graph is redrawn in place and may settle")
	}
	m.printedThrough = base + 1 // the waiting graph went to scrollback
	m.upsertWorkflowGraph(WorkflowRun{ID: "r1", Workflow: "w", State: "done", Done: 1, Total: 1, Tasks: []WorkflowTask{{Name: "a", State: "done"}}})
	if len(m.messages) != base+2 || !strings.Contains(m.messages[base+1].Content, "w · done") {
		t.Fatalf("a printed graph is drawn anew below, with the new state: %d messages", len(m.messages))
	}
}

// A diff keeps its indentation: a Python body indented four spaces must read
// four spaces in, not one.
func TestTheGateDiffKeepsIndentation(t *testing.T) {
	out := renderGateDiff("r", "+def f():\n+    return 1\n", 120)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(out, "")
	if !strings.Contains(plain, "+    return 1") {
		t.Fatalf("indentation collapsed: %q", plain)
	}
}

// A long workflow name fits its row: no line of the panel is wider than the
// screen (dogfood 2026-10-04: "article-to-workflow" wrapped onto two lines).
func TestALongWorkflowNameDoesNotWrap(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.width = 120
	m.workflowPanel = &workflowPanelState{screen: workflowScreenList, files: []WorkflowEntry{
		{Name: "article-to-workflow", Description: strings.Repeat("a long description ", 10)},
		{Name: "jeff", Description: "short"},
	}, runs: []WorkflowRun{{ID: "article-to-workflow-20261004-155915", Workflow: "article-to-workflow", State: "done", Done: 5, Total: 5}}}
	out := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderWorkflowPanel(), "")
	if !strings.Contains(out, "▷ article-to-workflow ") {
		t.Fatalf("the name is cut across lines:\n%s", out)
	}
	for _, l := range strings.Split(out, "\n") {
		if w := lipgloss.Width(l); w > m.width {
			t.Fatalf("a %d-wide line on a %d-wide screen: %q", w, m.width, l)
		}
	}
}

// A late step event cannot move a finished run back to running (review,
// 2026-10-04: the header read "running · 3/3" after "■ digest done").
func TestAFinishedRunStaysFinished(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	done := &WorkflowRun{ID: "d1", Workflow: "digest", State: "done", Done: 3, Total: 3, Tasks: []WorkflowTask{{Name: "a", State: "done"}}}
	late := &WorkflowRun{ID: "d1", Workflow: "digest", State: "running", Done: 3, Total: 3, Tasks: []WorkflowTask{{Name: "a", State: "done"}}}
	m.updateWorkflowPanel(workflowEventMsg{runID: "d1", state: "note", text: "■ digest done", status: done})
	m.updateWorkflowPanel(workflowEventMsg{runID: "d1", task: "a", state: "done", status: late})
	if got := m.workflowLast["d1"].State; got != "done" {
		t.Fatalf("a finished run went back to %q", got)
	}
	last := m.messages[len(m.messages)-1]
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "workflow" {
			last = m.messages[i]
			break
		}
	}
	if !strings.Contains(last.Content, "digest · done") {
		t.Fatalf("the graph header stays done: %q", last.Content)
	}
}

// With the panel closed, the run's final state still reaches the screen: the
// conversation is redrawn after the graph block changes, not only before.
func TestTheFinalStateIsDrawnWithThePanelClosed(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = mm.(Model)
	running := &WorkflowRun{ID: "d1", Workflow: "digest", State: "running", Done: 3, Total: 3, Tasks: []WorkflowTask{{Name: "a", State: "done"}}}
	done := &WorkflowRun{ID: "d1", Workflow: "digest", State: "done", Done: 3, Total: 3, Tasks: []WorkflowTask{{Name: "a", State: "done"}}}
	m.updateWorkflowPanel(workflowEventMsg{runID: "d1", task: "a", state: "done", status: running})
	m.updateWorkflowPanel(workflowEventMsg{runID: "d1", state: "note", text: "■ digest done", status: done})
	screen := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.viewport.View(), "")
	if !strings.Contains(screen, "digest · done") || strings.Contains(screen, "digest · running") {
		t.Fatalf("the screen shows the final state:\n%s", screen)
	}
}

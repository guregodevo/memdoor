package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// inColour draws with the 256-colour profile a terminal has, so a test can
// read which hue a word wears; tests run with no terminal and get none.
func inColour(t *testing.T) {
	t.Helper()
	was := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(was) })
}

// fg is the SGR a word in this palette colour starts with.
func fg(c string) string { return "\x1b[38;5;" + c + "m" }

// #132: each dropdown entry says what it does, in grey on its right: a
// built-in the line /help prints, a workflow the description `memdoor
// workflow` prints, a skill its own heading. /help prints the same table.
func TestTheDropdownSaysWhatEachCommandDoes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	SetSessionDir(dir)
	t.Cleanup(func() { SetSessionDir("") })
	wf := filepath.Join(dir, ".memdoor", "workflows", "digest")
	os.MkdirAll(filepath.Join(wf, "steps"), 0o755)
	os.WriteFile(filepath.Join(wf, "steps", "a.yaml"), []byte("type: agent\nprompt: x\n"), 0o644)
	os.WriteFile(filepath.Join(wf, "README.md"), []byte("# digest\n\nSummarizes the last ten commits into DIGEST.md.\n"), 0o644)

	cmds := loadSlashCommands("coder")
	for _, c := range []struct{ entry, says string }{
		{"/w", "The workflows and their runs"},
		{"/usage", "What this workspace has used this month"},
		{"/w digest", "Summarizes the last ten commits into DIGEST.md."},
		{"/skill:workflow", "build a DAG of agent tasks"},
	} {
		if !strings.Contains(slashDescriptions[c.entry], c.says) {
			t.Errorf("%s says %q, want %q", c.entry, slashDescriptions[c.entry], c.says)
		}
	}
	if len(cmds) == 0 {
		t.Fatal("no commands")
	}

	inColour(t)
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.width = 120
	m.filteredCommands = []string{"/usage", "/w digest"}
	box := m.renderAutocomplete()
	if !strings.Contains(plainText(box), "/w digest") || !strings.Contains(plainText(box), "Summarizes the last ten commits") {
		t.Fatalf("the box:\n%s", plainText(box))
	}
	if !strings.Contains(box, fg(colDim)+"Summarizes") && !strings.Contains(box, fg(colDim)+"What this") {
		t.Fatalf("the description is not grey:\n%q", box)
	}

	help := slashHelpText()
	for _, want := range []string{"- /w - The workflows and their runs", "- /usage - What this workspace has used this month", "Shift+Tab - The mode: auto (one turn) or team"} {
		if !strings.Contains(help, want) {
			t.Errorf("/help misses %q", want)
		}
	}
	if strings.Contains(help, "Cycle the reasoning effort") {
		t.Error("/help still says Shift+Tab cycles the effort")
	}
}

// #133: the state wears its glyph's hue, stopped is no longer grey, and a
// step that ran or runs is brighter than one waiting (ADR 0018).
func TestTheWorkflowGraphWearsThePalette(t *testing.T) {
	inColour(t)
	if _, c := workflowGlyph("stopped"); c != colWarn {
		t.Fatalf("stopped wears %s, want the warning hue %s", c, colWarn)
	}
	now := time.Now()
	r := WorkflowRun{ID: "r1", Workflow: "release", State: "failed", Done: 1, Total: 4, Tasks: []WorkflowTask{
		{Name: "tests", State: "done", Took: "2s"},
		{Name: "notes", State: "failed", Requires: []string{"tests"}, Error: "exit status 1"},
		{Name: "tag", State: "stopped", Requires: []string{"tests"}},
		{Name: "announce", State: "waiting", Requires: []string{"notes"}, Started: &now},
	}}
	out := strings.Join(workflowGraphRows(r, 100, false), "\n")
	for _, c := range []struct{ word, hue string }{{"done", colOK}, {"failed", colErr}, {"stopped", colWarn}} {
		if !strings.Contains(out, fg(c.hue)+c.word) {
			t.Errorf("%q is not in its hue %s:\n%q", c.word, c.hue, out)
		}
	}
	if !strings.Contains(out, fg(colText)+"tests") || !strings.Contains(out, fg(colDim)+"announce") {
		t.Errorf("a step that ran is bright and one waiting dim:\n%q", out)
	}
	head := workflowGraphMessage(r, 100, false).Content
	if !strings.Contains(head, fg(colErr)+"failed") {
		t.Errorf("the header's state is not in its hue:\n%q", head)
	}
}

// #134: a graph of more than 8 steps is grouped by state in the
// conversation, a few lines whatever its size; ctrl+o draws every step.
func TestALongGraphFitsTheWindow(t *testing.T) {
	started := time.Now().Add(-12 * time.Second)
	var tasks []WorkflowTask
	for i, n := range []string{"problem", "outline", "plan", "settle", "baseline", "direct", "test-0", "context", "ideate-a", "ideate-b", "solve", "test", "review", "repair", "recheck", "judge", "keep", "verify"} {
		t := WorkflowTask{Name: n, State: "waiting"}
		if i > 0 {
			t.Requires = []string{"problem"}
		}
		switch {
		case i < 6:
			t.State = "done"
			t.Outcome = n + ".md"
		case i == 6:
			t.State, t.Started = "running", &started
			t.Lines = []stepLine{{text: "⏺ bash · 4s"}}
			t.About = "The check before the team: the direct attempt, as the project tests it."
			t.InputTokens, t.OutputTokens = 1200, 300
		case i == 7:
			t.State, t.Error = "failed", "exit status 1 — no prompt"
		}
		tasks = append(tasks, t)
	}
	r := WorkflowRun{ID: "r1", Workflow: "team-solve", State: "running", Done: 6, Total: 18, Tasks: tasks}

	full := workflowGraphMessage(r, 120, true).Content
	compact := workflowGraphMessage(r, 120, false).Content
	nFull, nCompact := strings.Count(full, "\n")+1, strings.Count(compact, "\n")+1
	if nCompact > 14 || nFull < 20 {
		t.Fatalf("compact %d lines, full %d:\n%s", nCompact, nFull, plainText(compact))
	}
	plain := plainText(compact)
	if strings.Contains(plain, "✓✓✓") {
		t.Errorf("a strip of one glyph per step, the list of checks:\n%s", plain)
	}
	for _, want := range []string{"team-solve · running test-0 · 6/18 done", "6 done", "problem, outline", "running", "test-0 12s", "⏺ bash · 4s", "The check before the team: the direct attempt", "1.5k tokens", "baseline: baseline.md", "direct: direct.md", "failed", "context — exit status 1 — no prompt", "10 waiting", "ctrl+o shows every step (18)"} {
		if !strings.Contains(plain, want) {
			t.Errorf("compact misses %q:\n%s", want, plain)
		}
	}
	small := WorkflowRun{ID: "r2", Workflow: "release", State: "running", Total: 3, Tasks: tasks[:3]}
	if strings.Contains(plainText(workflowGraphMessage(small, 120, false).Content), "ctrl+o shows every step") {
		t.Error("a short graph is drawn in full")
	}
}

// The tab and cmux say Running while this window's run goes on after its
// turn: a team-mode turn answers at once and the team works for minutes (the
// maintainer, 2026-11-06: "in cmux i dont see running when runnning team mode").
func TestTheTabSaysRunningWhileTheTeamRuns(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	if m.working() {
		t.Fatal("an idle window is not working")
	}
	m.workflowRunning = "r1"
	m.workflowLast = map[string]WorkflowRun{"r1": {ID: "r1", State: "running"}}
	if !m.working() || !strings.HasPrefix(m.windowTitle(), spinFrames[0]) {
		t.Fatalf("a running run, no turn: working %v, title %q", m.working(), m.windowTitle())
	}
	if m.busy() {
		t.Fatal("busy stays the turn's")
	}
	m.workflowLast["r1"] = WorkflowRun{ID: "r1", State: "waiting"}
	if m.working() {
		t.Fatal("a run waiting at a gate is blocked, not working")
	}
	m.workflowLast["r1"] = WorkflowRun{ID: "r1", State: "running"}
	before := m
	m.workflowRunning = "" // the run ended
	next, cmd := before.Update(workflowTickMsg(time.Now()))
	_ = next
	if !before.working() || cmd == nil {
		t.Fatal("the run's end is a transition the window acts on")
	}
}

// A gate says "a approves" only once the run is waiting at it. Before, the
// step is one the run has not reached, and the line read as if a could
// approve it already (the maintainer, 2026-11-06: "why we always see ○
// approve  waiting · external — a approves").
func TestAGateAsksOnlyWhenTheRunIsAtIt(t *testing.T) {
	tasks := []WorkflowTask{
		{Name: "notes", State: "running"},
		{Name: "approve", Requires: []string{"notes"}, External: true, State: "waiting"},
		{Name: "commit", Requires: []string{"approve"}, State: "waiting"},
	}
	before := plainText(strings.Join(workflowGraphRows(WorkflowRun{State: "running", Tasks: tasks}, 100, false), "\n"))
	if strings.Contains(before, "a approves") || !strings.Contains(before, "approve  waiting · gate") {
		t.Errorf("a gate the run has not reached asks nobody:\n%s", before)
	}
	tasks[0].State = "done"
	at := plainText(strings.Join(workflowGraphRows(WorkflowRun{State: "waiting", Tasks: tasks}, 100, false), "\n"))
	if !strings.Contains(at, "⏸ approve  waiting · your gate — a approves") {
		t.Errorf("at the gate, the person's turn:\n%s", at)
	}
	// The compact view (more than 8 steps) names the gate under the waiting line.
	many := append([]WorkflowTask{}, tasks...)
	for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
		many = append(many, WorkflowTask{Name: n, State: "done"})
	}
	compact := plainText(workflowGraphMessage(WorkflowRun{ID: "r", Workflow: "release-notes", State: "waiting", Total: 9, Done: 7, Tasks: many}, 100, false).Content)
	if !strings.Contains(compact, "⏸ approve — your gate — a approves") {
		t.Errorf("the compact view names the gate:\n%s", compact)
	}
}

// a alone at the prompt approves this window's run at its gate, and the hint
// says so; with no run at a gate, a is a message like any other.
func TestAAloneApprovesTheWindowsGate(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	if _, send := m.hint(); send == "a" {
		t.Fatal("no run, no gate to approve")
	}
	if id, task := m.windowGate(); id != "" || task != "" {
		t.Fatalf("no run: %q %q", id, task)
	}
	m.workflowRunning = "r1"
	m.workflowLast = map[string]WorkflowRun{"r1": {ID: "r1", State: "waiting", Tasks: []WorkflowTask{
		{Name: "notes", State: "done"},
		{Name: "approve", Requires: []string{"notes"}, External: true, State: "waiting"},
		{Name: "commit", Requires: []string{"approve"}, State: "waiting"},
	}}}
	if id, task := m.windowGate(); id != "r1" || task != "approve" {
		t.Fatalf("the window's gate: %q %q", id, task)
	}
	if text, send := m.hint(); send != "a" || !strings.Contains(text, "a approves approve") {
		t.Fatalf("hint = %q, %q", text, send)
	}
}

// /agents fetches with the person's token (Ops.Agents), never the agent-side
// tool: that one sent the gateway's internal service token and every window
// got 401 (2026-11-06).
func TestAgentsListIsFetchedAsThePerson(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	if got := m.getAgentsList(); !strings.Contains(got, "aren't available") {
		t.Fatalf("no op: %q", got)
	}
	m.agents = func() (string, error) {
		return `{"agents":[{"id":"agent:coder","name":"coder","tools":["bash","edit_file","read_file","write_file","locate","cron"],"status":"online"},{"id":"agent:chief","name":"chief","tools":["bash"],"status":"online"},{"id":"agent:narrator","tools":[],"status":"offline"}]}`, nil
	}
	got := m.getAgentsList()
	for _, want := range []string{"**3 agents**", "- **coder** · 6 tools: bash, edit_file, read_file, write_file, … +2\n", "- **chief** · 1 tool: bash\n", "- **narrator** · no tools · offline\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "###") || strings.Contains(got, "agent:") {
		t.Errorf("headings and ids are the old form:\n%s", got)
	}
	m.agents = func() (string, error) { return "", fmt.Errorf("API error (401)") }
	if got := m.getAgentsList(); !strings.Contains(got, "Error loading agents: API error (401)") {
		t.Errorf("the error, in words: %q", got)
	}
}

// #128: a failed command step shows its exit status and the last lines it
// printed, not its script. The gateway sends the error as mario wrote it
// ("task <name>: <command>: exit status 1 — …") stripped of the command; the
// window draws the status line and the tail, in the graph and the compact view.
func TestAFailedCommandStepShowsItsStatusAndTail(t *testing.T) {
	err := "exit status 1 — no prompt: memdoor workflow run team-solve \"<what to do>\", or a task.md in this folder\nline two\nline three\nline four\nline five"
	tasks := []WorkflowTask{{Name: "problem", State: "failed", Error: err}}
	plain := plainText(strings.Join(workflowGraphRows(WorkflowRun{State: "failed", Tasks: tasks}, 120, false), "\n"))
	for _, want := range []string{"✗ problem  failed", "exit status 1 — no prompt: memdoor workflow run team-solve", "…", "line four", "line five"} {
		if !strings.Contains(plain, want) {
			t.Errorf("graph misses %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "line two") {
		t.Errorf("the graph shows the tail, not every line:\n%s", plain)
	}
	many := append([]WorkflowTask{}, tasks...)
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		many = append(many, WorkflowTask{Name: n, State: "waiting"})
	}
	compact := plainText(workflowGraphMessage(WorkflowRun{ID: "r", Workflow: "team-solve", State: "failed", Total: 9, Tasks: many}, 120, false).Content)
	if !strings.Contains(compact, "problem — exit status 1 — no prompt") || !strings.Contains(compact, "line five") {
		t.Errorf("compact view:\n%s", compact)
	}
}

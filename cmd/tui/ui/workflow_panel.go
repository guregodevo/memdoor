package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// THE WORKFLOW VIEW RENDERS THE DAG (the maintainer, 2026-10-02: "the workflow rendered
// should render a dag of tasks"). A run is drawn as its graph: tasks in the
// layers their dependencies impose, each with its state and what it requires,
// updated live from the gateway's workflow_event broadcasts. /workflow lists
// the project's workflows and runs; /w <name> [the task] starts one; enter
// on a run opens its graph; a approves an external task; s stops a run.

// WorkflowOps is how the panel reaches the gateway — injected by the CLI,
// declared here so the TUI imports nothing of the CLI (duck typing).
type WorkflowOps struct {
	List    func() ([]WorkflowRun, []WorkflowEntry, error)            // runs, and the workflows this project can run
	Run     func(name, partition, prompt string) (WorkflowRun, error) // partition "" = a fresh run; a scheme's word (a commit, a day, a version) otherwise. prompt: the task, for a workflow that reads one (team-solve)
	Status  func(runID string) (WorkflowRun, error)
	Stop    func(runID string) (WorkflowRun, error)
	Approve func(runID, task string) (WorkflowRun, error)
	// Changes asks for changes at a gate: task reruns with the comment, then
	// the steps after it, and the run comes back to the gate.
	Changes func(runID, task, comment string) (WorkflowRun, error)
	Resume  func(runID string) (WorkflowRun, error) // continue a failed or stopped run: what it finished is skipped
	Diff    func(runID string) (string, error)      // every line the run changed since it started, committed or not
}

// WorkflowRun is one run as the gateway reports it.
type WorkflowRun struct {
	ID       string
	Workflow string
	Dir      string
	State    string // running, waiting (at an external target), done, failed, stopped, stopping
	Started  time.Time
	Error    string
	Tasks    []WorkflowTask
	Done     int
	Total    int
	Review   string   // at a gate: the commits and files the run changed
	Results  []string // when done: the files the run produced
	// The steps' tokens and cost so far, summed (gateway meterRun).
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// WorkflowTask is one node.
type WorkflowTask struct {
	Name     string
	Agent    string
	Requires []string
	Target   string
	External bool
	State    string // waiting, running, done, failed, retrying, skipped
	Error    string
	Attempt  int
	Took     string
	Started  *time.Time // the clock ticks a running task's time from it between events
	Note     string     // the task's latest progress line (a tool it called), from events
	Receipt  string     // what proved a finished agent task: its turn's receipt line
	Outcome  string     `json:"outcome"` // a finished step's file and its verdict ("review.md: verdict: PASS")
	About    string     `json:"about"`   // what the step does, from its file's opening comment
	// Lines are what the step did, one per call (`✎ path`, `$ cmd → exit 0`),
	// oldest first, from the run's progress events; kept in the window only.
	Lines []stepLine `json:"-"`
	// What the step's model calls cost so far, live: the gateway sums the
	// ledger's rows for the step's session as they are written.
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	Calls        int     `json:"calls"`
}

// stepLine is one call of a step's turn as the run view shows it; key ties
// the call's start, beats and end to one line.
type stepLine struct{ key, text string }

const (
	// stepLinesShown is how many of a running step's lines the graph shows;
	// ctrl+o shows them all.
	stepLinesShown = 3
	// stepLinesKept is how many a step keeps for ctrl+o.
	stepLinesKept = 200
)

// addLine puts a line under the step: in place of the line with the same
// key (a call's beat or end), else after the others.
func (t *WorkflowTask) addLine(key, text string) {
	t.Note = text
	if key != "" {
		for i := range t.Lines {
			if t.Lines[i].key == key {
				t.Lines[i].text = text
				return
			}
		}
	}
	t.Lines = append(t.Lines, stepLine{key: key, text: text})
	if len(t.Lines) > stepLinesKept {
		t.Lines = t.Lines[len(t.Lines)-stepLinesKept:]
	}
}

// usageDetail is a step's or a run's tokens and cost as the row shows them
// ("12.3k tokens · $0.004"); "" before its first model call.
func usageDetail(in, out int64, cost float64) string {
	if in+out == 0 {
		return ""
	}
	d := formatTokens(int(in + out))
	if cost >= 0.0005 {
		d += fmt.Sprintf(" · $%.3f", cost)
	}
	return d
}

// workflowEventMsg is one transition from the gateway, with the run's whole
// state attached so the graph never drifts from the truth.
// workflowTickMsg redraws the running blocks once a second: a running task's
// time moves between events.
type workflowTickMsg time.Time

// WorkflowEntry is a workflow the project can run: its name, where it comes
// from (project or library) and what its README says.
type WorkflowEntry struct {
	Name        string
	Source      string
	Description string
}

type workflowEventMsg struct {
	runID  string
	task   string
	state  string // started, done, failed, retrying, note, progress, usage, question, answered
	text   string
	key    string // a progress line's call: its later lines replace it
	status *WorkflowRun
	// Set for state "question" (questionID, options; text is the question) and
	// "answered" (questionID only), from workflow_runs.go relayQuestion.
	questionID string
	options    []string
}

type workflowListMsg struct {
	runs  []WorkflowRun
	files []WorkflowEntry
	err   error
}

type workflowDiffMsg struct {
	runID string
	diff  string
	err   error
}

type workflowRunMsg struct {
	run WorkflowRun
	err error
}

const (
	workflowScreenList = iota
	workflowScreenRun
)

type workflowPanelState struct {
	screen  int
	runs    []WorkflowRun
	files   []WorkflowEntry
	index   int // in the list: files first, then runs
	run     *WorkflowRun
	loading string
	err     string
	confirm string // a stop waiting for its second s
}

// workflowCommand handles /w and /workflow: alone, the panel; <name> [the task]
// runs one; resume, stop, changes, diff and approve act on a run. "run" before
// the name still works; nobody found it (#130: "/workflow run is not
// intuitive … something easier like /w team-solve <prompt>").
func (m *Model) workflowCommand(args []string) tea.Cmd {
	if m.workflow.List == nil {
		m.note("Workflows aren't available in this build.")
		return nil
	}
	if len(args) == 0 {
		m.workflowPanel = &workflowPanelState{screen: workflowScreenList, loading: "Reading workflows …"}
		return m.workflowRefresh()
	}
	switch args[0] {
	case "run":
		if len(args) < 2 {
			m.note(workflowUsage)
			return nil
		}
		return m.workflowRunNamed(args[1], args[2:])
	case "resume", "continue":
		if len(args) < 2 {
			m.note("Which run? **/workflow resume <run-id>** — a failed or stopped run; what it finished is skipped.")
			return nil
		}
		op, id := m.workflow.Resume, args[1]
		return func() tea.Msg { r, err := op(id); return workflowRunMsg{run: r, err: err} }
	case "stop":
		if len(args) < 2 {
			m.note("Which run? **/workflow stop <run-id>**")
			return nil
		}
		op := m.workflow.Stop
		id := args[1]
		m.note("Stopping " + id + " …")
		return func() tea.Msg { r, err := op(id); return workflowRunMsg{run: r, err: err} }
	case "changes":
		// /workflow changes [run-id] <step> <comment …> — the run defaults to
		// the one waiting in this window (the review loop, 2026-10-03).
		rest := args[1:]
		id := m.workflowRunning
		if len(rest) > 0 && runIDPattern.MatchString(rest[0]) {
			id, rest = rest[0], rest[1:]
		}
		if id == "" && m.workflowPanel != nil && m.workflowPanel.run != nil {
			id = m.workflowPanel.run.ID
		}
		if id == "" || len(rest) < 2 || m.workflow.Changes == nil {
			m.note("**/workflow changes <step> <what to change>** — at a gate: the step reruns with your comment, then the steps after it, and the run comes back for review.")
			return nil
		}
		op, task, comment := m.workflow.Changes, rest[0], strings.Join(rest[1:], " ")
		m.note("↺ asking " + task + " for changes: " + comment)
		return func() tea.Msg { r, err := op(id, task, comment); return workflowRunMsg{run: r, err: err} }
	case "diff":
		// /workflow diff [run-id] — every line, in the conversation, where it scrolls.
		id := m.workflowRunning
		if len(args) > 1 {
			id = args[1]
		}
		if id == "" && m.workflowPanel != nil && m.workflowPanel.run != nil {
			id = m.workflowPanel.run.ID
		}
		if id == "" {
			m.note("Which run? **/workflow diff <run-id>** — every line a run changed, before you approve it.")
			return nil
		}
		return m.workflowDiff(id)
	case "approve":
		if len(args) < 3 {
			m.note("**/workflow approve <run-id> <task>** completes an external task.")
			return nil
		}
		op := m.workflow.Approve
		id, task := args[1], args[2]
		return func() tea.Msg { r, err := op(id, task); return workflowRunMsg{run: r, err: err} }
	}
	// Anything else is a workflow's name, and the words after it its task.
	return m.workflowRunNamed(args[0], args[1:])
}

// workflowUsage is what /w says when it is not told enough.
const workflowUsage = "**/w <workflow> <the task>** runs one (**/w team-solve <the task>**) · **/w** opens the panel · **/w stop <run-id>** · **/w approve <run-id> <task>**"

// workflowRunNamed starts a workflow with the words after its name as the
// task. team-solve with no task is asked for it instead: started without one
// it stops at its first step, "no prompt", and the window showed only the
// step's script (#129). A task.md in the project, or a partition (an earlier
// run that has its prompt), is a task.
func (m *Model) workflowRunNamed(name string, words []string) tea.Cmd {
	partition, prompt := workflowRunArgs(words)
	if name == teamSolveWorkflow && prompt == "" && partition == "" && !fileExists(filepath.Join(SessionDir(), "task.md")) {
		m.askTeamSolveTask()
		return nil
	}
	return m.workflowStart(name, partition, prompt)
}

// completesWorkflowName reports whether the line is "/w " and the start of a
// workflow's name, the space before the task not typed yet: the dropdown
// stays open and Tab completes the name.
func completesWorkflowName(line string) bool {
	rest, ok := strings.CutPrefix(line, "/w ")
	return ok && !strings.Contains(rest, " ")
}

// teamSolveWorkflow is the workflow that needs a task.
const teamSolveWorkflow = "team-solve"

// askTeamSolveTask puts "/w team-solve " on the line, ready for the task.
func (m *Model) askTeamSolveTask() {
	m.note("What should the team do? Type the task after **/w team-solve** and press enter.")
	m.input.SetValue("/w " + teamSolveWorkflow + " ")
	m.input.CursorEnd()
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func (m *Model) workflowDiff(id string) tea.Cmd {
	op := m.workflow.Diff
	if op == nil {
		m.note("This build can't show a run's diff.")
		return nil
	}
	return func() tea.Msg { d, err := op(id); return workflowDiffMsg{runID: id, diff: d, err: err} }
}

// renderGateDiff draws a run's diff for the conversation: what a line is
// (added, removed, a hunk, a file) is its hue; the context stays dim.
func renderGateDiff(runID, diff string, width int) string {
	add := lipgloss.NewStyle().Foreground(lipgloss.Color(colOK))
	del := lipgloss.NewStyle().Foreground(lipgloss.Color(colErr))
	hunk := lipgloss.NewStyle().Foreground(lipgloss.Color(colPlan))
	file := lipgloss.NewStyle().Foreground(lipgloss.Color(colText)).Bold(true)
	ctx := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	w := width - 4
	if w < 20 {
		w = 20
	}
	var b strings.Builder
	b.WriteString(file.Render("  Diff of run "+runID+" since it started") + "\n")
	for _, l := range strings.Split(strings.TrimRight(diff, "\n"), "\n") {
		st := ctx
		switch {
		case strings.HasPrefix(l, "diff --git"):
			st = file
		case strings.HasPrefix(l, "+++"), strings.HasPrefix(l, "---"), strings.HasPrefix(l, "index "):
			st = ctx
		case strings.HasPrefix(l, "+"):
			st = add
		case strings.HasPrefix(l, "-"):
			st = del
		case strings.HasPrefix(l, "@@"):
			st = hunk
		}
		lead := "  "
		if strings.HasPrefix(l, " ") { // a context line keeps its column outside the style, which trims it
			lead, l = "   ", l[1:]
		}
		b.WriteString(lead + st.Render(cutKeepingSpace(strings.ReplaceAll(l, "\t", "    "), w)) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) workflowStart(name, partition, prompt string) tea.Cmd {
	op := m.workflow.Run
	return func() tea.Msg { r, err := op(name, partition, prompt); return workflowRunMsg{run: r, err: err} }
}

// workflowRunArgs reads what follows the workflow's name the way `memdoor
// workflow run <name> [prompt]` does: the words are the prompt, the task of a
// workflow that reads one (team-solve); --partition <p> names the partition.
func workflowRunArgs(words []string) (partition, prompt string) {
	var rest []string
	for i := 0; i < len(words); i++ {
		if words[i] == "--partition" && i+1 < len(words) {
			partition = words[i+1]
			i++
			continue
		}
		rest = append(rest, words[i])
	}
	prompt = strings.TrimSpace(strings.Join(rest, " "))
	if len(prompt) >= 2 && (prompt[0] == '"' && prompt[len(prompt)-1] == '"' || prompt[0] == '\'' && prompt[len(prompt)-1] == '\'') {
		prompt = prompt[1 : len(prompt)-1]
	}
	return partition, prompt
}

func (m *Model) workflowRefresh() tea.Cmd {
	op := m.workflow.List
	return func() tea.Msg { runs, files, err := op(); return workflowListMsg{runs: runs, files: files, err: err} }
}

// updateWorkflowPanel owns the panel's messages and, while it is open, the
// keyboard. Events from the gateway are handled whether it is open or not.
func (m *Model) updateWorkflowPanel(msg tea.Msg) (bool, tea.Cmd) {
	p := m.workflowPanel
	switch t := msg.(type) {
	case workflowEventMsg:
		switch t.state {
		case "question":
			m.askFromStep(t)
			return true, nil
		case "answered":
			m.questionAnswered(t.questionID)
			return true, nil
		}
		var cmd tea.Cmd
		if t.state == "note" || t.state == "failed" || t.state == "done" && t.task == "" {
			m.note(t.text)
		}
		if t.state == "progress" && t.status == nil {
			m.noteTaskLine(t.runID, t.task, t.key, t.text)
			return true, nil
		}
		if t.status != nil {
			if m.workflowLast == nil {
				m.workflowLast = map[string]WorkflowRun{}
			}
			// A finished run does not go back to running: a defence against
			// events arriving out of order (the run's note and its steps'
			// events are sent from different goroutines). A resume is a new
			// run id. (The stale "running" header seen in review was the
			// redraw below, not this.)
			if last, ok := m.workflowLast[t.runID]; ok && last.ID == t.status.ID && runFinished(last.State) && !runFinished(t.status.State) {
				t.status = nil
			}
		}
		if t.status != nil {
			if last, ok := m.workflowLast[t.runID]; ok { // keep the progress lines across snapshots
				for i := range t.status.Tasks {
					for _, lt := range last.Tasks {
						if lt.Name != t.status.Tasks[i].Name {
							continue
						}
						// A finished step keeps its lines for ctrl+o; a step
						// that starts again (a retry) adds to them.
						t.status.Tasks[i].Lines = lt.Lines
						if t.status.Tasks[i].State == "running" {
							t.status.Tasks[i].Note = lt.Note
						}
					}
				}
			}
			m.workflowLast[t.runID] = *t.status
			m.upsertWorkflowGraph(*t.status)
			switch t.status.State {
			case "running", "waiting":
				ticking := m.workflowRunning != ""
				m.workflowRunning = t.status.ID
				if !ticking && t.status.State == "running" {
					cmd = m.workflowTick() // the clock starts with the first running run
				}
			default:
				if m.workflowRunning == t.status.ID {
					m.workflowRunning = "" // the clock stops with the run
					m.workflowRunEnded(*t.status)
				}
				if p != nil {
					p.err = "" // a refusal like "already running" is stale once a run ends
				}
			}
			if p != nil && p.run != nil && p.run.ID == t.status.ID {
				p.run = t.status
			}
			if p != nil {
				for i := range p.runs {
					if p.runs[i].ID == t.status.ID {
						p.runs[i] = *t.status
					}
				}
			}
		}
		// Redraw whether or not the panel is open: the graph block lives in
		// the conversation. Only with the panel open did the final "done"
		// reach the screen — the note before it had redrawn, the block after
		// it had not (review, 2026-10-04).
		if m.viewport.Height > 0 {
			m.refreshFollow()
		}
		return true, cmd
	case workflowTickMsg:
		if m.workflowRunning == "" {
			return true, nil
		}
		for _, last := range m.workflowLast {
			if last.State == "running" {
				m.upsertWorkflowGraph(last)
			}
		}
		if m.viewport.Height > 0 {
			m.refreshFollow()
		}
		return true, m.workflowTick()
	case workflowListMsg:
		if p == nil {
			return true, nil
		}
		p.loading = ""
		if t.err != nil {
			p.err = t.err.Error()
			return true, nil
		}
		p.runs, p.files, p.err = t.runs, t.files, ""
		if p.index >= len(p.files)+len(p.runs) {
			p.index = 0
		}
		return true, nil
	case workflowDiffMsg:
		switch {
		case t.err != nil:
			m.note("✗ " + t.err.Error())
		case strings.TrimSpace(t.diff) == "":
			m.note("No changes in " + t.runID + " since it started.")
		default:
			m.messages = append(m.messages, Message{Role: "workflow", Content: renderGateDiff(t.runID, t.diff, m.width), Timestamp: time.Now()})
			if m.viewport.Height > 0 {
				m.refreshFollow()
			}
		}
		return true, nil
	case workflowRunMsg:
		if t.err != nil {
			if p != nil {
				p.loading, p.err = "", t.err.Error()
			} else {
				m.note("✗ " + t.err.Error())
			}
			return true, nil
		}
		if p != nil {
			p.loading, p.err = "", ""
			r := t.run
			p.run, p.screen = &r, workflowScreenRun
			return true, m.workflowRefresh()
		}
		return true, nil
	case tea.KeyMsg:
		if p == nil {
			return false, nil
		}
		return true, m.workflowKey(t)
	}
	return false, nil
}

func (m *Model) workflowKey(k tea.KeyMsg) tea.Cmd {
	p := m.workflowPanel
	if k.Type == tea.KeyCtrlC {
		return nil
	}
	if !(k.Type == tea.KeyRunes && string(k.Runes) == "s") {
		p.confirm = ""
	}
	p.err = "" // an error answers the last key; the next key replaces it
	switch p.screen {
	case workflowScreenRun:
		switch k.Type {
		case tea.KeyEsc:
			p.screen, p.run, p.err = workflowScreenList, nil, ""
			return m.workflowRefresh()
		case tea.KeyRunes:
			switch string(k.Runes) {
			case "s":
				if p.run == nil || p.run.State != "running" && p.run.State != "waiting" {
					return nil
				}
				if p.confirm == p.run.ID {
					p.confirm = ""
					op, id := m.workflow.Stop, p.run.ID
					p.loading = "Stopping " + id + " …"
					return func() tea.Msg { r, err := op(id); return workflowRunMsg{run: r, err: err} }
				}
				p.confirm = p.run.ID
			case "c":
				if p.run == nil || p.run.State != "failed" && p.run.State != "stopped" {
					return nil
				}
				op, id := m.workflow.Resume, p.run.ID
				p.loading = "Resuming " + id + " …"
				return func() tea.Msg { r, err := op(id); return workflowRunMsg{run: r, err: err} }
			case "a":
				if p.run == nil {
					return nil
				}
				for _, t := range p.run.Tasks {
					if t.External && t.State != "done" && t.State != "skipped" {
						op, id, task := m.workflow.Approve, p.run.ID, t.Name
						p.loading = "Approving " + task + " …"
						return func() tea.Msg { r, err := op(id, task); return workflowRunMsg{run: r, err: err} }
					}
				}
				p.err = "nothing is waiting for approval"
			case "d":
				if p.run == nil {
					return nil
				}
				// The conversation scrolls and the panel doesn't: the diff goes
				// there, and the panel closes so it can be read (/workflow reopens).
				id := p.run.ID
				m.workflowPanel = nil
				return m.workflowDiff(id)
			case "r":
				if p.run != nil {
					op, id := m.workflow.Status, p.run.ID
					return func() tea.Msg { r, err := op(id); return workflowRunMsg{run: r, err: err} }
				}
			}
		}
		return nil
	}
	// the list
	n := len(p.files) + len(p.runs)
	switch k.Type {
	case tea.KeyEsc:
		m.workflowPanel = nil
	case tea.KeyUp:
		if p.index > 0 {
			p.index--
		}
	case tea.KeyDown:
		if p.index < n-1 {
			p.index++
		}
	case tea.KeyEnter:
		if p.index < len(p.files) {
			if name := p.files[p.index].Name; name == teamSolveWorkflow && !fileExists(filepath.Join(SessionDir(), "task.md")) {
				m.workflowPanel = nil // the task is typed on the line
				m.askTeamSolveTask()
				return nil
			}
			p.loading = "Starting " + p.files[p.index].Name + " …"
			return m.workflowStart(p.files[p.index].Name, "", "")
		}
		if i := p.index - len(p.files); i >= 0 && i < len(p.runs) {
			r := p.runs[i]
			p.run, p.screen = &r, workflowScreenRun
		}
	case tea.KeyRunes:
		if string(k.Runes) == "r" {
			p.loading = "Reading workflows …"
			return m.workflowRefresh()
		}
	}
	return nil
}

// ---- rendering: the DAG --------------------------------------------------------

// workflowLayers places every task in the layer its dependencies impose:
// a task's layer is one past its deepest requirement. Tasks in a layer are
// in file order, so the picture is stable from one event to the next.
func workflowLayers(tasks []WorkflowTask) [][]WorkflowTask {
	byName := map[string]WorkflowTask{}
	order := map[string]int{}
	for i, t := range tasks {
		byName[t.Name] = t
		order[t.Name] = i
	}
	depth := map[string]int{}
	var depthOf func(name string, seen map[string]bool) int
	depthOf = func(name string, seen map[string]bool) int {
		if d, ok := depth[name]; ok {
			return d
		}
		if seen[name] {
			return 0
		}
		seen[name] = true
		d := 0
		for _, r := range byName[name].Requires {
			if rd := depthOf(r, seen) + 1; rd > d {
				d = rd
			}
		}
		depth[name] = d
		return d
	}
	maxDepth := 0
	for _, t := range tasks {
		if d := depthOf(t.Name, map[string]bool{}); d > maxDepth {
			maxDepth = d
		}
	}
	layers := make([][]WorkflowTask, maxDepth+1)
	for _, t := range tasks {
		layers[depth[t.Name]] = append(layers[depth[t.Name]], t)
	}
	for i := range layers {
		sort.Slice(layers[i], func(a, b int) bool { return order[layers[i][a].Name] < order[layers[i][b].Name] })
	}
	return layers
}

func workflowGlyph(state string) (string, string) {
	switch state {
	case "running":
		return "▶", colRun
	case "done":
		return "✓", colOK
	case "skipped":
		return "✓", colDim
	case "failed":
		return "✗", colErr
	case "stopped":
		// Asked for, not an error: no error hue, and not grey either — a
		// stopped step did not finish and should be seen (#133: "stopped
		// should have a color maybe"). The warning hue; ■ tells it from ▶.
		return "■", colWarn
	case "retrying":
		return "↻", colWarn
	}
	return "○", colDim
}

// workflowGraphRows draws the DAG in dependency layers: one row per task,
// a │ between layers, the state and its time, ← what it requires. The panel
// and the conversation both draw from it.
func workflowGraphRows(r WorkflowRun, width int, expanded bool) []string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	bad := lipgloss.NewStyle().Foreground(lipgloss.Color(colErr))
	var rows []string
	w := 0
	for _, t := range r.Tasks {
		if len(t.Name) > w {
			w = len(t.Name)
		}
	}
	for li, layer := range workflowLayers(r.Tasks) {
		if li > 0 {
			rows = append(rows, dim.Render("  │"))
		}
		for _, t := range layer {
			g, c := workflowGlyph(t.State)
			// Hue says what the state is, on the glyph and its word; luminance
			// says how much the step matters now: a step that ran, runs or
			// failed is read, one waiting or skipped is not (ADR 0018, #133).
			hue := lipgloss.NewStyle().Foreground(lipgloss.Color(c))
			glyph := hue.Render(g)
			nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colText))
			if t.State == "" || t.State == "waiting" || t.State == "skipped" {
				nameStyle = dim
			}
			state := t.State
			if state == "" {
				state = "waiting"
			}
			detail := ""
			switch {
			case t.State == "running" && t.Started != nil:
				detail += " " + time.Since(*t.Started).Round(time.Second).String() // ticks between events
			case t.Took != "" && (t.State == "done" || t.State == "failed" || t.State == "running"):
				detail += " " + t.Took
			}
			if u := usageDetail(t.InputTokens, t.OutputTokens, t.CostUSD); u != "" {
				detail += " · " + u
			}
			if t.Attempt > 1 {
				detail += fmt.Sprintf(" (try %d)", t.Attempt)
			}
			if t.External && t.State != "done" && t.State != "skipped" {
				detail += " · external — a approves"
			}
			stateWord := hue.Render(state)
			if t.State == "" || t.State == "waiting" || t.State == "skipped" {
				stateWord = dim.Render(state) // nothing to look at yet, or nothing done
			}
			line := fmt.Sprintf("%s %s  %s%s", glyph, nameStyle.Render(fmt.Sprintf("%-*s", w, t.Name)), stateWord, dim.Render(detail))
			if len(t.Requires) > 0 {
				line += dim.Render("  ← " + strings.Join(t.Requires, ", "))
			}
			rows = append(rows, "  "+line)
			for _, l := range stepLinesOf(t, expanded) {
				rows = append(rows, "      "+dim.Render(truncateEnd(l, width-10)))
			}
			if t.State != "running" {
				for _, l := range []string{t.Receipt, t.Outcome} {
					if l != "" {
						rows = append(rows, "      "+dim.Render(truncateEnd(l, width-10)))
					}
				}
			}
			if t.Error != "" && t.State != "done" {
				rows = append(rows, "    "+bad.Render(truncateEnd(t.Error, width-8)))
			}
		}
	}
	return rows
}

// stepLinesOf is what the graph draws under a step from its lines: a running
// step's last few, every line of any step with ctrl+o, none of a finished
// step otherwise (its receipt and outcome say how it ended).
func stepLinesOf(t WorkflowTask, expanded bool) []string {
	if len(t.Lines) == 0 && t.Note != "" && t.State == "running" {
		return []string{t.Note} // a question, from an older gateway's event
	}
	if !expanded && t.State != "running" {
		return nil
	}
	lines := t.Lines
	var out []string
	if !expanded && len(lines) > stepLinesShown {
		out = append(out, fmt.Sprintf("… %d more (ctrl+o)", len(lines)-stepLinesShown))
		lines = lines[len(lines)-stepLinesShown:]
	}
	for _, l := range lines {
		out = append(out, l.text)
	}
	return out
}

// workflowCompactAfter is how many steps a graph draws in full in the
// conversation; past it, the steps are grouped by state (#134: "listing all
// workflow steps can not fit the full window"). team-solve has 18 steps, 36
// lines with the layers' separators. ctrl+o draws every step; the panel
// always does.
const workflowCompactAfter = 8

// workflowCompactRows is a long graph in a few lines: a strip with every
// step's glyph in run order, then one line per state that has steps — what
// is done, what runs (with its time and its last lines), what failed (with
// its error), what waits — and how to see every step.
func workflowCompactRows(r WorkflowRun, width int) []string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	text := lipgloss.NewStyle().Foreground(lipgloss.Color(colText))
	bad := lipgloss.NewStyle().Foreground(lipgloss.Color(colErr))
	var strip strings.Builder
	groups := map[string][]WorkflowTask{}
	for _, layer := range workflowLayers(r.Tasks) {
		for _, t := range layer {
			g, c := workflowGlyph(t.State)
			strip.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(g))
			st := t.State
			if st == "" {
				st = "waiting"
			}
			groups[st] = append(groups[st], t)
		}
	}
	rows := []string{"  " + strip.String()}
	room := width - 22
	if room < 20 {
		room = 20
	}
	names := func(ts []WorkflowTask) string {
		var out []string
		used := 0
		for i, t := range ts {
			if used+len(t.Name)+2 > room && i > 0 {
				return strings.Join(out, ", ") + fmt.Sprintf(", … +%d", len(ts)-i)
			}
			out = append(out, t.Name)
			used += len(t.Name) + 2
		}
		return strings.Join(out, ", ")
	}
	label := func(state string, n int) string {
		g, c := workflowGlyph(state)
		word := state
		if n > 1 || state == "done" || state == "waiting" || state == "skipped" {
			word = fmt.Sprintf("%d %s", n, state)
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(g+" "+fmt.Sprintf("%-11s", word)) + "  "
	}
	for _, st := range []string{"done", "skipped", "running", "retrying", "failed", "stopped", "waiting"} {
		ts := groups[st]
		if len(ts) == 0 {
			continue
		}
		switch st {
		case "running", "retrying":
			// Each running step on its own line, with what it does and what it
			// has spent so far: a step that is one model call has no lines of
			// its own, and "▶ plan running 56s" alone read as stuck (the
			// maintainer, 2026-11-06: "it says running but users dont see
			// anything more").
			for i, t := range ts {
				lead := strings.Repeat(" ", 15)
				if i == 0 {
					lead = label(st, len(ts))
				}
				p := t.Name
				if t.Started != nil {
					p += " " + time.Since(*t.Started).Round(time.Second).String()
				}
				line := text.Render(p)
				if u := usageDetail(t.InputTokens, t.OutputTokens, t.CostUSD); u != "" {
					line += dim.Render(" · " + u)
				}
				rows = append(rows, "  "+lead+line)
				if t.About != "" {
					rows = append(rows, "      "+dim.Render(truncateEnd(t.About, width-10)))
				}
				for _, l := range stepLinesOf(t, false) {
					rows = append(rows, "      "+dim.Render(truncateEnd(l, width-10)))
				}
			}
		case "failed":
			rows = append(rows, "  "+label(st, len(ts))+text.Render(names(ts)))
			for _, t := range ts {
				if t.Error != "" {
					rows = append(rows, "    "+bad.Render(truncateEnd(t.Name+" — "+t.Error, width-8)))
				}
			}
		case "done":
			rows = append(rows, "  "+label(st, len(ts))+text.Render(names(ts)))
			// What the last two finished steps produced, so the run reads as
			// moving between them ("review: review.md: verdict: PASS").
			var said []string
			for _, t := range ts {
				for _, l := range []string{t.Outcome, t.Receipt} {
					if l != "" {
						said = append(said, t.Name+": "+l)
						break
					}
				}
			}
			if len(said) > 2 {
				said = said[len(said)-2:]
			}
			for _, l := range said {
				rows = append(rows, "      "+dim.Render(truncateEnd(l, width-10)))
			}
		case "stopped":
			rows = append(rows, "  "+label(st, len(ts))+text.Render(names(ts)))
		default: // waiting, skipped: nothing to read yet
			rows = append(rows, "  "+label(st, len(ts))+dim.Render(names(ts)))
		}
	}
	rows = append(rows, "  "+dim.Render(fmt.Sprintf("ctrl+o shows every step (%d)", len(r.Tasks))))
	return rows
}

// workflowGraphMessage is the DAG as one block in the conversation, redrawn
// in place on every event (the maintainer, 2026-10-02: "a tool to run workflow that
// we can see in real time the dag running"). Keyed by the run's id.
func workflowGraphMessage(r WorkflowRun, width int, expanded bool) Message {
	head := fmt.Sprintf("%s · %s · %d/%d done", r.Workflow, r.State, r.Done, r.Total)
	if u := usageDetail(r.InputTokens, r.OutputTokens, r.CostUSD); u != "" {
		head += " · " + u
	}
	if r.Error != "" {
		head += " — " + r.Error
	}
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	// The run's state in its hue, the rest of the line grey (#133).
	_, hc := workflowGlyph(r.State)
	if r.State == "stopping" {
		hc = colWarn
	}
	pre, post, _ := strings.Cut(head, " · "+r.State)
	headLine := dim.Render(pre+" · ") + lipgloss.NewStyle().Foreground(lipgloss.Color(hc)).Render(r.State) + dim.Render(post)
	rows := workflowGraphRows(r, width, expanded)
	if !expanded && len(r.Tasks) > workflowCompactAfter {
		rows = workflowCompactRows(r, width)
	}
	body := "  " + headLine + "\n" + strings.Join(rows, "\n")
	// A running graph is still changing: like a streaming reply it stays in
	// the live view, never frozen into scrollback, or its later redraws land
	// on a copy nobody sees (2026-10-04: a finished run's header still read
	// "running" in the scrollback).
	live := r.State == "running" || r.State == "stopping"
	return Message{Role: "workflow", Content: body, ToolID: "workflow:" + r.ID, Timestamp: time.Now(), Streaming: live}
}

func (m Model) renderWorkflowPanel() string {
	p := m.workflowPanel
	if p == nil {
		return ""
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colText)).PaddingLeft(2)
	norm := lipgloss.NewStyle().Foreground(lipgloss.Color(colText))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color(colSelFG)).Background(lipgloss.Color(colSelBG)).Bold(true)
	bad := lipgloss.NewStyle().Foreground(lipgloss.Color(colErr))
	keys := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).PaddingLeft(2)
	box := func(body string) string {
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(colRule)).
			Padding(0, 1).Width(boxWidth(m.width)).Render(body)
	}
	errLine := ""
	if p.err != "" {
		errLine = "\n" + bad.Render("✗ "+p.err)
	}
	if p.loading != "" {
		return title.Render("Workflows") + "\n" + box(dim.Render(p.loading)+errLine) + "\n" + keys.Render("esc close")
	}

	var b strings.Builder
	if p.screen == workflowScreenRun && p.run != nil {
		r := p.run
		head := fmt.Sprintf("%s · run %s · %s · %d/%d done", r.Workflow, r.ID, r.State, r.Done, r.Total)
		if u := usageDetail(r.InputTokens, r.OutputTokens, r.CostUSD); u != "" {
			head += " · " + u
		}
		if r.Error != "" {
			head += " — " + r.Error
		}
		b.WriteString(title.Render(head) + "\n")
		shown := *r
		if last, ok := m.workflowLast[r.ID]; ok && last.ID == r.ID {
			shown.Tasks = last.Tasks // the lines live in the window's copy
		}
		b.WriteString(box(strings.Join(workflowGraphRows(shown, boxWidth(m.width), m.expandTools), "\n")+errLine) + "\n")
		if r.State == "done" && len(r.Results) > 0 {
			b.WriteString(norm.Render("Results:") + "\n")
			for _, f := range r.Results {
				b.WriteString(norm.Render("  "+truncateEnd(f, boxWidth(m.width)-2)) + "\n")
			}
		}
		if r.State == "waiting" && r.Review != "" {
			// What to look at before answering the gate (the review loop).
			b.WriteString(dim.Render("To review before approving:") + "\n")
			for _, l := range strings.Split(r.Review, "\n") {
				b.WriteString(dim.Render("  "+truncateEnd(l, boxWidth(m.width)-2)) + "\n")
			}
		}
		hint := "esc back · r refresh"
		switch r.State {
		case "waiting":
			hint = "a approve · d full diff · /workflow changes <step> <comment> sends it back · s stop (twice) · " + hint
		case "running":
			hint = "s stop (twice) · " + hint
		case "failed", "stopped":
			hint = "c continue (what it finished is skipped) · " + hint
		}
		if p.confirm != "" {
			hint = "press s again to stop " + p.confirm + " · " + hint
		}
		b.WriteString(keys.Render(hint))
		return b.String()
	}

	b.WriteString(title.Render("Workflows — a DAG of agent tasks, each done when its target exists") + "\n")
	var rows []string
	if len(p.files) == 0 && len(p.runs) == 0 {
		rows = append(rows, norm.Render("No workflows here."), dim.Render("Add .memdoor/workflows/<name>.yaml to this project: tasks, what each requires, and the file or command that proves it."))
	}
	// One name column, as wide as the longest name (capped): a fixed 16 let
	// "article-to-workflow" push its row past the box, which wrapped it.
	nameW := 14
	for _, f := range p.files {
		nameW = max(nameW, len([]rune(f.Name)))
	}
	for _, r := range p.runs {
		nameW = max(nameW, len([]rune(r.Workflow)))
	}
	nameW = min(nameW, 28)
	i := 0
	for _, f := range p.files {
		about := f.Description
		if about == "" {
			about = "enter runs it"
		}
		if f.Source == "library" {
			about = "library · " + about
		}
		row := fmt.Sprintf("▷ %-*s  %s", nameW, truncateEnd(f.Name, nameW), dim.Render(truncateEnd(about, boxWidth(m.width)-nameW-10)))
		if i == p.index {
			rows = append(rows, sel.Render("→ "+row))
		} else {
			rows = append(rows, "  "+norm.Render(row))
		}
		i++
	}
	if len(p.files) > 0 && len(p.runs) > 0 {
		rows = append(rows, "")
	}
	for _, r := range p.runs {
		g, c := workflowGlyph(map[string]string{"running": "running", "waiting": "retrying", "done": "done", "failed": "failed", "stopped": "stopped", "stopping": "retrying"}[r.State])
		glyph := lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(g)
		row := fmt.Sprintf("%s %-*s  %-8s  %d/%d  %s", glyph, nameW, truncateEnd(r.Workflow, nameW), r.State, r.Done, r.Total, dim.Render(truncateEnd(r.ID+"  enter opens the graph", boxWidth(m.width)-nameW-24)))
		if i == p.index {
			rows = append(rows, sel.Render("→ "+row))
		} else {
			rows = append(rows, "  "+row)
		}
		i++
	}
	b.WriteString(box(strings.Join(rows, "\n")+errLine) + "\n")
	b.WriteString(keys.Render("↑↓ move · enter run or open · r refresh · esc close"))
	return b.String()
}

// upsertWorkflowGraph replaces the run's block in the conversation, or adds it.
func (m *Model) upsertWorkflowGraph(r WorkflowRun) {
	msg := workflowGraphMessage(r, boxWidth(m.width), m.expandTools)
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].ToolID == msg.ToolID && m.messages[i].Role == "workflow" {
			if i < m.printedThrough {
				break // already printed and frozen (a run that waited at a gate): draw it anew below
			}
			msg.Timestamp = m.messages[i].Timestamp
			m.messages[i] = msg
			return
		}
	}
	m.messages = append(m.messages, msg)
}

// redrawWorkflowGraphs draws the runs still on screen again (ctrl+o): one
// already printed into the scrollback is left as it was.
func (m *Model) redrawWorkflowGraphs() {
	for i := m.printedThrough; i < len(m.messages); i++ {
		if m.messages[i].Role != "workflow" {
			continue
		}
		id, ok := strings.CutPrefix(m.messages[i].ToolID, "workflow:")
		if r, found := m.workflowLast[id]; ok && found {
			msg := workflowGraphMessage(r, boxWidth(m.width), m.expandTools)
			msg.Timestamp = m.messages[i].Timestamp
			m.messages[i] = msg
		}
	}
}

func (m Model) workflowTick() tea.Cmd {
	return tick(time.Second, func(t time.Time) tea.Msg { return workflowTickMsg(t) })
}

// runIDPattern is a run id as the gateway names them: "<workflow>-YYYYMMDD-HHMMSS".
var runIDPattern = regexp.MustCompile(`-\d{8}-\d{6}(-\d+)?$`)

// cutKeepingSpace shortens a code line to n runes without touching its
// whitespace: indentation is meaning in a diff (truncateEnd collapses runs of
// spaces, fine for a label; Python came out flat at a gate, 2026-10-04).
func cutKeepingSpace(s string, n int) string {
	r := []rune(s)
	if len(r) <= n || n < 2 {
		return s
	}
	return string(r[:n-1]) + "…"
}

// runFinished: a state a run does not leave on its own.
func runFinished(state string) bool {
	return state == "done" || state == "failed" || state == "stopped"
}

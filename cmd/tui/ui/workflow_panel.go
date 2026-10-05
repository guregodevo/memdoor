package ui

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// THE WORKFLOW VIEW RENDERS THE DAG (Greg, 2026-10-02: "the workflow rendered
// should render a dag of tasks"). A run is drawn as its graph: tasks in the
// layers their dependencies impose, each with its state and what it requires,
// updated live from the gateway's workflow_event broadcasts. /workflow lists
// the project's workflows and runs; /workflow run <name> starts one; enter
// on a run opens its graph; a approves an external task; s stops a run.

// WorkflowOps is how the panel reaches the gateway — injected by the CLI,
// declared here so the TUI imports nothing of the CLI (duck typing).
type WorkflowOps struct {
	List    func() ([]WorkflowRun, []WorkflowEntry, error)    // runs, and the workflows this project can run
	Run     func(name, partition string) (WorkflowRun, error) // partition "" = a fresh run; a scheme's word (a commit, a day, a version) otherwise
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
	state  string // started, done, failed, retrying, note
	text   string
	status *WorkflowRun
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

// workflowCommand handles /workflow [run <name> | approve <task> | stop <run>].
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
			m.note("Which workflow? **/workflow run <name>** [partition] — a directory in .memdoor/workflows/ of this project.")
			return nil
		}
		partition := ""
		if len(args) > 2 {
			partition = args[2]
		}
		return m.workflowStart(args[1], partition)
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
	m.note("**/workflow** opens the panel · **/workflow run <name>** · **/workflow stop <run-id>** · **/workflow approve <run-id> <task>**")
	return nil
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

func (m *Model) workflowStart(name, partition string) tea.Cmd {
	op := m.workflow.Run
	return func() tea.Msg { r, err := op(name, partition); return workflowRunMsg{run: r, err: err} }
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
		var cmd tea.Cmd
		if t.state == "note" || t.state == "failed" || t.state == "done" && t.task == "" {
			m.note(t.text)
		}
		if t.state == "progress" && t.status == nil {
			// a tool the task called: shown under the task, nothing else redrawn
			if last, ok := m.workflowLast[t.runID]; ok {
				for i := range last.Tasks {
					if last.Tasks[i].Name == t.task {
						last.Tasks[i].Note = t.text
					}
				}
				m.workflowLast[t.runID] = last
				m.upsertWorkflowGraph(last)
				if m.viewport.Height > 0 {
					m.refreshFollow()
				}
			}
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
						if lt.Name == t.status.Tasks[i].Name && t.status.Tasks[i].State == "running" {
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
			p.loading = "Starting " + p.files[p.index].Name + " …"
			return m.workflowStart(p.files[p.index].Name, "")
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
		return "■", colDim // asked for, not an error: no error hue
	case "retrying":
		return "↻", colWarn
	}
	return "○", colDim
}

// workflowGraphRows draws the DAG in dependency layers: one row per task,
// a │ between layers, the state and its time, ← what it requires. The panel
// and the conversation both draw from it.
func workflowGraphRows(r WorkflowRun, width int) []string {
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
			glyph := lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(g)
			detail := t.State
			switch {
			case t.State == "running" && t.Started != nil:
				detail += " " + time.Since(*t.Started).Round(time.Second).String() // ticks between events
			case t.Took != "" && (t.State == "done" || t.State == "failed" || t.State == "running"):
				detail += " " + t.Took
			}
			if t.Attempt > 1 {
				detail += fmt.Sprintf(" (try %d)", t.Attempt)
			}
			if t.External && t.State != "done" && t.State != "skipped" {
				detail += " · external — a approves"
			}
			line := fmt.Sprintf("%s %-*s  %s", glyph, w, t.Name, dim.Render(detail))
			if len(t.Requires) > 0 {
				line += dim.Render("  ← " + strings.Join(t.Requires, ", "))
			}
			rows = append(rows, "  "+line)
			if t.Note != "" && t.State == "running" {
				rows = append(rows, "      "+dim.Render(truncateEnd(t.Note, width-10)))
			}
			if t.Receipt != "" && t.State != "running" {
				rows = append(rows, "      "+dim.Render(truncateEnd(t.Receipt, width-10)))
			}
			if t.Error != "" && t.State != "done" {
				rows = append(rows, "    "+bad.Render(truncateEnd(t.Error, width-8)))
			}
		}
	}
	return rows
}

// workflowGraphMessage is the DAG as one block in the conversation, redrawn
// in place on every event (Greg, 2026-10-02: "a tool to run workflow that
// we can see in real time the dag running"). Keyed by the run's id.
func workflowGraphMessage(r WorkflowRun, width int) Message {
	head := fmt.Sprintf("%s · %s · %d/%d done", r.Workflow, r.State, r.Done, r.Total)
	if r.Error != "" {
		head += " — " + r.Error
	}
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	body := "  " + dim.Render(head) + "\n" + strings.Join(workflowGraphRows(r, width), "\n")
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
		if r.Error != "" {
			head += " — " + r.Error
		}
		b.WriteString(title.Render(head) + "\n")
		b.WriteString(box(strings.Join(workflowGraphRows(*r, boxWidth(m.width)), "\n")+errLine) + "\n")
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
	msg := workflowGraphMessage(r, boxWidth(m.width))
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

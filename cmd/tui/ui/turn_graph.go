package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The turn as a graph (Greg, 2026-10-11, "the turn drawn as a graph"):
// ctrl+g (or /graph) draws what this turn did as the DAG it is, in the
// workflow panel's own shape — the reads and searches, the edits that
// depended on them, the runs and checks that depended on the edits — with
// the failing check in red and the running step live. Up/down move, enter
// opens a file step in the files panel, esc or ctrl+g closes. The question
// a graph answers that the transcript does not: which change was the last
// one checked, and what the check that failed depended on.

type turnNode struct {
	task  WorkflowTask
	path  string // the file the step read or changed, for enter
	index int    // the frame's position in m.messages
}

type turnGraphState struct {
	nodes  []turnNode
	layers [][]int // node indexes per dependency layer, drawn in order
	rows   []int   // the node drawn on each row (−1 for a separator)
	index  int     // the selected node
	head   string
}

const turnGraphRows = 14

const commandFailedMark = "Command FAILED"

// turnGraphCommand is "/graph".
func (m *Model) turnGraphCommand() tea.Cmd {
	m.openTurnGraph()
	return nil
}

func (m *Model) openTurnGraph() {
	g := buildTurnGraph(m.messages, m.width)
	if len(g.nodes) == 0 {
		m.note("Nothing to draw yet: the graph is this turn's reads, edits and runs.")
		return
	}
	g.index = len(g.nodes) - 1
	m.turnGraph = g
	m.filesPanel = nil
}

// buildTurnGraph reads the frames since the last request. Each tool call
// is a node; an edit requires the latest read or search of its file, a run
// or check requires the edits since the previous run, and a check's state
// is what its exit said.
func buildTurnGraph(msgs []Message, width int) *turnGraphState {
	start := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			start = i
			break
		}
	}
	g := &turnGraphState{}
	lastSeen := map[string]string{} // file → the node that last read or searched it
	var editsSinceRun []string
	var lastRun string
	reads, edits, runs := 0, 0, 0
	lastCheck := ""
	for i := start; i < len(msgs); i++ {
		msg := msgs[i]
		if msg.Role != "tool_call" || msg.ToolName == "" || graphBookkeeping(msg.ToolName) {
			continue
		}
		n := len(g.nodes)
		label := plainText(viewFor(msg.ToolName).Label(msg.ToolInput, 60))
		name := fmt.Sprintf("%d %s", n+1, truncateEnd(label, 44))
		t := WorkflowTask{Name: name, State: "done"}
		switch {
		case !msg.toolSettled():
			t.State = "running"
			st := msg.Timestamp
			t.Started = &st
		case msg.ToolError != "" || strings.Contains(msg.ToolOutput, commandFailedMark):
			t.State = "failed"
			t.Error = firstLineOf(strings.TrimSpace(firstNonEmpty(msg.ToolError, afterMark(msg.ToolOutput, commandFailedMark))))
		}
		if msg.ToolTook > 0 {
			t.Took = msg.ToolTook.Round(time.Second).String()
		}
		path := framePath(msg)
		switch toolKindOf(msg.ToolName) {
		case fileToolEdit:
			edits++
			if path != "" {
				if dep, ok := lastSeen[path]; ok {
					t.Requires = append(t.Requires, dep)
				}
				lastSeen[path] = name
			}
			editsSinceRun = append(editsSinceRun, name)
		case fileToolRead:
			reads++
			if path != "" {
				lastSeen[path] = name
			}
			// A search names its files in its result: an edit of one of
			// them depends on the search that found it.
			if files, _ := searchHitFiles(msg.ToolName, msg.ToolOutput); len(files) > 0 {
				for _, f := range files {
					lastSeen[f] = name
				}
			}
		default:
			if strings.EqualFold(msg.ToolName, "bash") {
				runs++
				if len(editsSinceRun) > 0 {
					t.Requires = append(t.Requires, editsSinceRun...)
				} else if lastRun != "" {
					t.Requires = append(t.Requires, lastRun)
				}
				editsSinceRun = nil
				lastRun = name
				if t.State != "running" {
					lastCheck = t.State
				}
			}
		}
		g.nodes = append(g.nodes, turnNode{task: t, path: path, index: i})
	}
	tasks := make([]WorkflowTask, len(g.nodes))
	byName := map[string]int{}
	for i, n := range g.nodes {
		tasks[i] = n.task
		byName[n.task.Name] = i
	}
	for _, layer := range workflowLayers(tasks) {
		var idx []int
		for _, t := range layer {
			idx = append(idx, byName[t.Name])
		}
		g.layers = append(g.layers, idx)
	}
	g.head = fmt.Sprintf("this turn · %d reads · %d edits · %d runs", reads, edits, runs)
	switch lastCheck {
	case "done":
		g.head += " · last run passed"
	case "failed":
		g.head += " · last run FAILED"
	}
	return g
}

func framePath(msg Message) string {
	var in map[string]any
	if json.Unmarshal([]byte(msg.ToolInput), &in) != nil {
		return ""
	}
	for _, k := range []string{"file_path", "path", "file"} {
		if v, ok := in[k].(string); ok && v != "" {
			return v
		}
	}
	for _, k := range []string{"patch", "input"} {
		if v, ok := in[k].(string); ok {
			if mm := patchFilesRe.FindStringSubmatch(v); mm != nil {
				return mm[1]
			}
		}
	}
	return ""
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func afterMark(s, mark string) string {
	if i := strings.Index(s, mark); i >= 0 {
		return s[i:]
	}
	return s
}

// updateTurnGraph takes every key while the graph is open.
func (m *Model) updateTurnGraph(msg tea.Msg) (bool, tea.Cmd) {
	g := m.turnGraph
	if g == nil {
		return false, nil
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return false, nil
	}
	switch k.Type {
	case tea.KeyEsc, tea.KeyCtrlG:
		m.turnGraph = nil
		return true, nil
	case tea.KeyUp:
		if g.index > 0 {
			g.index--
		}
		return true, nil
	case tea.KeyDown:
		if g.index < len(g.nodes)-1 {
			g.index++
		}
		return true, nil
	case tea.KeyEnter:
		if n := g.nodes[g.index]; n.path != "" {
			m.turnGraph = nil
			m.openFilesPanel(n.path)
		}
		return true, nil
	}
	return true, nil
}

// renderTurnGraph draws the layers like the workflow panel, the selected
// node on the selection pair, the window scrolled to keep it in view.
func (m Model) renderTurnGraph() string {
	g := m.turnGraph
	if g == nil {
		return ""
	}
	// A frame may have settled since the graph was built: rebuild cheaply.
	fresh := buildTurnGraph(m.messages, m.width)
	if len(fresh.nodes) >= len(g.nodes) {
		fresh.index = g.index
		if fresh.index >= len(fresh.nodes) {
			fresh.index = len(fresh.nodes) - 1
		}
		g = fresh
	}
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(colText)).Bold(true)
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color(colSelFG)).Background(lipgloss.Color(colSelBG))
	width := m.width - 2
	if width < 40 {
		width = 40
	}
	w := 0
	for _, n := range g.nodes {
		if len(n.task.Name) > w {
			w = len(n.task.Name)
		}
	}
	type row struct {
		text string
		node int
	}
	var rows []row
	for li, layer := range g.layers {
		if li > 0 {
			rows = append(rows, row{dim.Render("  │"), -1})
		}
		for _, ni := range layer {
			t := g.nodes[ni].task
			gl, c := workflowGlyph(t.State)
			detail := t.State
			if t.State == "running" && t.Started != nil {
				detail += " " + time.Since(*t.Started).Round(time.Second).String()
			} else if t.Took != "" {
				detail += " " + t.Took
			}
			line := fmt.Sprintf("%s %-*s  %s", lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(gl), w, t.Name, dim.Render(detail))
			if len(t.Requires) > 0 {
				line += dim.Render("  ← " + strings.Join(t.Requires, ", "))
			}
			rows = append(rows, row{"  " + line, ni})
			if t.Error != "" && t.State == "failed" {
				rows = append(rows, row{"      " + lipgloss.NewStyle().Foreground(lipgloss.Color(colErr)).Render(truncateEnd(t.Error, width-8)), ni})
			}
		}
	}
	selRow := 0
	for i, r := range rows {
		if r.node == g.index {
			selRow = i
			break
		}
	}
	start := 0
	if selRow >= turnGraphRows {
		start = selRow - turnGraphRows + 1
	}
	var out []string
	out = append(out, title.Render(truncateTo("  "+g.head+"   ↑↓ move · enter the file · esc closes", width)))
	for i := start; i < len(rows) && i < start+turnGraphRows; i++ {
		r := rows[i]
		if r.node == g.index && r.node >= 0 {
			out = append(out, sel.Render(padTo(truncateTo(plainText(r.text), width), width)))
			continue
		}
		out = append(out, truncateTo(r.text, width))
	}
	if rest := len(rows) - (start + turnGraphRows); rest > 0 {
		out = append(out, dim.Render(fmt.Sprintf("  … %d more rows", rest)))
	}
	return strings.Join(out, "\n")
}

// graphBookkeeping: the agent's own housekeeping is not a step of the work.
func graphBookkeeping(tool string) bool {
	switch strings.ToLower(tool) {
	case "notes", "todo_write", "todo_read", "recall", "memory":
		return true
	}
	return false
}

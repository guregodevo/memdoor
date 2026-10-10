package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// How a tool presents itself in the transcript.
//
// Every tool answers the same three questions — what colour is this kind of
// work, what does the call line say, and how does its result read — so adding a
// tool means writing a ToolView, not extending three switch statements in the
// renderer. Unknown tools (including ones an agent writes for itself) get the
// generic view rather than a guess.
type ToolView interface {
	// Colour is the palette entry the tool's name wears.
	Colour() string
	// Label is the one-line call header, e.g. `Bash(go test ./...)`.
	Label(input string, width int) string
	// Body renders the result. Empty means "use the generic body" — the
	// markdown-formatted output with the usual line cap.
	Body(r ToolRender) string
}

// ToolRender is what a view needs from the TUI to draw a body, passed
// explicitly so views stay testable without a Model.
type ToolRender struct {
	Input, Output string
	Err           string // non-empty when the tool failed
	Width         int
	Expand        bool // ctrl+o — show everything
	// Trail is what a spawned run has done so far (spawnView), TrailDone
	// whether it reported back; the renderer fills them from the window's
	// mirror of the child's beats.
	Trail     []trailStep
	TrailDone bool
	// Root is the project the window runs in: a search's absolute paths
	// read relative to it.
	Root string
}

// viewFor resolves a tool to its view. Aliases are folded here (the model
// writes `bash` or `Bash`, `apply_patch` or `Edit`) so the rest of the TUI
// never has to know which spelling arrived.
func viewFor(tool string) ToolView {
	switch strings.ToLower(tool) {
	case "bash", "shell", "run", "run_command":
		return commandView{}
	case "read", "read_file", "ls", "fetch":
		return readView{tool: tool}
	case "grep", "glob", "locate", "search", "web_search":
		return searchView{tool: tool}
	case "sessions_spawn":
		return spawnView{}
	case "workflow":
		return workflowView{}
	case "jgrep", "jread", "jlogs":
		return judgedView{tool: strings.ToLower(tool)}
	case "apply_patch", "edit", "edit_file", "write", "write_file", "create_file", "str_replace":
		return writeView{tool: tool}
	case "todo_write", "todo_read", "exit_plan_mode":
		return planView{tool: tool}
	case "memory", "recall":
		return recallView{tool: tool}
	case "ask_user_question", "report_bug":
		return askView{tool: tool}
	case "cron":
		return cronView{}
	}
	return genericView{tool: tool}
}

// displayName maps a TECHNICAL tool name — what the model calls, what the
// session JSON records, what `memdoor logs query` prints — to the name the TUI
// shows a human.
//
// The two are deliberately different and must not be confused. "apply_patch" is
// the contract; "Edit" is the label. Keeping the mapping in ONE place is what
// makes that safe: it used to be hardcoded inside each view's Label, so the
// same tool could render differently in two frames, and reading a tool name off
// the screen told you nothing reliable about what to grep for. (Measured
// 2026-08-30: counting tools from the pane missed every apply_patch, because
// the screen says "Edit" and the logs say "apply_patch".)
//
// A tool with no entry renders under its technical name — a missing mapping
// should look plain, never wrong.
var toolDisplayNames = map[string]string{
	"apply_patch":    "Edit",
	"edit":           "Edit",
	"edit_file":      "Edit",
	"str_replace":    "Edit",
	"write":          "Write",
	"write_file":     "Write",
	"create_file":    "Write",
	"read_file":      "Read",
	"bash":           "Bash",
	"todo_write":     "Plan",
	"todo_read":      "Plan",
	"exit_plan_mode": "Plan",
}

// displayName returns what to SHOW for a tool. Never use it to identify a tool:
// several technical names share one label.
func displayName(tool string) string {
	if n, ok := toolDisplayNames[strings.ToLower(tool)]; ok {
		return n
	}
	return tool
}

// ---- running a command ------------------------------------------------------

type commandView struct{}

func (commandView) Colour() string { return colRun }

func (commandView) Label(input string, width int) string {
	if cmd := stringField(input, "command"); cmd != "" {
		max := width - 15 // room for "Bash()" and the margins
		if max < 40 {
			max = 40
		}
		if len(cmd) > max {
			cmd = cmd[:max-3] + "..."
		}
		return fmt.Sprintf("Bash(%s)", cmd)
	}
	return genericLabel(displayName("bash"), input, width)
}

func (commandView) Body(ToolRender) string { return "" }

// ---- reading and searching --------------------------------------------------

type readView struct{ tool string }

func (readView) Colour() string { return colRead }

func (v readView) Label(input string, width int) string {
	name := displayName(v.tool)
	for _, k := range []string{"file_path", "path", "pattern", "query", "url"} {
		if s := stringField(input, k); s != "" {
			return fmt.Sprintf("%s(%s)", name, s)
		}
	}
	return genericLabel(name, input, width)
}

// Body summarises a read instead of reprinting the file — the agent needed the
// contents, the reader needs to know it looked.
func (v readView) Body(r ToolRender) string {
	if strings.ToLower(v.tool) != "read_file" && strings.ToLower(v.tool) != "read" {
		return "" // grep/glob/locate results ARE the answer — show them
	}
	if strings.TrimSpace(r.Output) == "" {
		return ""
	}
	n := len(strings.Split(strings.TrimRight(r.Output, "\n"), "\n"))
	return "  " + frameConnector + " " + dimStyle().Render(fmt.Sprintf("Read %d lines", n))
}

// ---- the agent's own schedule --------------------------------------------------

// cronView shows a scheduled check as the schedule it is — "Cron(every 30s ×10:
// check whether CI passed)" — so the reader sees the cadence and the bound at
// a glance, and a stop as what it stopped.
type cronView struct{}

func (cronView) Colour() string { return colRun }

func (cronView) Label(input string, width int) string {
	action := strings.ToLower(stringField(input, "action"))
	switch action {
	case "every", "create", "add":
		every, task := stringField(input, "every"), stringField(input, "task")
		times := fieldText(input, "times")
		if times == "" {
			times = "10"
		}
		head := fmt.Sprintf("Cron(every %s ×%s: ", every, times)
		max := width - len(head) - 3
		if max < 24 {
			max = 24
		}
		if len(task) > max {
			task = task[:max-1] + "…"
		}
		return head + task + ")"
	case "stop", "cancel", "remove":
		return "Cron(stop " + stringField(input, "id") + ")"
	case "list":
		return "Cron(list)"
	}
	return genericLabel("Cron", input, width)
}

// Body is the tool's own sentence — "Scheduled poll-123: every 30s, up to 10
// times…" — which already says everything; the generic body would reprint it
// under a cap it never reaches.
func (cronView) Body(r ToolRender) string { return "" }

// ---- judged reads ------------------------------------------------------------

// judgedView shows what the decision model decided, not the text it let
// through. A judged read's verdict is its FIRST line ("kept 3 of 41 hunks")
// and its value is which places survived and how sure the judge was; the
// tail-first preview every other tool gets showed the last lines of the last
// hunk instead, and the label showed keep: "10" rather than the pattern
// (2026-09-27, recording the landing demo). ctrl+o still shows everything.
type judgedView struct{ tool string }

func (judgedView) Colour() string { return colRead }

func (v judgedView) Label(input string, width int) string {
	for _, k := range []string{"pattern", "path", "file_path", "query"} {
		if s := stringField(input, k); s != "" {
			return fmt.Sprintf("%s(%s)", v.tool, s)
		}
	}
	return genericLabel(v.tool, input, width)
}

func (v judgedView) Body(r ToolRender) string {
	if r.Expand || strings.TrimSpace(r.Output) == "" {
		return "" // the full result, as every tool shows it
	}
	verdict, places := judgedSummary(r.Output)
	if verdict == "" {
		return ""
	}
	const shown = 5
	b := strings.Builder{}
	b.WriteString("  " + frameConnector + " " + verdict)
	for i, p := range places {
		if i == shown {
			b.WriteString("\n     " + dimStyle().Render(fmt.Sprintf("+%d more (ctrl+o)", len(places)-shown)))
			break
		}
		b.WriteString("\n     " + p)
	}
	return b.String()
}

// judgedSummary reads a judged result's first line and the place of each hunk
// it kept: "== path:2-9  (p=0.83)" from jgrep, "(p=0.80) path:1-38" from jread.
// Paths are shown relative to the working directory.
func judgedSummary(out string) (verdict string, places []string) {
	cwd, _ := os.Getwd()
	rel := func(path string) string {
		if cwd != "" && strings.HasPrefix(path, cwd+"/") {
			return strings.TrimPrefix(path, cwd+"/")
		}
		return path
	}
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if verdict == "" {
			verdict = strings.TrimSuffix(strings.TrimPrefix(t, "jgrep: "), ":")
			if cwd != "" {
				verdict = strings.ReplaceAll(verdict, cwd+"/", "")
			}
			continue
		}
		switch {
		case strings.HasPrefix(t, "== "):
			f := strings.Fields(strings.TrimPrefix(t, "== "))
			if len(f) >= 2 {
				places = append(places, rel(f[0])+"  "+strings.Trim(f[1], "()"))
			} else if len(f) == 1 {
				places = append(places, rel(f[0]))
			}
		case strings.HasPrefix(t, "(p="):
			if f := strings.Fields(t); len(f) >= 2 {
				places = append(places, rel(f[1])+"  "+strings.Trim(f[0], "()"))
			}
		}
	}
	return verdict, places
}

// ---- changing files ---------------------------------------------------------

type writeView struct{ tool string }

func (writeView) Colour() string { return colWrite }

func (v writeView) Label(input string, width int) string {
	name := displayName(v.tool)
	for _, k := range []string{"file_path", "path"} {
		if s := stringField(input, k); s != "" {
			return fmt.Sprintf("%s(%s)", name, s)
		}
	}
	// apply_patch carries its paths inside the patch body — every one of
	// them: a two-file patch labelled with its first file read as an edit to
	// that file alone, next to an error about the other (2026-09-28).
	if paths := patchTargets(unwrapPatch(input)); len(paths) > 0 {
		if len(paths) > 3 {
			paths = append(paths[:3], fmt.Sprintf("+%d more", len(paths)-3))
		}
		return fmt.Sprintf("%s(%s)", name, strings.Join(paths, ", "))
	}
	return genericLabel(name, input, width)
}

// Body renders the change as a diff — the whole point of an edit frame is
// seeing WHAT changed, which a truncated one-line header cannot show.
func (v writeView) Body(r ToolRender) string {
	if patch := unwrapPatch(r.Input); strings.Contains(patch, "*** ") {
		// A failed patch changed nothing. Showing its diff under a plain
		// "Update foo.go +1 −0" header reads as applied — the reader has to
		// notice the error line underneath to learn otherwise.
		return renderDiff(patch, fileEcho(r.Output), r.Expand, r.Err != "")
	}
	return ""
}

// ---- plans and checklists ---------------------------------------------------

type planView struct{ tool string }

func (planView) Colour() string { return colPlan }

// Label counts the steps instead of stringifying the list — a Go map printed
// into a label ("[map[active_form:… content:…") tells the reader nothing.
func (v planView) Label(input string, width int) string {
	if n := countTodos(input); n > 0 {
		unit := "steps"
		if n == 1 {
			unit = "step"
		}
		return fmt.Sprintf("%s(%d %s)", displayName(v.tool), n, unit)
	}
	return displayName(v.tool) + "()"
}

func countTodos(input string) int {
	var p struct {
		Todos []json.RawMessage `json:"todos"`
	}
	if json.Unmarshal([]byte(input), &p) == nil {
		return len(p.Todos)
	}
	return 0
}

// Body draws the list as a checklist. It is the densest frame in a run and the
// one worth reading at a glance, so it gets glyphs, a progress bar, and the
// follow-up kept visible instead of a wall of "1. [x] …" lines.
func (v planView) Body(r ToolRender) string {
	return renderChecklist(r.Output, r.Expand)
}

// ---- memory -----------------------------------------------------------------

type recallView struct{ tool string }

func (recallView) Colour() string { return colRecall }

func (v recallView) Label(input string, width int) string {
	// Show the meaningful argument (query/slug/title), never an incidental
	// "limit" — Go map iteration is random, so a generic pick would print
	// limit: "1" on some turns and the query on others.
	for _, k := range []string{"query", "slug", "title", "text"} {
		if s := stringField(input, k); s != "" {
			return fmt.Sprintf("%s(%s)", displayName(v.tool), s)
		}
	}
	return genericLabel(displayName(v.tool), input, width)
}

func (recallView) Body(ToolRender) string { return "" }

// ---- asking the human -------------------------------------------------------

type askView struct{ tool string }

func (askView) Colour() string { return colAsk }

func (v askView) Label(input string, width int) string {
	if q := stringField(input, "question"); q != "" {
		return fmt.Sprintf("%s(%s)", displayName(v.tool), q)
	}
	return genericLabel(displayName(v.tool), input, width)
}

func (askView) Body(ToolRender) string { return "" }

// ---- anything else ----------------------------------------------------------

type genericView struct{ tool string }

func (genericView) Colour() string                         { return colDim }
func (v genericView) Label(input string, width int) string { return genericLabel(v.tool, input, width) }
func (genericView) Body(ToolRender) string                 { return "" }

// ---- shared helpers ---------------------------------------------------------

// resultBlock is the shape every tool result takes: a ⎿ connector on the first
// line, continuation lines aligned under it, and a trailer that says how much
// was left out. One shape for every tool means the eye learns it once.
func resultBlock(body string, max int, expand bool) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if expand {
		max = 2000
	}
	// Keep the TAIL, not the head: for command output the newest lines are the
	// answer (a 1..10 counter capped at the head hid 9 and 10 — the very lines
	// the user was watching stream), and a build/test failure's verdict is at
	// the bottom. The elision note goes FIRST so the frame reads top-to-bottom.
	shown := lines
	hidden := 0
	if len(lines) > max {
		hidden = len(lines) - max
		shown = lines[len(lines)-max:]
	}
	var b strings.Builder
	if hidden > 0 {
		b.WriteString("  " + frameConnector + " " + dimStyle().Render(fmt.Sprintf("… +%d earlier lines (ctrl+o)", hidden)))
		for _, l := range shown {
			b.WriteString("\n     " + l)
		}
		return b.String()
	}
	b.WriteString("  " + frameConnector + " " + shown[0])
	for _, l := range shown[1:] {
		b.WriteString("\n     " + l)
	}
	return b.String()
}

// frameConnector ties a result to the call above it, the way every tool frame
// in a modern coding TUI does.
const frameConnector = "⎿"

func dimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Italic(true)
}

// stringField pulls one string field out of a tool's JSON input.
// fieldText is a field as text whatever its JSON type: the cron tool's
// `times` is a number, and stringField would read it as nothing.
func fieldText(input, key string) string {
	if input == "" {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(input), &m); err != nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	}
	return ""
}

func stringField(input, key string) string {
	if input == "" {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(input), &m); err != nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

// genericLabel shows ONE parameter, chosen deterministically: a
// human-meaningful key when there is one, else the alphabetically first, so the
// label doesn't change between turns (map ranging is randomised).
func genericLabel(tool, input string, width int) string {
	if input == "" {
		return tool + "()"
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(input), &m); err == nil {
		if key := pickDisplayKey(m); key != "" {
			value := fmt.Sprintf("%v", m[key])
			max := width - len(tool) - 20
			if max < 40 {
				max = 40
			}
			if max > 80 {
				max = 80
			}
			if len(value) > max {
				value = value[:max-3] + "..."
			}
			return fmt.Sprintf("%s(%s: %q)", tool, key, value)
		}
	}
	return tool + "(...)"
}

// patchTargets is every file a patch names, in order, each once.
func patchTargets(patch string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(patch, "\n") {
		if path, _ := patchTarget(line); path != "" && !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	return out
}

// patchTarget reads the file and operation out of a `*** Add File: x` header.
func patchTarget(patch string) (path, op string) {
	for _, line := range strings.Split(patch, "\n") {
		line = strings.TrimSpace(line)
		for _, p := range []struct{ prefix, verb string }{
			{"*** Add File: ", "Add"},
			{"*** Update File: ", "Update"},
			{"*** Delete File: ", "Delete"},
		} {
			if strings.HasPrefix(line, p.prefix) {
				return strings.TrimSpace(strings.TrimPrefix(line, p.prefix)), p.verb
			}
		}
	}
	return "", ""
}

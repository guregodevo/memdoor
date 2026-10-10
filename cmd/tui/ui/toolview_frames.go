package ui

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// The frames that used to be a generic line of text (Greg, 2026-10-11: "we
// need better visualization in general for some tools"): a spawned run with
// its own trail, a search as its hits grouped by file, a workflow call as its
// tasks. Each says what happened in the shape the reader scans for, and
// ctrl+o still shows the raw result (an expanded frame returns "" here, so
// the generic body prints everything).

// ---- a spawned run ------------------------------------------------------------

// trailStep is one thing a spawned run did, mirrored from its session's
// beats (agent_runtime_progress.go): the tool and how long it has been in it.
type trailStep struct {
	tool    string
	seconds int
}

// spawnTrail is what the window keeps of one spawned run: its steps in
// order and whether it has reported back.
type spawnTrail struct {
	steps []trailStep
	done  bool
	last  time.Time // the latest beat
}

const trailShown = 6

// The spawned line, with or without the label the model gave the run:
// ▶ run-7f3a "tests" spawned as coder (session agent:coder:subagent:run-7f3a, up to 600s)
var spawnedLineRe = regexp.MustCompile(`^▶ (\S+)(?: "([^"]*)")? spawned as (\S+) \(session (\S+), up to (\d+)s\)`)

// spawnSession is the child session named by sessions_spawn's result.
func spawnSession(output string) string {
	if m := spawnedLineRe.FindStringSubmatch(strings.TrimSpace(output)); m != nil {
		return m[4]
	}
	return ""
}

type spawnView struct{}

func (spawnView) Colour() string { return colRun }

func (spawnView) Label(input string, width int) string {
	for _, k := range []string{"label", "task", "message", "prompt"} {
		if s := stringField(input, k); s != "" {
			max := width - 15
			if max < 40 {
				max = 40
			}
			if len(s) > max {
				s = s[:max-3] + "..."
			}
			return fmt.Sprintf("Spawn(%s)", firstLineOf(s))
		}
	}
	return genericLabel("Spawn", input, width)
}

// Body: the run's head line (which agent, which run, its bound) and, under
// it, what the child is doing as it does it, newest last; ✓ when it has
// reported back.
func (spawnView) Body(r ToolRender) string {
	if r.Err != "" || r.Expand {
		return ""
	}
	m := spawnedLineRe.FindStringSubmatch(strings.TrimSpace(r.Output))
	if m == nil {
		return ""
	}
	dim := dimStyle()
	run := lipgloss.NewStyle().Foreground(lipgloss.Color(colRun))
	var b strings.Builder
	head := "▶ " + m[3]
	if m[2] != "" {
		head += " · " + m[2]
	}
	b.WriteString("  " + frameConnector + " " + run.Render(head) + dim.Render(fmt.Sprintf(" · %s · up to %ss", m[1], m[5])))
	steps := r.Trail
	if hidden := len(steps) - trailShown; hidden > 0 {
		b.WriteString("\n     " + dim.Render(fmt.Sprintf("… +%d earlier steps", hidden)))
		steps = steps[hidden:]
	}
	for i, s := range steps {
		line := "◦ " + displayName(s.tool)
		if s.seconds > 0 {
			line += fmt.Sprintf(" %ds", s.seconds)
		}
		if i == len(steps)-1 && !r.TrailDone {
			b.WriteString("\n     " + run.Render(line))
		} else {
			b.WriteString("\n     " + dim.Render(line))
		}
	}
	switch {
	case r.TrailDone:
		b.WriteString("\n     " + lipgloss.NewStyle().Foreground(lipgloss.Color(colOK)).Render("✓ reported back"))
	case len(steps) == 0:
		b.WriteString("\n     " + dim.Render("starting …"))
	}
	return b.String()
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i]) + " …"
	}
	return s
}

// ---- a search: hits grouped --------------------------------------------------------

type searchView struct{ tool string }

func (searchView) Colour() string { return colRead }

func (v searchView) Label(input string, width int) string {
	return readView{tool: v.tool}.Label(input, width)
}

const searchGroupsShown = 8

var (
	grepHitRe    = regexp.MustCompile(`^(.+?):(\d+):`)
	grepCountRe  = regexp.MustCompile(`^(.+?): (\d+)$`)
	grepPathRe   = regexp.MustCompile(`^((?:/|\./|[\w.-]+/)?[^:\s]+\.[A-Za-z0-9_]+):`) // file:content, no line number
	locateRefRe  = regexp.MustCompile(`^\d+\. (\S+?)(?::\d+)?(?:-\d+)?\s+p=`)
	webSourceRe  = regexp.MustCompile(`^\[(\d+)\] (.*?) — (\S+)$`)
	locateHeadRe = regexp.MustCompile(`^(Where .* lives|Sections of )`)
)

// Body: a grep as "N hits in M files" and the files with their counts, a
// glob as its files, locate as its ranked places without the sections, a
// web search as its sources. Nothing found reads as nothing found.
func (v searchView) Body(r ToolRender) string {
	if r.Err != "" || r.Expand {
		return ""
	}
	out := strings.TrimRight(r.Output, "\n")
	if strings.TrimSpace(out) == "" {
		return "  " + frameConnector + " " + dimStyle().Render("no hits")
	}
	switch strings.ToLower(v.tool) {
	case "grep", "search":
		return grepBody(out, r.Root)
	case "glob":
		return listBody(nonBlank(out), "file", "files")
	case "locate":
		return locateBody(out)
	case "web_search":
		return webSearchBody(out)
	}
	return ""
}

func nonBlank(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// searchHits groups a grep's output by file: hits per file in first-seen
// order, from match lines (file:line:text), count lines (file: n) or a
// bare file list.
func searchHits(out string) (files []string, hits map[string]int, total int) {
	hits = map[string]int{}
	add := func(f string, n int) {
		if _, seen := hits[f]; !seen {
			files = append(files, f)
		}
		hits[f] += n
		total += n
	}
	for _, l := range nonBlank(out) {
		if m := grepHitRe.FindStringSubmatch(l); m != nil {
			add(m[1], 1)
		} else if m := grepCountRe.FindStringSubmatch(l); m != nil {
			n := 0
			fmt.Sscanf(m[2], "%d", &n)
			add(m[1], n)
		} else if m := grepPathRe.FindStringSubmatch(l); m != nil {
			add(m[1], 1)
		} else if !strings.ContainsAny(l, " \t") && strings.Contains(l, ".") {
			add(l, 1)
		}
	}
	return files, hits, total
}

func grepBody(out, root string) string {
	if strings.HasPrefix(out, "No matches") || strings.HasPrefix(out, "No files") {
		return "  " + frameConnector + " " + dimStyle().Render("no hits")
	}
	files, hits, total := searchHits(out)
	if len(files) == 0 {
		return ""
	}
	sort.SliceStable(files, func(i, j int) bool { return hits[files[i]] > hits[files[j]] })
	dim := dimStyle()
	var b strings.Builder
	fmt.Fprintf(&b, "  %s %d hit%s in %d file%s", frameConnector, total, plural(total), len(files), plural(len(files)))
	// Absolute paths from one folder read as the folder once and the
	// files short (live 2026-10-11: three 120-column paths for two files).
	prefix := commonDir(files)
	if prefix != "" {
		if shown := relativeDir(prefix, root); shown != "" {
			b.WriteString(dim.Render(" · in " + shown))
		}
	}
	shown := files
	if len(shown) > searchGroupsShown {
		shown = shown[:searchGroupsShown]
	}
	for _, f := range shown {
		b.WriteString("\n     " + strings.TrimPrefix(f, prefix) + dim.Render(fmt.Sprintf(" · %d", hits[f])))
	}
	if more := len(files) - len(shown); more > 0 {
		b.WriteString("\n     " + dim.Render(fmt.Sprintf("… +%d files (ctrl+o)", more)))
	}
	return b.String()
}

func listBody(lines []string, one, many string) string {
	if len(lines) == 0 {
		return ""
	}
	dim := dimStyle()
	var b strings.Builder
	name := many
	if len(lines) == 1 {
		name = one
	}
	fmt.Fprintf(&b, "  %s %d %s", frameConnector, len(lines), name)
	shown := lines
	if len(shown) > searchGroupsShown {
		shown = shown[:searchGroupsShown]
	}
	for _, l := range shown {
		b.WriteString("\n     " + l)
	}
	if more := len(lines) - len(shown); more > 0 {
		b.WriteString("\n     " + dim.Render(fmt.Sprintf("… +%d more (ctrl+o)", more)))
	}
	return b.String()
}

// locateBody keeps locate's head line and its ranked places; the sections
// it quoted are the model's to read, ctrl+o shows them.
func locateBody(out string) string {
	lines := nonBlank(out)
	var head string
	var refs []string
	for _, l := range lines {
		switch {
		case head == "" && locateHeadRe.MatchString(l):
			head = l
		case locateRefRe.MatchString(l) || strings.HasPrefix(l, "== "):
			refs = append(refs, strings.TrimPrefix(l, "== "))
		}
	}
	if head == "" && len(refs) == 0 {
		return ""
	}
	dim := dimStyle()
	var b strings.Builder
	if head == "" {
		head = fmt.Sprintf("%d places", len(refs))
	}
	b.WriteString("  " + frameConnector + " " + dim.Render(strings.TrimSuffix(head, ":")))
	shown := refs
	if len(shown) > searchGroupsShown {
		shown = shown[:searchGroupsShown]
	}
	for _, l := range shown {
		b.WriteString("\n     " + l)
	}
	if more := len(refs) - len(shown); more > 0 {
		b.WriteString("\n     " + dim.Render(fmt.Sprintf("… +%d more (ctrl+o)", more)))
	}
	return b.String()
}

// webSearchBody: the answer's first line, then the sources it cited.
func webSearchBody(out string) string {
	answer, sources, _ := strings.Cut(out, "\n\nSources:")
	var cited []string
	for _, l := range nonBlank(sources) {
		if m := webSourceRe.FindStringSubmatch(l); m != nil {
			cited = append(cited, fmt.Sprintf("[%s] %s %s", m[1], m[2], dimStyle().Render(hostOf(m[3]))))
		}
	}
	dim := dimStyle()
	var b strings.Builder
	fmt.Fprintf(&b, "  %s %d source%s", frameConnector, len(cited), plural(len(cited)))
	if first := strings.TrimSpace(strings.SplitN(strings.TrimSpace(answer), "\n", 2)[0]); first != "" {
		b.WriteString("\n     " + dim.Render(cutKeepingSpace(first, 100)))
	}
	for i, c := range cited {
		if i == searchGroupsShown {
			b.WriteString("\n     " + dim.Render(fmt.Sprintf("… +%d more (ctrl+o)", len(cited)-i)))
			break
		}
		b.WriteString("\n     " + c)
	}
	return b.String()
}

func hostOf(u string) string {
	u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	if i := strings.IndexByte(u, '/'); i > 0 {
		u = u[:i]
	}
	return strings.TrimPrefix(u, "www.")
}

// ---- the workflow tool --------------------------------------------------------------

type workflowView struct{}

func (workflowView) Colour() string { return colPlan }

func (workflowView) Label(input string, width int) string {
	var parts []string
	for _, k := range []string{"action", "name", "run_id", "task"} {
		if s := stringField(input, k); s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return genericLabel("Workflow", input, width)
	}
	return fmt.Sprintf("Workflow(%s)", strings.Join(parts, " "))
}

var workflowTaskLineRe = regexp.MustCompile(`^  (\S+)\s+(\S+)(.*)$`)

// Body: the run's head line and its tasks as the panel draws them, one
// glyph per state (workflowGlyph), what a task requires dim after it.
func (workflowView) Body(r ToolRender) string {
	if r.Err != "" || r.Expand {
		return ""
	}
	var head []string
	var tasks []string
	dim := dimStyle()
	for _, l := range strings.Split(strings.TrimRight(r.Output, "\n"), "\n") {
		if m := workflowTaskLineRe.FindStringSubmatch(l); m != nil {
			glyph, colour := workflowGlyph(m[1])
			rest := strings.TrimSpace(m[3])
			if strings.Contains(rest, "(external)") && m[1] != "done" {
				glyph, colour = "⏸", colPlan
			}
			line := lipgloss.NewStyle().Foreground(lipgloss.Color(colour)).Render(glyph) + " " + m[2]
			if rest != "" {
				line += "  " + dim.Render(rest)
			}
			tasks = append(tasks, line)
			continue
		}
		if strings.TrimSpace(l) != "" && len(tasks) == 0 {
			head = append(head, l)
		}
	}
	if len(tasks) == 0 {
		return ""
	}
	var b strings.Builder
	first := "run"
	if len(head) > 0 {
		first = cutKeepingSpace(head[len(head)-1], 120)
	}
	b.WriteString("  " + frameConnector + " " + first)
	for _, t := range tasks {
		b.WriteString("\n     " + t)
	}
	return b.String()
}

// noteSubagentStep keeps the spawned run's trail: a new tool is a new step,
// the same tool again is the same step with more seconds, done closes it.
func (m *Model) noteSubagentStep(msg subagentWorkMsg) {
	if m.subagentTrail == nil {
		m.subagentTrail = map[string]*spawnTrail{}
	}
	t := m.subagentTrail[msg.sessionID]
	if t == nil {
		t = &spawnTrail{}
		m.subagentTrail[msg.sessionID] = t
	}
	t.last = time.Now()
	if msg.done {
		t.done = true
		return
	}
	if msg.tool == "" {
		return
	}
	if n := len(t.steps); n > 0 && t.steps[n-1].tool == msg.tool && msg.seconds >= t.steps[n-1].seconds {
		t.steps[n-1].seconds = msg.seconds
		return
	}
	t.steps = append(t.steps, trailStep{tool: msg.tool, seconds: msg.seconds})
}

// searchHitFiles are the files a search's result names, with how many
// times each, for the files panel.
func searchHitFiles(tool, output string) (files []string, counts map[string]int) {
	counts = map[string]int{}
	switch tool {
	case "grep", "search":
		files, counts, _ = searchHits(output)
	case "glob":
		files = nonBlank(output)
		for _, f := range files {
			counts[f]++
		}
	case "locate":
		for _, l := range nonBlank(output) {
			if m := locateRefRe.FindStringSubmatch(l); m != nil {
				if counts[m[1]] == 0 {
					files = append(files, m[1])
				}
				counts[m[1]]++
			}
		}
	}
	return files, counts
}

// commonDir is the directory every path shares, with its trailing slash,
// when it is more than the root; "" otherwise.
func commonDir(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	prefix := paths[0]
	for _, p := range paths[1:] {
		for !strings.HasPrefix(p, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	if i := strings.LastIndex(prefix, "/"); i > 0 {
		return prefix[:i+1]
	}
	return ""
}

// spawnGrace is how long a spawn frame waits for its child's first beat
// before it may be frozen; spawnStale how long a silent, unfinished trail
// keeps a frame live (a child that died without its done beat).
const (
	spawnGrace = time.Minute
	spawnStale = 10 * time.Minute
)

// spawnLive: the frame of a spawned run whose child is still working. Such
// a frame must not be frozen into scrollback (scrollback.go), or the trail
// it draws lands on a copy nobody sees.
func (m Model) spawnLive(msg Message) bool {
	if msg.Role != "tool_call" || !strings.EqualFold(msg.ToolName, "sessions_spawn") {
		return false
	}
	session := spawnSession(msg.ToolOutput)
	if session == "" {
		return false
	}
	t := m.subagentTrail[session]
	if t == nil {
		return time.Since(msg.Timestamp) < spawnGrace
	}
	return !t.done && time.Since(t.last) < spawnStale
}

// trailKey is the trail's part of a spawn frame's render-cache key.
func (m Model) trailKey(msg Message) string {
	if msg.Role != "tool_call" || !strings.EqualFold(msg.ToolName, "sessions_spawn") {
		return ""
	}
	t := m.subagentTrail[spawnSession(msg.ToolOutput)]
	if t == nil {
		return ""
	}
	last := 0
	if n := len(t.steps); n > 0 {
		last = t.steps[n-1].seconds
	}
	return fmt.Sprintf("%d:%d:%v", len(t.steps), last, t.done)
}

// relativeDir is a shared folder as the reader knows it: nothing when it is
// the project itself, its path inside the project when it is under it, the
// whole path otherwise.
func relativeDir(dir, root string) string {
	if root == "" {
		return dir
	}
	root = strings.TrimSuffix(root, "/") + "/"
	if dir == root {
		return ""
	}
	return strings.TrimPrefix(dir, root)
}

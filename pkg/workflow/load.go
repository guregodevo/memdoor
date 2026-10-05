package workflow

import (
	"encoding/json"
	"fmt"
	"memdoor/pkg/shared"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/guregodevo/mario/factory"
	"github.com/guregodevo/mario/fileio"
	"github.com/guregodevo/mario/static"
	"github.com/guregodevo/mario/tasks"
	"github.com/guregodevo/mario/templates"
)

// A workflow is a directory of mario task definitions — one YAML per task,
// at <dir>/<group>/<task>.yaml, the layout mario reads (project / dataset /
// table). The directory's name is the workflow's. Each file says its
// `type`; the registered factory for that type builds the task.
type Workflow struct {
	Name  string
	Dir   string                                   // absolute
	Defs  map[string]*templates.YamlTaskDefinition // by mario name, <workflow>.<group>.<task>
	Tasks []Task                                   // every task the DAG names, externals included, in name order
	// siblings are the definitions of external requirements that live in
	// another workflow beside this one (project_id): read for their TARGET,
	// never run here — mario's cross-DAG dependency through a data endpoint.
	siblings map[string]*templates.YamlTaskDefinition
	reg      factory.Component
}

// Task is one definition, read for a person: the graph the TUI draws, the
// row the CLI prints.
type Task struct {
	Name   string // short: the file's name, when no other group has one like it
	Full   string // <workflow>.<group>.<task>
	Type   string // agent, command, llm …
	Agent  string
	Model  string // which model answers the task's turn ("groq:qwen/qwen3.8-27b"); "" = the agent's ladder
	Budget int64  // tokens the task's turn may spend running on alone; 0 = the workspace's turn_token_budget
	// OutputSchema is the JSON Schema (as JSON) the task's answer must
	// match; the answer is kept as that JSON. "" = free text.
	OutputSchema string
	Prompt       string // as written; rendered at run time
	Requires     []string
	External     bool   // made outside the workflow: only checked, never run
	Target       string // "file …", "command: …", or "output" (what the task answers, kept as the proof)
	Retries      int
	Timeout      time.Duration
}

// Load reads the workflow at dir and validates every task against the
// schema of its type, as registered. A missing or empty directory is an
// error that names it; so is what mario refuses when it builds the graph (a
// cycle, a defined task marked external, a missing requirement).
func Load(dir string, reg factory.Component) (Workflow, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Workflow{}, err
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return Workflow{}, fmt.Errorf("no workflow at %s: it is a directory of <group>/<task>.yaml files", abs)
	}
	walkErr, defs := templates.Walk(&fileio.LocalFileIO{}, nil, factory.NewValidator(reg), abs, "", "", "Prompt")
	if walkErr != nil {
		return Workflow{}, fmt.Errorf("workflow %s: %w", filepath.Base(abs), walkErr)
	}
	wf := Workflow{Name: filepath.Base(abs), Dir: abs, Defs: map[string]*templates.YamlTaskDefinition{},
		siblings: map[string]*templates.YamlTaskDefinition{}, reg: reg}
	for _, d := range defs {
		if d.Type == TypeAgent && d.Agent == "" {
			d.Agent = defaultAgent
		}
		wf.Defs[d.Name] = d
	}
	if wf.Tasks, err = wf.tasks(); err != nil {
		return Workflow{}, err
	}
	// mario builds the graph once here, against a throwaway repository, so
	// what it refuses is refused when the files are read, not when the run
	// starts.
	if _, err := factory.BuildDAG("validate", "validate", component, static.NewWorkflowRepository(), wf.Defs, reg); err != nil {
		return Workflow{}, err
	}
	return wf, nil
}

// tasks lists the definitions and every external they require.
func (w Workflow) tasks() ([]Task, error) {
	byFull := map[string]Task{}
	for full, d := range w.Defs {
		t := taskOf(full, d)
		if d.Timeout != "" {
			to, err := tasks.ParseTimeout(d.Timeout)
			if err != nil {
				return nil, fmt.Errorf("task %s: timeout %q: %w", full, d.Timeout, err)
			}
			t.Timeout = to
		}
		for _, r := range d.Requires {
			t.Requires = append(t.Requires, r.Name())
			if _, defined := w.Defs[r.Name()]; defined {
				continue
			}
			if !r.External {
				return nil, fmt.Errorf("task %s requires %s, which has no definition and is not external", full, r.Name())
			}
			if _, seen := byFull[r.Name()]; !seen {
				byFull[r.Name()] = Task{Full: r.Name(), External: true, Target: w.targetOf(r.Name())}
			}
		}
		byFull[full] = t
	}
	short := map[string]int{}
	for full := range byFull {
		short[lastSegment(full)]++
	}
	out := make([]Task, 0, len(byFull))
	for full, t := range byFull {
		t.Name = full
		if short[lastSegment(full)] == 1 {
			t.Name = lastSegment(full)
		}
		for i, r := range t.Requires {
			if short[lastSegment(r)] == 1 {
				t.Requires[i] = lastSegment(r)
			}
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// taskOf reads a definition for a person.
func taskOf(full string, d *templates.YamlTaskDefinition) Task {
	return Task{Full: full, Type: d.Type, Agent: d.Agent, Model: d.Model, Budget: d.Budget, OutputSchema: outputSchemaJSON(d.OutputSchema), Prompt: d.Prompt, Retries: int(d.MaxRetries), Target: tasks.DescribeTarget(d)}
}

// targetOf reads a task's proof as a sentence: its own target, the target of
// its definition in a sibling workflow, or its output.
func (w Workflow) targetOf(full string) string {
	if d := w.definition(full); d != nil {
		return tasks.DescribeTarget(d)
	}
	return "output"
}

// definition is the task's own, or the sibling workflow's when the name
// points there (<project>/<group>/<task>.yaml beside this directory), read
// once and validated like any other.
func (w Workflow) definition(full string) *templates.YamlTaskDefinition {
	if d, ok := w.Defs[full]; ok {
		return d
	}
	if d, ok := w.siblings[full]; ok {
		return d
	}
	seg := strings.Split(full, ".")
	if len(seg) != 3 || seg[0] == w.Name {
		return nil
	}
	path := filepath.Join(filepath.Dir(w.Dir), seg[0], seg[1], seg[2]+".yaml")
	d, err := templates.ExtractYAML(&fileio.LocalFileIO{}, factory.NewValidator(w.reg), path)
	if err != nil || d == nil {
		return nil
	}
	d.Name = full
	w.siblings[full] = d
	return d
}

// Task finds one by its short or full name.
func (w Workflow) Task(name string) (Task, bool) {
	for _, t := range w.Tasks {
		if t.Name == name || t.Full == name {
			return t, true
		}
	}
	return Task{}, false
}

// Sinks are the tasks nothing requires: triggering them pulls the whole DAG.
func (w Workflow) Sinks() []string {
	required := map[string]bool{}
	for _, t := range w.Tasks {
		for _, r := range t.Requires {
			if rt, ok := w.Task(r); ok {
				required[rt.Full] = true
			}
		}
	}
	var sinks []string
	for _, t := range w.Tasks {
		if !required[t.Full] && !t.External {
			sinks = append(sinks, t.Full)
		}
	}
	return sinks
}

// WorkflowsDir is where a project keeps its workflows: .memdoor/workflows/
// <name>/<group>/<task>.yaml — under the dot directory Memdoor already owns
// in a project, so a repo that does not want them ignores one path.
func WorkflowsDir(workdir string) string {
	return filepath.Join(ProjectRoot(workdir), shared.MemdoorDirName, "workflows")
}

// ProjectRoot is the directory a workdir belongs to: workdir itself, or the
// nearest parent up to $HOME that has a .memdoor/ or is a git root (Greg,
// 2026-10-02: "they should also be looked up in current dir /.memdoor") — a
// TUI opened in a subdirectory still sees the project's workflows, and a
// run's outputs land at the project's root, not in the subdirectory. With
// none found, workdir.
func ProjectRoot(workdir string) string {
	home, _ := os.UserHomeDir()
	dir := workdir
	for {
		for _, mark := range []string{shared.MemdoorDirName, ".git"} {
			if _, err := os.Stat(filepath.Join(dir, mark)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir || dir == home || parent == home && home != "" {
			return workdir
		}
		dir = parent
	}
}

// SharedDir is the person's own library of workflows, ~/.memdoor/workflows:
// a DAG kept there runs in any project (Greg, 2026-10-02: "workflow can be
// reusable … do the shared library of workflows"). A project's own workflow
// of the same name wins.
func SharedDir() string {
	return shared.MemdoorHome("workflows")
}

// Find answers the directory of the workflow called name for a project:
// the project's own, then the shared library, then name as a path (absolute,
// or relative to the project) — a DAG written anywhere for one run.
func Find(workdir, name string) (string, error) {
	// The project's own first, then the library (shared.ProjectOrHome).
	if dir := shared.ProjectOrHome(ProjectRoot(workdir), "workflows", name); isWorkflowDir(dir) {
		return dir, nil
	}
	p := name
	if !filepath.IsAbs(p) {
		p = filepath.Join(workdir, p)
	}
	if isWorkflowDir(p) {
		return p, nil
	}
	return "", fmt.Errorf("no workflow %q: not in %s, not in %s, not a directory of <group>/<task>.yaml", name, WorkflowsDir(workdir), SharedDir())
}

// Entry is a workflow a project can run, with where it comes from and what
// its README says (the first paragraph) — so a shared one explains itself
// (Greg, 2026-10-02: "so users can share them, with a readme file").
type Entry struct {
	Name        string
	Dir         string
	Source      string // "project" or "library"
	Description string
}

// Describe lists the workflows a project can run, with their descriptions:
// the project's own, then the library's it does not shadow.
func Describe(workdir string) []Entry {
	var out []Entry
	seen := map[string]bool{}
	for _, src := range []struct{ root, source string }{{WorkflowsDir(workdir), "project"}, {SharedDir(), "library"}} {
		for _, n := range listWorkflows(src.root) {
			if seen[n] {
				continue
			}
			seen[n] = true
			dir := filepath.Join(src.root, n)
			out = append(out, Entry{Name: n, Dir: dir, Source: src.source, Description: readmeLead(dir)})
		}
	}
	return out
}

// readmeLead is the first paragraph of a workflow's README.md, one line.
func readmeLead(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		return ""
	}
	var lines []string
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			if len(lines) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(l, "#") {
			continue
		}
		lines = append(lines, l)
	}
	return strings.Join(lines, " ")
}

// Names lists the workflows a project can run: its own, then the shared
// library's that it does not shadow. Sorted within each.
func Names(workdir string) []string {
	own := listWorkflows(WorkflowsDir(workdir))
	seen := map[string]bool{}
	for _, n := range own {
		seen[n] = true
	}
	for _, n := range listWorkflows(SharedDir()) {
		if !seen[n] {
			own = append(own, n)
		}
	}
	return own
}

func listWorkflows(root string) []string {
	var names []string
	if root == "" {
		return nil
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if e.IsDir() && isWorkflowDir(filepath.Join(root, e.Name())) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// isWorkflowDir: a directory holding <group>/<task>.yaml.
func isWorkflowDir(dir string) bool {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return false
	}
	m, _ := filepath.Glob(filepath.Join(dir, "*", "*.y*ml"))
	return len(m) > 0
}

// OutputsDir is where a project keeps every task's output, by workflow,
// group, task and partition — mario's Outputs under the project.
func OutputsDir(workdir string) string {
	return filepath.Join(ProjectRoot(workdir), shared.MemdoorDirName, "runs")
}

const ignoreFile = "# written by memdoor: run outputs, the run table, the rules memdoor keeps and this machine's workspace are never committed; workflows/ is the project's — commit it\nruns/\nruns.db\nruns.db-*\nAGENTS.md\nworkspace\nworktrees/\npaste-*.png\n"

// ignoredPaths is what Memdoor keeps under .memdoor and never commits: the
// run outputs and the run table (sqlite, with its -wal and -shm), the kept
// rules, this machine's workspace marker (a slug, not the project's) and the
// worktrees. What is left — workflows/ — is the project's, so a person can
// commit .memdoor and every clone gets its workflows (Greg, 2026-10-04: "so
// that a user can have it in its git repo").
var ignoredPaths = []string{"runs/", "runs.db", "runs.db-*", "AGENTS.md", "workspace", "worktrees/", "paste-*.png"}

// pasteOnlyIgnore is what an image paste used to write: everything ignored,
// workflows/ included, so a project's workflows silently stopped being
// committable (2026-10-04). EnsureIgnored replaces exactly this file with
// the list above; any other file a person wrote is only appended to.
const pasteOnlyIgnore = "*\n"

// EnsureIgnored writes .memdoor/.gitignore in a project the first time
// outputs are kept there, so a run's outputs and its run table never land
// in a commit while .memdoor/workflows stays the person's to commit or not
// (Greg: "… and is git ignore"). An existing file keeps every line it has
// and gains the ones it lacks (the run table arrived after the first
// files were written, 2026-10-03); one that ignores everything (`*`) is
// left alone.
func EnsureIgnored(workdir string) {
	dir := filepath.Join(ProjectRoot(workdir), shared.MemdoorDirName)
	p := filepath.Join(dir, ".gitignore")
	existing, err := os.ReadFile(p)
	if err != nil {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return
		}
		_ = os.WriteFile(p, []byte(ignoreFile), 0o644)
		return
	}
	have := map[string]bool{}
	for _, l := range strings.Split(string(existing), "\n") {
		have[strings.TrimSpace(l)] = true
	}
	if string(existing) == pasteOnlyIgnore {
		_ = os.WriteFile(p, []byte(ignoreFile), 0o644)
		return
	}
	if have["*"] {
		return
	}
	add := ""
	for _, want := range ignoredPaths {
		if !have[want] {
			add += want + "\n"
		}
	}
	if add == "" {
		return
	}
	out := string(existing)
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	_ = os.WriteFile(p, []byte(out+add), 0o644)
}

// OutputPath is where a task's output lives — the proof of a task with no
// target of its own, and where an external's maker (a person approving)
// writes.
func OutputPath(workdir, partition, full string) string {
	return tasks.Outputs{Dir: OutputsDir(workdir)}.Path(full, partition)
}

func lastSegment(full string) string { return segment(full, -1) }

// segment answers the i-th dot-separated part of a mario name; -1 is the last.
func segment(full string, i int) string {
	seg := strings.Split(full, ".")
	if i < 0 {
		return seg[len(seg)-1]
	}
	if i < len(seg) {
		return seg[i]
	}
	return ""
}

// outputSchemaJSON is the task's output_schema as JSON text, "" when none.
func outputSchemaJSON(m map[string]interface{}) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(normalizeYAML(m))
	if err != nil {
		return ""
	}
	return string(b)
}

// normalizeYAML turns the map[interface{}]interface{} YAML may produce into
// JSON-encodable maps.
func normalizeYAML(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, e := range x {
			out[k] = normalizeYAML(e)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, e := range x {
			out[fmt.Sprint(k)] = normalizeYAML(e)
		}
		return out
	case []interface{}:
		for i, e := range x {
			x[i] = normalizeYAML(e)
		}
		return x
	}
	return v
}

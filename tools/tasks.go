package tools

import (
	"encoding/json"
	"fmt"
	"memdoor/pkg/shared"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// taskStore holds the DURABLE, SHARED todo list per coordination key (the
// channel). The planner and coder are different agents in the same channel, so
// keying by channel gives them ONE live list they both read and write across
// turns — the coordination spine of the plan → execute → follow-up loop. It's
// in-memory: the gateway is a single process and a task list is ephemeral to the
// work session; persistence to disk can come later if a run must survive restart.
var taskStore = struct {
	mu sync.RWMutex
	m  map[string][]TodoItem
}{m: map[string][]TodoItem{}}

// SetTasks replaces the todo list for a coordination key (channel) and
// persists it — a plan must survive a gateway restart (live: "continue the
// plan" after a dev-loop restart found "No task list yet" and the model
// replanned from scratch).
func SetTasks(key string, todos []TodoItem) {
	taskStore.mu.Lock()
	defer taskStore.mu.Unlock()
	taskStore.m[key] = todos
	if p := todoPath(key); p != "" {
		if b, err := json.Marshal(todos); err == nil {
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.WriteFile(p, b, 0o644)
		}
	}
}

// GetTasks returns the current todo list for a coordination key, falling back
// to the on-disk copy after a restart (nil if none anywhere).
func GetTasks(key string) []TodoItem {
	taskStore.mu.RLock()
	todos, ok := taskStore.m[key]
	taskStore.mu.RUnlock()
	if ok {
		return todos
	}
	p := todoPath(key)
	if p == "" {
		return nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var loaded []TodoItem
	if json.Unmarshal(b, &loaded) != nil {
		return nil
	}
	taskStore.mu.Lock()
	taskStore.m[key] = loaded
	taskStore.mu.Unlock()
	return loaded
}

var todoKeySanitizer = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func todoPath(key string) string {
	if key == "" {
		return ""
	}
	return shared.MemdoorHome("todos", todoKeySanitizer.ReplaceAllString(key, "_")+".json")
}

// FormatTodos renders the list (plain markers) plus the FOLLOW-UP directive: the
// single next action — finish the in-progress item, else start the next pending
// one — or, when all are completed, an explicit verify+done directive. This is
// what stops an agent from ending a turn with work still pending: the tool always
// names the next step, or declares the whole task done.
func FormatTodos(todos []TodoItem) string { return FormatTodosFor(todos, false) }

// planRuns records the run each key's plan was made in: the run whose
// todo_write brought items the list did not have. In memory — after a
// restart every saved plan is from an earlier request anyway.
var planRuns = struct {
	mu sync.Mutex
	m  map[string]string
}{m: map[string]string{}}

// NotePlanWrite records run as the plan's run when todos adds items the
// saved list lacks (a new plan); ticking or reordering the same items keeps
// the plan's original run.
func NotePlanWrite(key, run string, before, todos []TodoItem) {
	had := map[string]bool{}
	for _, t := range before {
		had[strings.TrimSpace(t.Content)] = true
	}
	fresh := len(before) == 0
	for _, t := range todos {
		if !had[strings.TrimSpace(t.Content)] {
			fresh = true
		}
	}
	if fresh {
		planRuns.mu.Lock()
		planRuns.m[key] = run
		planRuns.mu.Unlock()
	}
}

// PlanFromEarlierRun reports whether key's saved plan was made before run.
func PlanFromEarlierRun(key, run string) bool {
	planRuns.mu.Lock()
	defer planRuns.mu.Unlock()
	return planRuns.m[key] != run
}

// FormatTodosFor is FormatTodos for a plan that may predate the request being
// worked on. A finished plan from an earlier request must not end this one:
// live 2026-09-28 the coder carried the previous prompt's six steps into a
// new request, ticked them, was told "Nothing is left to do … STOP", and
// stopped with two of the new request's three asks undone.
func FormatTodosFor(todos []TodoItem, fromEarlierRequest bool) string {
	if len(todos) == 0 {
		return "No task list yet — create one with todo_write."
	}
	var lines []string
	var inProgress, nextPending string
	completed := 0
	for i, todo := range todos {
		marker := "[ ]"
		switch todo.Status {
		case "in_progress":
			marker = "[~]"
			if inProgress == "" {
				inProgress = todo.Content
			}
		case "completed":
			marker = "[x]"
			completed++
		default: // pending
			if nextPending == "" {
				nextPending = todo.Content
			}
		}
		lines = append(lines, fmt.Sprintf("%d. %s %s", i+1, marker, todo.Content))
	}
	total := len(todos)

	var followup string
	switch {
	case inProgress != "":
		followup = fmt.Sprintf("In progress: %s — finish it, then mark it completed.", inProgress)
	case nextPending != "":
		followup = fmt.Sprintf("Next: %s — start it now (do NOT stop here).", nextPending)
	default:
		// Agent-neutral: this message reaches EVERY agent that keeps a todo
		// list. It used to prescribe the coder's ending — "VERIFY (build/run)"
		// — and an agent without those tools went hunting for a build to run
		// (live 2026-09-02). Each agent's own prompt owns how it finishes.
		// TERMINAL wording. "verify the result the way your tools verify it"
		// sent an agent back to a check that succeeded, which it marked done,
		// which produced this line again — the check and todo_write
		// alternating every second for six minutes (live 2026-09-02). The
		// tools already verified; the only thing left is to say so and stop.
		followup = "All steps completed and already verified by the tools. Nothing is left to do: report what you did and STOP — do not call any more tools."
		if fromEarlierRequest {
			followup = "The saved plan is done, but it was made for an earlier request. Check that everything THIS request asks for is done; if something is not, write a new plan for it with todo_write and do it. Only then report and stop."
		}
	}
	return fmt.Sprintf("%s\n\n(%d/%d done) %s", strings.Join(lines, "\n"), completed, total, followup)
}

// CoerceTodos rescues the malformed todo_write calls a small model actually
// produces. Asked for a checklist it will send {"content":"a, b, c"}, or a
// plain string for "todos", or a list of bare strings — the intent is never in
// doubt, and rejecting it burns a round trip teaching a schema the model
// half-knows. Returns nil when there is genuinely nothing to read.
func CoerceTodos(raw json.RawMessage) []TodoItem {
	var loose struct {
		Todos   json.RawMessage `json:"todos"`
		Content json.RawMessage `json:"content"`
		Items   json.RawMessage `json:"items"`
		Steps   json.RawMessage `json:"steps"`
	}
	if err := json.Unmarshal(raw, &loose); err != nil {
		return nil
	}
	for _, field := range []json.RawMessage{loose.Todos, loose.Content, loose.Items, loose.Steps} {
		if len(field) == 0 {
			continue
		}
		// A list of bare strings.
		var strs []string
		if err := json.Unmarshal(field, &strs); err == nil && len(strs) > 0 {
			return todosFromStrings(strs)
		}
		// One string holding the whole list.
		var one string
		if err := json.Unmarshal(field, &one); err == nil && strings.TrimSpace(one) != "" {
			return todosFromStrings(splitSteps(one))
		}
	}
	return nil
}

// splitSteps breaks a single string into steps on the separators a model
// reaches for: newlines first, then commas or semicolons.
func splitSteps(s string) []string {
	sep := func(r rune) bool { return r == '\n' }
	if !strings.ContainsRune(s, '\n') {
		sep = func(r rune) bool { return r == ',' || r == ';' }
	}
	var out []string
	for _, part := range strings.FieldsFunc(s, sep) {
		part = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(part), "-*0123456789. "))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func todosFromStrings(steps []string) []TodoItem {
	var out []TodoItem
	for i, s := range steps {
		status := "pending"
		if i == 0 {
			status = "in_progress" // something has to be the live step
		}
		out = append(out, TodoItem{Content: s, Status: status, ActiveForm: s})
	}
	return out
}

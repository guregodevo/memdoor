package gateway

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// WHAT A STEP IS DOING (#94). The run panel showed a running step's last tool
// by name (`⏺ bash`): a person watching team mode could not tell which file
// the solver was editing or what a check said. Each tool call of a step's turn
// is now one line under the step: the file it read or changed, the command it
// ran and how that exited. A call's start, its beats and its end carry the
// same key, so the window redraws one line per call instead of adding three.

// stepCalls remembers what each running call of a run is about, from its
// start, for the beats and the end that carry only its id.
type stepCalls struct {
	mu   sync.Mutex
	what map[string]string
}

// line answers the key and the line for one tool event of a step's turn; ""
// when the event says nothing a person watching needs (a streamed chunk of
// output, a todo list, a spawned agent's own beat).
func (c *stepCalls) line(dir string, data map[string]interface{}) (string, string) {
	tool, _ := data["tool"].(string)
	if tool == "" {
		tool, _ = data["name"].(string)
	}
	id, _ := data["id"].(string)
	event, _ := data["event"].(string)
	if tool == "" || id == "" {
		return "", ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.what == nil {
		c.what = map[string]string{}
	}
	switch event {
	case "start":
		input, _ := data["input"].(string)
		what := stepCallLine(dir, tool, input)
		c.what[id] = what
		return id, what
	case toolEventProgress:
		what, ok := c.what[id]
		if !ok {
			return "", ""
		}
		if secs, ok := data["seconds"].(int); ok && secs >= 2 {
			return id, fmt.Sprintf("%s · %ds", what, secs)
		}
		return "", ""
	case "complete":
		what, ok := c.what[id]
		if !ok {
			return "", ""
		}
		delete(c.what, id)
		output, _ := data["output"].(string)
		errText, _ := data["error"].(string)
		return id, what + stepCallEnd(tool, output, errText)
	}
	return "", ""
}

// stepCallLine is a call as one line: `✎ path` for a change, `$ command` for
// bash, `read path`, `grep pattern`; the tool's name for any other.
func stepCallLine(dir, tool, input string) string {
	var in struct {
		Path     string `json:"path"`
		FilePath string `json:"file_path"`
		Command  string `json:"command"`
		Pattern  string `json:"pattern"`
		Search   string `json:"search"`
		Input    string `json:"input"`
	}
	_ = json.Unmarshal([]byte(input), &in)
	path := in.Path
	if path == "" {
		path = in.FilePath
	}
	path = relToRun(dir, path)
	switch tool {
	case "apply_patch":
		if files := patchFiles(in.Input); len(files) > 0 {
			for i := range files {
				files[i] = relToRun(dir, files[i])
			}
			return "✎ " + strings.Join(files, ", ")
		}
		return "✎ a patch"
	case "write_file", "edit_file":
		if path != "" {
			return "✎ " + path
		}
	case "search_replace":
		if in.Search != "" {
			return "✎ " + oneLine(in.Search, 60) + " in " + orDot(path)
		}
	case "bash":
		if c := strings.TrimSpace(in.Command); c != "" {
			first, _, more := strings.Cut(c, "\n")
			if more {
				first += " …"
			}
			return "$ " + first
		}
	case "read_file", "jread":
		if path != "" {
			return "read " + path
		}
	case "grep", "jgrep", "glob":
		if in.Pattern != "" {
			return tool + " " + in.Pattern
		}
	}
	return "⏺ " + tool
}

// stepCallEnd is how a call ended, after its line: a command's exit status;
// for any other tool only a failure.
func stepCallEnd(tool, output, errText string) string {
	if errText != "" {
		return " ✗ " + oneLine(errText, 80)
	}
	if tool != "bash" {
		return ""
	}
	if m := exitStatusRe.FindStringSubmatch(output); m != nil {
		return " → exit " + m[1]
	}
	if strings.HasPrefix(output, "Command FAILED (") {
		return " → failed"
	}
	return " → exit 0"
}

// exitStatusRe reads the status bash's result carries for a command that
// exited non-zero (tools.bashFailedOutput: "Command FAILED (exit status 2)…",
// or "(exit status 1. In a command that only reads…").
var exitStatusRe = regexp.MustCompile(`\(exit status (\d+)`)

// patchFiles are the files an apply_patch input adds, updates or deletes.
func patchFiles(patch string) []string {
	var out []string
	for _, l := range strings.Split(patch, "\n") {
		for _, mk := range []string{"*** Add File: ", "*** Update File: ", "*** Delete File: "} {
			if f, ok := strings.CutPrefix(strings.TrimSpace(l), mk); ok && strings.TrimSpace(f) != "" {
				out = append(out, strings.TrimSpace(f))
			}
		}
	}
	return out
}

// relToRun is a path as the person reads it: relative to the run's project
// when it is inside it.
func relToRun(dir, path string) string {
	if path == "" || dir == "" || !filepath.IsAbs(path) {
		return path
	}
	if rel, err := filepath.Rel(dir, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

func orDot(path string) string {
	if path == "" {
		return "."
	}
	return path
}

// stepOutcome is the line a finished step leaves under it: the file it owed
// and, when that file opens with one, its verdict (`review.md: verdict:
// PASS`). "" for a step with no file target, or one whose file is not there.
func stepOutcome(dir, partition, name, target string) string {
	f, ok := strings.CutPrefix(target, "file ")
	if !ok {
		return ""
	}
	f = renderItem(renderPartition(strings.TrimSpace(f), partition), name)
	if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
		return ""
	}
	if v := fileVerdict(filepath.Join(dir, f)); v != "" {
		return filepath.Base(f) + ": verdict: " + v
	}
	return "wrote " + filepath.Base(f)
}

// renderItem fills {{.item}} in a target of a task made by `each:`: mario
// names each copy <name>-<item> (templates.Expand).
func renderItem(target, name string) string {
	if !strings.Contains(target, ".item") {
		return target
	}
	i := strings.LastIndex(name, "-")
	if i < 0 {
		return target
	}
	item := name[i+1:]
	for _, ph := range []string{"{{.item}}", "{{ .item }}"} {
		target = strings.ReplaceAll(target, ph, item)
	}
	return target
}

// fileVerdict is PASS or FAIL from the first `verdict:` line in a file's
// first ten, markdown emphasis aside, the way team-solve's verdict.sh reads
// it; "" when there is none.
func fileVerdict(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.SplitN(string(data), "\n", 11)
	if len(lines) > 10 {
		lines = lines[:10]
	}
	for _, l := range lines {
		l = strings.Map(func(r rune) rune {
			if r == '*' || r == '`' || r == '_' {
				return -1
			}
			return r
		}, l)
		if m := verdictLineRe.FindStringSubmatch(l); m != nil {
			return strings.ToUpper(m[1])
		}
	}
	return ""
}

var verdictLineRe = regexp.MustCompile(`(?i)^\s*verdict:\s*(PASS|FAIL)\b`)

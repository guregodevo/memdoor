package gateway

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// THE AGENT CANNOT EDIT ITS OWN CHECK (Greg, 2026-10-24: "fix the hole, the
// agent can't edit its own check"). Measuring what a retry loop costs, a task
// whose target was `command: ./never.sh`, a script that always exits 1, was
// marked done on its third attempt: the agent rewrote never.sh to exit 0 and
// the engine ran it. "A step is done when its check passes, never because the
// model says so" is false while the check is a file the task may write.
//
// So: when a task starts, the programs its target command runs that live in
// the project (`./check.sh`, `sh scripts/verify.sh`) are taken as they are,
// bytes and mode, onto the task's session. The writing tools refuse a call
// that names one, with the reason. After every tool call, whatever changed
// one of them anyway (a shell redirect, a patch, a server) is put back and
// the model is told. The check that runs is then always the one declared.
// Not covered, on purpose: what the check itself depends on (the tests
// `go test` runs, a Makefile's recipe) is the task's own work.

const targetProgramsKey = "done_when_programs"

// targetProgramMax is the largest program kept; a check script is small.
const targetProgramMax = 1 << 20

type targetProgram struct {
	Path string `json:"path"` // relative to the project
	Hash string `json:"hash"`
	Mode uint32 `json:"mode"`
	Body []byte `json:"body"`
}

// interpreters run the file named by their first argument.
var interpreters = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "python": true, "python3": true, "node": true, "ruby": true, "perl": true, "deno": true, "bun": true}

// targetPrograms names the project files a target command runs: the first
// word of each command in it, or the file an interpreter is given, when
// that resolves to a regular file under dir.
func targetPrograms(command, dir string) []string {
	seen := map[string]bool{}
	var out []string
	for _, seg := range splitCommands(command) {
		words := strings.Fields(seg)
		if len(words) == 0 {
			continue
		}
		candidates := []string{words[0]}
		if interpreters[filepath.Base(words[0])] {
			for _, w := range words[1:] {
				if !strings.HasPrefix(w, "-") {
					candidates = append(candidates, w)
					break
				}
			}
		}
		for _, c := range candidates {
			rel, ok := projectFile(c, dir)
			if ok && !seen[rel] {
				seen[rel] = true
				out = append(out, rel)
			}
		}
	}
	sort.Strings(out)
	return out
}

// splitCommands cuts a shell line at its command boundaries.
func splitCommands(command string) []string {
	r := strings.NewReplacer("&&", "\n", "||", "\n", ";", "\n", "|", "\n", "(", "\n", ")", "\n", "$(", "\n")
	return strings.Split(r.Replace(command), "\n")
}

// projectFile resolves a word to a regular file under dir, as a path
// relative to it; a bare name (`test`, `make`) is a program on PATH, not
// in the project, unless it is a file right there.
func projectFile(word, dir string) (string, bool) {
	word = strings.Trim(word, `"'`)
	if word == "" || strings.ContainsAny(word, "*?[{}$`") {
		return "", false
	}
	abs := word
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(dir, word)
	}
	abs = filepath.Clean(abs)
	rel, err := filepath.Rel(filepath.Clean(dir), abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	st, err := os.Stat(abs)
	if err != nil || !st.Mode().IsRegular() {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// snapshotTargetPrograms keeps the target's programs as they are now on the
// session: the check the task is done by. A target with none clears it.
func snapshotTargetPrograms(session *Session, command, dir string) []string {
	var progs []targetProgram
	for _, rel := range targetPrograms(command, dir) {
		abs := filepath.Join(dir, filepath.FromSlash(rel))
		st, err := os.Stat(abs)
		if err != nil || st.Size() > targetProgramMax {
			continue
		}
		body, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		progs = append(progs, targetProgram{Path: rel, Hash: hashBytes(body), Mode: uint32(st.Mode().Perm()), Body: body})
	}
	if len(progs) == 0 {
		session.SetMetadata(targetProgramsKey, "")
		return nil
	}
	b, _ := json.Marshal(progs)
	session.SetMetadata(targetProgramsKey, string(b))
	names := make([]string, 0, len(progs))
	for _, p := range progs {
		names = append(names, p.Path)
	}
	return names
}

// targetProgramsOf reads the kept programs, none without a task target.
func targetProgramsOf(session *Session) []targetProgram {
	if session == nil {
		return nil
	}
	v, ok := session.GetMetadataValue(targetProgramsKey)
	if !ok {
		return nil
	}
	raw, _ := v.(string)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var progs []targetProgram
	if json.Unmarshal([]byte(raw), &progs) != nil {
		return nil
	}
	return progs
}

// writingTools change files by name.
var writingTools = map[string]bool{"write_file": true, "edit_file": true, "apply_patch": true, "search_replace": true}

// targetGuardRefuses is the teaching error for a writing tool that names
// one of the task's check programs, "" when the call may run.
func targetGuardRefuses(session *Session, tool string, input []byte) string {
	if !writingTools[tool] {
		return ""
	}
	in := string(input)
	for _, p := range targetProgramsOf(session) {
		if namesPath(in, p.Path) {
			return targetGuardMessage(tool, []string{p.Path}, false)
		}
	}
	return ""
}

// namesPath reports whether text names the file: the path, or its base
// name as a path component ("./never.sh", "never.sh", "/srv/app/never.sh").
func namesPath(text, rel string) bool {
	if strings.Contains(text, rel) {
		return true
	}
	base := filepath.Base(rel)
	for _, prefix := range []string{"./", "/", `"`, " ", "'"} {
		if strings.Contains(text, prefix+base) {
			return true
		}
	}
	return false
}

func targetGuardMessage(tool string, paths []string, restored bool) string {
	what := strings.Join(paths, ", ")
	if restored {
		return fmt.Sprintf("%s changed %s: that is the check this task is done by, and it was put back as it was. The task may not change its own check; make the check pass instead.", tool, what)
	}
	return fmt.Sprintf("%s is the check this task is done by: the task may not change it. Make the check pass instead.", what)
}

// restoreTargetPrograms puts back every kept program that no longer matches,
// and names them. The check runs as declared whatever the task did.
func restoreTargetPrograms(session *Session, dir string) []string {
	var changed []string
	for _, p := range targetProgramsOf(session) {
		abs := filepath.Join(dir, filepath.FromSlash(p.Path))
		body, err := os.ReadFile(abs)
		if err == nil && hashBytes(body) == p.Hash {
			if st, err := os.Stat(abs); err == nil && uint32(st.Mode().Perm()) == p.Mode {
				continue
			}
		}
		_ = os.MkdirAll(filepath.Dir(abs), 0o755)
		if err := os.WriteFile(abs, p.Body, os.FileMode(p.Mode)); err != nil {
			continue
		}
		_ = os.Chmod(abs, os.FileMode(p.Mode))
		changed = append(changed, p.Path)
	}
	return changed
}

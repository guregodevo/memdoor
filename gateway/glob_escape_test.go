package gateway

import (
	"encoding/json"
	"testing"
)

const cwd = "/Users/you/memdoor-coder"

func pathOf(t *testing.T, tool, input string) string {
	t.Helper()
	out := confineToCoderWorkdir(tool, json.RawMessage(input), cwd)
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	p, _ := got["path"].(string)
	return p
}

// An ABSENT path is the dangerous case, not a wrong one.
//
// glob and grep take `path` as OPTIONAL, documented "default: current
// directory" — and that is the GATEWAY's cwd, which in development is the
// memdoor repository itself. Confinement rewrote only the paths that were
// present, so an omitted one escaped.
//
// Observed 2026-08-30 on a live coder turn: the model called
// glob("**/*_test.go") with no path and got 299 lines of memdoor's own source
// (gateway/rag/, pkg/domain/, pkg/shared/), none of it in its workdir. It then
// lost the task and never edited the file it had been asked about — which had
// looked like a prompt-phrasing problem until the transcript showed otherwise.
func TestGlobWithoutPathIsConfined(t *testing.T) {
	if got := pathOf(t, "glob", `{"pattern":"**/*_test.go"}`); got != cwd {
		t.Errorf("glob with no path must search the workdir, got %q — "+
			"an empty path means the gateway's cwd, which is this repo", got)
	}
}

func TestGrepWithoutPathIsConfined(t *testing.T) {
	if got := pathOf(t, "grep", `{"pattern":"func main"}`); got != cwd {
		t.Errorf("grep with no path must search the workdir, got %q", got)
	}
}

func TestListFilesWithoutPathIsConfined(t *testing.T) {
	if got := pathOf(t, "list_files", `{}`); got != cwd {
		t.Errorf("list_files with no path must list the workdir, got %q", got)
	}
}

// A path the model DID supply keeps its existing treatment: relative paths are
// joined to the workdir, absolute ones are left alone (the deliberate "may
// target a real project on purpose" behaviour).
func TestSuppliedPathsKeepTheirExistingTreatment(t *testing.T) {
	if got := pathOf(t, "glob", `{"pattern":"*.go","path":"internal"}`); got != cwd+"/internal" {
		t.Errorf("a relative path must be joined to the workdir, got %q", got)
	}
	if got := pathOf(t, "glob", `{"pattern":"*.go","path":"/tmp/elsewhere"}`); got != "/tmp/elsewhere" {
		t.Errorf("an absolute path stays as given, got %q", got)
	}
}

// The injection must NOT apply to tools whose path is required — a write_file
// with no path is a malformed call, and silently pointing it at the workdir
// root would turn it into a write to a directory.
func TestRequiredPathToolsAreNotGivenADefault(t *testing.T) {
	out := confineToCoderWorkdir("write_file", json.RawMessage(`{"content":"x"}`), cwd)
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	if _, has := got["path"]; has {
		t.Error("write_file with no path is malformed; inventing one would make it write to a directory")
	}
}

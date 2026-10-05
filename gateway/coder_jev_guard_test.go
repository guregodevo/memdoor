package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

// The jev tools read like grep and read_file do, and reads go anywhere since
// 2026-10-05 ("like omp"); what they still share with the plain readers is
// the ROOT: with no path they search the workdir, not the gateway's cwd.
func TestJevToolsReadAnywhereAndRootInTheWorkdir(t *testing.T) {
	wd := "/Users/me/memdoor-coder"
	for _, tool := range []string{"jgrep", "jread"} {
		for _, path := range []string{"/Users/me/Dev/secret/repo", "/Users/me/memdoor-coder/src"} {
			in := json.RawMessage(`{"task":"t","pattern":"x","path":"` + path + `"}`)
			if msg := coderPathEscape(tool, in, wd); msg != "" {
				t.Errorf("%s may read %s, got %q", tool, path, msg)
			}
		}
	}
	out := confineToCoderWorkdir("jgrep", json.RawMessage(`{"task":"t","pattern":"x"}`), wd)
	if !strings.Contains(string(out), wd) {
		t.Errorf("jgrep with no path must search the workdir, not the gateway's cwd: %s", out)
	}
}

package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

// An absolute WRITE outside the coder workdir is refused with an instructive
// message (the stray-main.go incidents); a READ anywhere is allowed (Greg,
// 2026-10-05, "like omp": oh-my-pi confines no read, and bash could read the
// same file anyway). Absolute paths INSIDE the workdir and relative paths
// stay allowed — the rewrite layer roots the relative ones.
func TestCoderPathEscape(t *testing.T) {
	wd := "/Users/dev/memdoor-coder"

	if msg := coderPathEscape("write_file", json.RawMessage(`{"file_path":"/etc/passwd","content":"x"}`), wd); msg == "" {
		t.Error("outside-workdir absolute write_file path should be refused")
	} else if !strings.Contains(msg, wd) {
		t.Errorf("the refusal should name the workdir: %q", msg)
	}
	if msg := coderPathEscape("edit_file", json.RawMessage(`{"file_path":"/Users/dev/repo/main.go","old_string":"a","new_string":"b"}`), wd); msg == "" {
		t.Error("outside-workdir absolute edit_file path should be refused")
	}
	for _, tool := range []string{"read_file", "grep", "glob", "jgrep", "jread", "list_files"} {
		if msg := coderPathEscape(tool, json.RawMessage(`{"path":"/Users/dev/repo/secret.go"}`), wd); msg != "" {
			t.Errorf("%s may read anywhere, got refusal %q", tool, msg)
		}
	}
	// Prefix trickery: /Users/dev/memdoor-coder-evil is NOT inside the workdir.
	if msg := coderPathEscape("write_file", json.RawMessage(`{"path":"/Users/dev/memdoor-coder-evil/x.go","content":"x"}`), wd); msg == "" {
		t.Error("sibling dir sharing the workdir prefix should be refused")
	}

	for name, input := range map[string]string{
		"absolute inside workdir":  `{"path":"/Users/dev/memdoor-coder/main.go"}`,
		"relative":                 `{"path":"main.go"}`,
		"traversal-cleaned inside": `{"path":"/Users/dev/memdoor-coder/sub/../main.go"}`,
		"no path key":              `{"pattern":"*.go"}`,
	} {
		if msg := coderPathEscape("write_file", json.RawMessage(input), wd); msg != "" {
			t.Errorf("%s should be allowed, got refusal %q", name, msg)
		}
	}
	// Cleaned traversal OUT of the workdir is an escape.
	if msg := coderPathEscape("write_file", json.RawMessage(`{"path":"/Users/dev/memdoor-coder/../repo/x.go","content":"x"}`), wd); msg == "" {
		t.Error("dot-dot traversal out of the workdir should be refused")
	}
	// Non-file tools and empty workdir are out of scope.
	if msg := coderPathEscape("bash", json.RawMessage(`{"command":"ls /"}`), wd); msg != "" {
		t.Errorf("bash is fenced elsewhere, got %q", msg)
	}
	if msg := coderPathEscape("write_file", json.RawMessage(`{"path":"/anywhere/x.go","content":"x"}`), ""); msg != "" {
		t.Errorf("no workdir means no fence, got %q", msg)
	}
}

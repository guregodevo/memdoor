package gateway

import (
	"encoding/json"
	"memdoor/internal/testenv"
	"strings"
	"testing"
)

// An absolute WRITE outside the coder workdir is refused with an instructive
// message (the stray-main.go incidents); a READ anywhere is allowed (the maintainer,
// 2026-10-05, "like omp": oh-my-pi confines no read, and bash could read the
// same file anyway). Absolute paths INSIDE the workdir and relative paths
// stay allowed — the rewrite layer roots the relative ones.
func TestCoderPathEscape(t *testing.T) {
	wd := testenv.Abs("/Users/dev/memdoor-coder")
	at := func(key, posix string) json.RawMessage {
		in, _ := json.Marshal(map[string]string{key: testenv.Abs(posix), "content": "x"})
		return in
	}

	if msg := coderPathEscape("write_file", at("file_path", "/etc/passwd"), wd); msg == "" {
		t.Error("outside-workdir absolute write_file path should be refused")
	} else if !strings.Contains(msg, wd) {
		t.Errorf("the refusal should name the workdir: %q", msg)
	}
	if msg := coderPathEscape("edit_file", at("file_path", "/Users/dev/repo/main.go"), wd); msg == "" {
		t.Error("outside-workdir absolute edit_file path should be refused")
	}
	for _, tool := range []string{"read_file", "grep", "glob", "jgrep", "jread", "list_files"} {
		if msg := coderPathEscape(tool, at("path", "/Users/dev/repo/secret.go"), wd); msg != "" {
			t.Errorf("%s may read anywhere, got refusal %q", tool, msg)
		}
	}
	// Prefix trickery: /Users/dev/memdoor-coder-evil is NOT inside the workdir.
	if msg := coderPathEscape("write_file", at("path", "/Users/dev/memdoor-coder-evil/x.go"), wd); msg == "" {
		t.Error("sibling dir sharing the workdir prefix should be refused")
	}

	for name, input := range map[string]json.RawMessage{
		"absolute inside workdir":  at("path", "/Users/dev/memdoor-coder/main.go"),
		"relative":                 json.RawMessage(`{"path":"main.go"}`),
		"traversal-cleaned inside": at("path", "/Users/dev/memdoor-coder/sub/../main.go"),
		"no path key":              json.RawMessage(`{"pattern":"*.go"}`),
	} {
		if msg := coderPathEscape("write_file", input, wd); msg != "" {
			t.Errorf("%s should be allowed, got refusal %q", name, msg)
		}
	}
	// Cleaned traversal OUT of the workdir is an escape.
	if msg := coderPathEscape("write_file", at("path", "/Users/dev/memdoor-coder/../repo/x.go"), wd); msg == "" {
		t.Error("dot-dot traversal out of the workdir should be refused")
	}
	// Non-file tools and empty workdir are out of scope.
	if msg := coderPathEscape("bash", json.RawMessage(`{"command":"ls /"}`), wd); msg != "" {
		t.Errorf("bash is fenced elsewhere, got %q", msg)
	}
	if msg := coderPathEscape("write_file", at("path", "/anywhere/x.go"), ""); msg != "" {
		t.Errorf("no workdir means no fence, got %q", msg)
	}
}

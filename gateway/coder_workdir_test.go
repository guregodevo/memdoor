package gateway

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestConfineToCoderWorkdir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	wd := filepath.Join(home, "memdoor-coder")

	field := func(in json.RawMessage, key string) string {
		var m map[string]any
		_ = json.Unmarshal(in, &m)
		s, _ := m[key].(string)
		return s
	}

	// Relative write_file path -> under the workspace (NOT the gateway cwd).
	out := confineToCoderWorkdir("write_file", json.RawMessage(`{"path":"main.go","content":"x"}`), coderWorkdir())
	if got := field(out, "path"); got != filepath.Join(wd, "main.go") {
		t.Fatalf("relative path should resolve under workspace, got %q", got)
	}

	// The REWRITE leaves absolute paths alone — but an absolute path outside
	// the workdir never reaches a tool: coderPathEscape refuses it at the call
	// site (see TestCoderPathEscape), the same split as bash's cd fence.
	abs := `{"file_path":"/tmp/elsewhere/x.go","content":"y"}`
	if got := field(confineToCoderWorkdir("edit_file", json.RawMessage(abs), coderWorkdir()), "file_path"); got != "/tmp/elsewhere/x.go" {
		t.Fatalf("absolute path must be unchanged by the rewrite, got %q", got)
	}

	// bash runs in the workspace — via cwd, not a cd prefix.
	//
	// This used to assert `cd '<wd>' && <cmd>`. That expressed the workdir to a
	// SHELL, which the model's own cd could then override, and on 2026-08-29 a
	// model did exactly that. The workdir now travels as cwd — the same
	// field apply_patch, locate and skill already take — and the tool applies
	// it with cmd.Dir, where the operating system enforces it.
	bashOut := confineToCoderWorkdir("bash", json.RawMessage(`{"command":"go run main.go"}`), coderWorkdir())
	if b := field(bashOut, "command"); b != "go run main.go" {
		t.Fatalf("the command must be passed through unrewritten, got %q", b)
	}
	if c := field(bashOut, "cwd"); c != coderWorkdir() {
		t.Fatalf("bash must receive the workdir as cwd, got %q want %q", c, coderWorkdir())
	}
}

package tools

import (
	"encoding/json"
	"testing"
)

// A small model nests the real command under "parameters" and leaves the tool name
// in "command": {"command":"bash","parameters":{"command":"go build ./..."}}. The
// parser must recover the real command, not run the literal "bash".
func TestParseBashCommandUnwrapsNestedParameters(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", `{"command":"go build ./..."}`, "go build ./..."},
		{"nested under parameters", `{"command":"bash","parameters":{"command":"go build ./... && go run tetris.go"}}`, "go build ./... && go run tetris.go"},
		{"nested under input", `{"command":"bash","input":{"command":"go test ./..."}}`, "go test ./..."},
		{"empty top-level, nested", `{"command":"","parameters":{"command":"ls -la"}}`, "ls -la"},
		{"legit bash invocation kept", `{"command":"bash script.sh"}`, "bash script.sh"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseBashCommand(json.RawMessage(c.in))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseBashCommandMissing(t *testing.T) {
	if _, err := ParseBashCommand(json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error for missing command")
	}
}

// Live 2026-08-16 loop: {"command":"go","path":"tetris.go"} ran bare `go`
// (usage, exit 2) four times straight. Split commands reassemble VERBATIM —
// no language-specific rewriting; a still-wrong command self-corrects off the
// failure output, which echoes the exact command line.
func TestParseBashCommandAbsorbsSplitFields(t *testing.T) {
	for in, want := range map[string]string{
		`{"command":"go","path":"tetris.go"}`:     "go tetris.go",
		`{"command":"python3","file":"x.py"}`:     "python3 x.py",
		`{"command":"ls","args":"-la"}`:           "ls -la",
		`{"command":"go build ./...","path":"z"}`: "go build ./...", // multi-word command: extras ignored
		`{"command":"go test ./..."}`:             "go test ./...",
	} {
		got, err := ParseBashCommand(json.RawMessage(in))
		if err != nil || got != want {
			t.Fatalf("%s → %q (err %v), want %q", in, got, err, want)
		}
	}
}

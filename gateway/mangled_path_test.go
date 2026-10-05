package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

// A path the model mangled is not a path.
//
// When a small model emits a malformed tool call, its arguments can arrive as a
// fragment of the tool SCHEMA — and filepath.Join will happily turn that into a
// filename. Found 2026-08-30 in the coder workdir, dated Jul 27: a zero-byte
// file named
//
//	greetname.go and the full corrected content", "Fix the patch to correctly
//	modify the function"], "default": "Fix the patch to correctly modify ...
//
// which is JSON out of apply_patch's own schema. It sat there for over a month
// polluting every `ls` the coder ran, which is eventually how it was noticed.
func TestMangledPathIsRefusedNotCreated(t *testing.T) {
	mangled := `greetname.go and the full corrected content", "Fix the patch to correctly modify the function"], "default": "Fix it`
	in, _ := json.Marshal(map[string]any{"file_path": mangled, "content": "x"})

	out := confineToCoderWorkdir("write_file", in, "/Users/you/memdoor-coder")
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if p, _ := got["file_path"].(string); p != "" {
		t.Errorf("a schema fragment must not become a filename, got %q", p)
	}
}

// Narrow on purpose: spaces are legal in filenames and a great many real
// projects use them. Rejecting broadly would start refusing paths people
// actually have.
func TestOrdinaryPathsAreUntouched(t *testing.T) {
	wd := "/Users/you/memdoor-coder"
	for _, p := range []string{
		"stats.go",
		"internal/server/main.go",
		"My Documents/notes.md", // spaces are fine
		"a-b_c.123.go",
	} {
		in, _ := json.Marshal(map[string]any{"file_path": p})
		out := confineToCoderWorkdir("read_file", in, wd)
		var got map[string]any
		_ = json.Unmarshal(out, &got)
		g, _ := got["file_path"].(string)
		if g == "" {
			t.Errorf("%q is a legitimate path and must not be refused", p)
		}
		if !strings.HasPrefix(g, wd) {
			t.Errorf("%q must still be confined to the workdir, got %q", p, g)
		}
	}
}

// Newlines are the other marker of a mangled argument — a multi-line "path" is
// a tool call that lost its structure.
func TestMultilinePathRefused(t *testing.T) {
	in, _ := json.Marshal(map[string]any{"file_path": "good.go\nand then some prose"})
	out := confineToCoderWorkdir("write_file", in, "/wd")
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	if p, _ := got["file_path"].(string); p != "" {
		t.Errorf("a multi-line path must be refused, got %q", p)
	}
}

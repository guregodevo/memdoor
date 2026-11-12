package main

import (
	"memdoor/internal/testenv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/gateway/logs"
)

// The two arms must differ the way the coder's do: under hashline the
// apply_patch description is the line-anchored one and reads come back
// tagged and numbered; under patch neither.
func TestArmsUseTheToolsPackage(t *testing.T) {
	testenv.Logger(t)
	// Windows will not remove a directory holding events.db, so the store this
	// test opened is closed before its temporary home goes (#283).
	t.Cleanup(func() { _ = logs.CloseGlobalLogger() })
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	desc := map[string]string{}
	for _, format := range []string{"patch", "hashline"} {
		for _, s := range toolsFor(format) {
			if s.Function.Name == "apply_patch" {
				desc[format] = s.Function.Description
			}
		}
		out, err := runTool(dir, "read_file", `{"path":"a.go"}`)
		if err != nil {
			t.Fatal(err)
		}
		if tagged := strings.HasPrefix(out, "[a.go#"); tagged != (format == "hashline") {
			t.Fatalf("%s read: %q", format, out)
		}
	}
	if desc["patch"] == "" || desc["patch"] == desc["hashline"] {
		t.Fatal("the two arms must describe apply_patch differently")
	}
}

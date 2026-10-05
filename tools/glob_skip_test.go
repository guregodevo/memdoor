package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// glob never lists VCS internals, dependency trees or Memdoor's own state,
// whatever the pattern — `**/*` in a project with a .git returned ~400
// paths of .git/objects and hook samples (live 2026-10-05).
func TestGlobSkipsWhatGrepSkips(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{".git/objects/ab/cdef", ".git/hooks/pre-push.sample", "node_modules/x/index.js", ".memdoor/runs.db", "src/a.go", "docs/b.sample"} {
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for pattern, want := range map[string][]string{
		"**/*":        {"src/a.go", "docs/b.sample"},
		"**/*.sample": {"docs/b.sample"},
		"**/*.js":     nil,
	} {
		in, _ := json.Marshal(map[string]string{"pattern": pattern, "path": dir})
		out, err := Glob(in)
		if err != nil {
			t.Fatalf("%s: %v", pattern, err)
		}
		for _, bad := range []string{".git", "node_modules", ".memdoor"} {
			if strings.Contains(out, bad) {
				t.Errorf("%s lists %s:\n%s", pattern, bad, out)
			}
		}
		for _, w := range want {
			if !strings.Contains(out, w) {
				t.Errorf("%s misses %s:\n%s", pattern, w, out)
			}
		}
	}
}

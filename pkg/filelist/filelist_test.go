package filelist

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range []string{"main.go", "cmd/run.go", "docs/readme.md", ".git/HEAD", "node_modules/x/index.js", ".hidden/secret"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("line one\nline two\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// The listing is what a person would mention: shallow first, folders with a
// slash, nothing hidden and no dependency trees.
func TestListIsShallowFirstWithoutHiddenOrVendored(t *testing.T) {
	got := List(project(t))
	// Depth first (a slash counts, so a top-level folder sorts with its
	// children's level), then name: the order the picker has always shown.
	want := []string{"main.go", "cmd/", "cmd/run.go", "docs/", "docs/readme.md"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("list: %v, want %v", got, want)
	}
}

func TestMatchRanksBasenamePrefixFirst(t *testing.T) {
	files := []string{"main.go", "cmd/", "cmd/run.go", "docs/", "docs/readme.md"}
	if got := Match(files, "r", 10); got[0] != "cmd/run.go" || got[1] != "docs/readme.md" {
		t.Fatalf("match r: %v", got)
	}
	if got := Match(files, "", 2); len(got) != 2 {
		t.Fatalf("empty query caps at limit: %v", got)
	}
}

func TestPreviewNumbersLinesAndListsFolders(t *testing.T) {
	root := project(t)
	lines := Preview(root, "main.go", false)
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "   1  line one") {
		t.Fatalf("file preview: %q", lines)
	}
	if got := Preview(root, "cmd/", false); len(got) != 1 || got[0] != "run.go" {
		t.Fatalf("folder preview: %q", got)
	}
}

func TestStatExpandsHomeAndSaysDirectory(t *testing.T) {
	root := project(t)
	abs, isDir, exists := Stat(root)
	if !exists || !isDir || abs != filepath.Clean(root) {
		t.Fatalf("stat dir: %s %v %v", abs, isDir, exists)
	}
	if _, isDir, exists := Stat(filepath.Join(root, "main.go")); !exists || isDir {
		t.Fatalf("stat file: dir=%v exists=%v", isDir, exists)
	}
	if _, _, exists := Stat(filepath.Join(root, "nowhere")); exists {
		t.Fatal("stat missing: exists")
	}
	home, _ := os.UserHomeDir()
	if abs, _, _ := Stat("~"); abs != filepath.Clean(home) {
		t.Fatalf("~ is %s, want %s", abs, home)
	}
}

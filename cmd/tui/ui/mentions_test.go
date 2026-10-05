package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMentionAt(t *testing.T) {
	for _, c := range []struct {
		in    string
		start int
		q     string
		ok    bool
	}{
		{"@", 0, "", true},
		{"fix @cmd/ma", 4, "cmd/ma", true},
		{"see @a.go and", 0, "", false}, // a space after the mention closes it
		{"mail me@x", 0, "", false},     // not preceded by whitespace
		{"line one\n@sr", 9, "sr", true},
		{"no mention", 0, "", false},
	} {
		start, q, ok := mentionAt(c.in)
		if start != c.start || q != c.q || ok != c.ok {
			t.Errorf("%q: got (%d,%q,%v) want (%d,%q,%v)", c.in, start, q, ok, c.start, c.q, c.ok)
		}
	}
	if got := insertMention("fix @cmd/ma", 4, "cmd/main.go"); got != "fix @cmd/main.go " {
		t.Fatalf("insert: %q", got)
	}
	if v, cont := newlineOnBackslash(`first line\`); !cont || v != "first line\n" {
		t.Fatalf("backslash continuation: %q %v", v, cont)
	}
	if _, cont := newlineOnBackslash("plain"); cont {
		t.Fatal("no backslash, no continuation")
	}
}

func TestFileListAndMatching(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"main.go", "cmd/app/main.go", "pkg/store/store.go", "pkg/store/store_test.go", "node_modules/x/index.js", ".git/HEAD", "vendor/y.go", "docs/README.md"} {
		os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755)
		os.WriteFile(filepath.Join(root, p), []byte("x"), 0o644)
	}
	files := loadFileList(root)
	for _, f := range files {
		for _, bad := range []string{"node_modules", ".git", "vendor"} {
			if len(f) >= len(bad) && (f[:len(bad)] == bad) {
				t.Fatalf("listed %s", f)
			}
		}
	}
	if len(files) != 10 || files[0] != "main.go" {
		t.Fatalf("shallow first, folders in with a slash, dependencies skipped: %v", files)
	}
	got := matchFiles(files, "store", 10)
	if len(got) != 3 || got[0] != "pkg/store/" || got[1] != "pkg/store/store.go" {
		t.Fatalf("basename prefix first (the folder too), shorter first: %v", got)
	}
	if got := matchFiles(files, "docs", 10); len(got) != 2 || got[0] != "docs/" {
		t.Fatalf("a folder by name: %v", got)
	}
	if got := matchFiles(files, "main", 10); len(got) != 2 || got[0] != "main.go" {
		t.Fatalf("main: %v", got)
	}
	if got := matchFiles(files, "readme", 10); len(got) != 1 || got[0] != "docs/README.md" {
		t.Fatalf("case-insensitive: %v", got)
	}
	if got := matchFiles(files, "", 3); len(got) != 3 {
		t.Fatalf("empty query lists the first ones: %v", got)
	}
}

func TestPasteTargetUnderMemdoor(t *testing.T) {
	p := pasteTarget("/w", time.Date(2026, 9, 26, 17, 4, 5, 0, time.UTC))
	if p != "/w/.memdoor/paste-20260926-170405.png" {
		t.Fatal(p)
	}
}

package tools

import (
	"os"
	"path/filepath"
	"testing"
)

// An edit keeps the file's layout: its line ending, its final newline or
// the lack of one, and a byte-order mark. A patch's lines arrive with "\n"
// and were written as they came: a CRLF file got LF lines, a file with no
// final newline got one (the class of openclaw#124390).
func TestPatchKeepsTheFileLayout(t *testing.T) {
	patch := beginPatchMarker + "\n" + updateFileMarker + " f.txt\n@@\n one\n-two\n+TWO\n" + endPatchMarker + "\n"
	for name, c := range map[string]struct{ before, want string }{
		"crlf":               {"one\r\ntwo\r\nthree\r\n", "one\r\nTWO\r\nthree\r\n"},
		"no final newline":   {"one\ntwo\nthree", "one\nTWO\nthree"},
		"crlf, no final":     {"one\r\ntwo\r\nthree", "one\r\nTWO\r\nthree"},
		"lf":                 {"one\ntwo\nthree\n", "one\nTWO\nthree\n"},
		"byte-order mark":    {"\ufeffzero\none\ntwo\n", "\ufeffzero\none\nTWO\n"},
		"mixed keeps its \r": {"one\r\ntwo\nthree\r\n", "one\r\nTWO\nthree\r\n"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "f.txt")
			if err := os.WriteFile(path, []byte(c.before), 0o644); err != nil {
				t.Fatal(err)
			}
			applyPatchIn(t, dir, patch)
			got, _ := os.ReadFile(path)
			if string(got) != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// The first line replaced keeps the byte-order mark in front of it.
func TestPatchReplacingTheFirstLineKeepsTheBOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("\ufeffone\r\ntwo\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	applyPatchIn(t, dir, beginPatchMarker+"\n"+updateFileMarker+" f.txt\n@@\n-one\n+ONE\n two\n"+endPatchMarker+"\n")
	got, _ := os.ReadFile(path)
	if want := "\ufeffONE\r\ntwo\r\n"; string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A line-anchored edit keeps the layout too.
func TestHashlineKeepsTheFileLayout(t *testing.T) {
	hashlineOn(t)
	dir := t.TempDir()
	src := "one\r\ntwo\r\nthree"
	path, _ := readHL(t, dir, src)
	if _, err := applyHL(t, dir, "[calc.go#"+FileTag(src)+"]\nreplace 2-2 \"two\":\n+TWO\n"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if want := "one\r\nTWO\r\nthree"; string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

package tools

import "testing"

// A header filename the model decorated must come back clean. Live 2026-09-01:
// "*** Add File: wordcount.go ***" created a file literally named
// "wordcount.go ***", and every later `go run wordcount.go` failed while the
// model re-patched the same wrong name in a loop.
func TestCleanHunkPath(t *testing.T) {
	for in, want := range map[string]string{
		"wordcount.go ***": "wordcount.go",
		"wordcount.go":     "wordcount.go",
		"`main.go`":        "main.go",
		"\"sub/x.go\"":     "sub/x.go",
		"  a/b.go\t":       "a/b.go",
		"star*.go ***":     "star*.go", // only a TRAILING run is decoration
	} {
		if got := cleanHunkPath(in); got != want {
			t.Errorf("cleanHunkPath(%q) = %q, want %q", in, got, want)
		}
	}
}

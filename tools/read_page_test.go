package tools

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// A read is paged (omp's default, 300 lines): whole files were 41% of the
// tool output entering coder conversations, reads over 300 lines 77% of that
// (2026-09-30). The note says what is left and how to get it.
func TestReadFileIsPaged(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 812; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	src := []byte(b.String())
	first := readableContent("big.go", src, "", false, 0, 0)
	if !strings.HasPrefix(first, "line 1\n") || !strings.Contains(first, "line 300\n") || strings.Contains(first, "line 301\n") {
		t.Fatalf("first page is not lines 1-300:\n%s", first[len(first)-200:])
	}
	if !strings.HasSuffix(first, "[512 more lines in big.go (812 in all). read_file with offset: 301 to continue]") {
		t.Fatalf("note: %q", first[len(first)-120:])
	}
	next := readableContent("big.go", src, "", false, 301, 0)
	if !strings.HasPrefix(next, "line 301\n") || !strings.Contains(next, "offset: 601 to continue]") {
		t.Fatalf("second page: %q … %q", next[:20], next[len(next)-80:])
	}
	last := readableContent("big.go", src, "", false, 801, 50)
	if !strings.HasPrefix(last, "line 801\n") || !strings.HasSuffix(last, "line 812\n\n[lines 801-812 of big.go, the end]") {
		t.Fatalf("last page: %q", last)
	}
	if got := readableContent("big.go", src, "", false, 900, 0); !strings.Contains(got, "has 812 lines; line 900 is past the end") {
		t.Fatalf("past the end: %q", got)
	}
	tail := readableContent("big.go", src, "", false, 812, 0)
	if !strings.HasPrefix(tail, "line 812\n") || !strings.HasSuffix(tail, "\n\n[lines 812-812 of big.go, the end]") {
		t.Fatalf("offset at the last line: %q", tail)
	}
	if got := readableContent("big.go", src, "", true, 0, 0); got != string(src) {
		t.Fatal("all: true must return the whole file")
	}
	small := []byte("package x\n\nfunc A() {}\n")
	if got := readableContent("s.go", small, "", false, 0, 0); got != string(small) {
		t.Fatalf("a short file must come back unchanged, got %q", got)
	}
}

// read_file is what every turn stands on: the edges must be boring.
func TestReadFilePagingEdges(t *testing.T) {
	// Short files, however they end, come back byte for byte.
	for _, src := range []string{"", "a", "a\n", "a\r\nb\r\n", "no newline\nat end"} {
		if got := readableContent("f", []byte(src), "", false, 0, 0); got != src {
			t.Errorf("%q came back as %q", src, got)
		}
	}
	// Zero, negative and huge values mean the defaults and "to the end".
	src := []byte(strings.Repeat("x\n", 350))
	for _, c := range []struct{ off, lim int }{{0, 0}, {-3, -1}} {
		if got := readableContent("f", src, "", false, c.off, c.lim); !strings.Contains(got, "(350 in all). read_file with offset: 301") {
			t.Errorf("offset %d limit %d: %q", c.off, c.lim, got[len(got)-80:])
		}
	}
	// A read that covers the whole file is the file, byte for byte, as before paging.
	if got := readableContent("f", src, "", false, 1, 1_000_000); got != string(src) {
		t.Errorf("a whole-file read must be the file itself: %q", got[len(got)-60:])
	}
	if got := readableContent("f", src, "", false, 340, 1_000_000); !strings.HasSuffix(got, "[lines 340-350 of f, the end]") {
		t.Errorf("a huge limit reads to the end: %q", got[len(got)-60:])
	}
	// A page stops on a whole line at the byte cap, and says where to go on.
	long := strings.Repeat(strings.Repeat("y", 1000)+"\n", 200) // 200 KB in 200 lines
	got := readableContent("wide", []byte(long), "", false, 0, 0)
	if len(got) > readFileMax+200 || !strings.Contains(got, "read_file with offset: ") || strings.Contains(got, strings.Repeat("y", 1001)) {
		t.Errorf("a wide page is not cut on a line: len %d, tail %q", len(got), got[len(got)-100:])
	}
	// One line longer than a page is cut on a rune boundary, and says so.
	mono := "a" + strings.Repeat("é", readFileMax) + "\nnext\n" // the cap lands mid-rune
	got = readableContent("min.js", []byte(mono), "", false, 0, 0)
	if !utf8.ValidString(got) || !strings.Contains(got, "line 1 of min.js is ") || !strings.Contains(got, "read_file with offset: 2 to continue") {
		t.Errorf("a giant line: valid=%v tail %q", utf8.ValidString(got), got[len(got)-200:])
	}
}

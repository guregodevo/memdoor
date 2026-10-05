package memory

import (
	"os"
	"path/filepath"
	"testing"

	"memdoor/gateway/logs"
)

func TestMain(m *testing.M) {
	_ = logs.InitGlobalLoggerDefault(false)
	os.Exit(m.Run())
}

// The memory tool must not depend on a BUILD FLAG.
//
// mattn/go-sqlite3 compiles FTS5 in only when asked (the Makefile passes
// CGO_CFLAGS=-DSQLITE_ENABLE_FTS5), so any binary built with a plain `go build`
// has no fts5 module and "CREATE VIRTUAL TABLE ... USING fts5" fails with "no
// such module: fts5". That took the whole tool down: measured 2026-08-30 on a
// live coder turn, the agent's finding was lost to
// "failed to store memory: ... no such module: fts5".
//
// Storing must work either way; only ranking may degrade.
func TestMemoryWorksWithoutFTS5(t *testing.T) {
	s, err := NewBM25Store(filepath.Join(t.TempDir(), "m.sqlite"), false)
	if err != nil {
		t.Fatalf("store must open whether or not FTS5 is compiled in: %v", err)
	}
	id, err := s.Store("news_summarizer.py parses RSS with the stdlib", []string{"finding"}, nil)
	if err != nil {
		t.Fatalf("storing a memory must not depend on FTS5: %v", err)
	}
	if id == "" {
		t.Fatal("a stored memory needs an id")
	}
	got, err := s.Search("stdlib", 5)
	if err != nil {
		t.Fatalf("search must work in both modes: %v", err)
	}
	if len(got) == 0 {
		t.Error("the memory just stored must be findable — degraded ranking is fine, losing it is not")
	}
}

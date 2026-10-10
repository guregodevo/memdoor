package tools

import (
	"path/filepath"
	"sort"
	"sync"
)

// Reads are lock-free; a writer is not (Greg, 2026-10-11: "there are lock
// free tools in general but some like apply_patch is not"). apply_patch,
// write_file, edit_file and search_replace each read a file, compute, and
// write it back; two sessions in one gateway (a spawned subtask, a scheduled
// check, a second window on the same project) doing that to the same file at
// once lose one of the two edits. A writer holds the file's lock for the
// whole read-modify-write. Locks are per cleaned absolute path and taken in
// sorted order, so a patch over several files cannot deadlock with another.
var fileLocks sync.Map // path -> *sync.Mutex

// LockFiles takes the lock of every path and returns the release.
func LockFiles(paths ...string) (unlock func()) {
	seen := map[string]bool{}
	var keys []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		p = filepath.Clean(p)
		if !seen[p] {
			seen[p] = true
			keys = append(keys, p)
		}
	}
	sort.Strings(keys)
	held := make([]*sync.Mutex, 0, len(keys))
	for _, k := range keys {
		m, _ := fileLocks.LoadOrStore(k, &sync.Mutex{})
		mu := m.(*sync.Mutex)
		mu.Lock()
		held = append(held, mu)
	}
	return func() {
		for i := len(held) - 1; i >= 0; i-- {
			held[i].Unlock()
		}
	}
}

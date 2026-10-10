package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A writer waits for the file's lock: apply_patch and edit_file on a file
// another writer holds do not run until it is released, and then land.
func TestAWriterWaitsForTheFilesLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	os.WriteFile(path, []byte("one\n"), 0o644)

	writers := []struct {
		name string
		run  func() (string, error)
	}{
		{"apply_patch", func() (string, error) {
			in, _ := json.Marshal(map[string]string{"input": "*** Begin Patch\n*** Update File: a.txt\n@@\n-one\n+two\n*** End Patch", "cwd": dir})
			return ApplyPatch(in)
		}},
		{"edit_file", func() (string, error) {
			in, _ := json.Marshal(map[string]string{"file_path": path, "old_string": "two", "new_string": "three"})
			return EditFile(in)
		}},
		{"write_file", func() (string, error) {
			in, _ := json.Marshal(map[string]string{"path": path, "content": "four\n"})
			return WriteFile(in)
		}},
	}
	for _, w := range writers {
		name, run := w.name, w.run
		unlock := LockFiles(path)
		done := make(chan error, 1)
		go func() { _, err := run(); done <- err }()
		select {
		case err := <-done:
			t.Fatalf("%s ran while the file was locked (err=%v)", name, err)
		case <-time.After(150 * time.Millisecond):
		}
		unlock()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("%s after the release: %v", name, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s never ran after the release", name)
		}
	}
	if b, _ := os.ReadFile(path); string(b) != "four\n" {
		t.Fatalf("the three writers in order should leave four, got %q", b)
	}
}

// The same file named two ways is one lock, and several files lock in one
// order whichever order they are named.
func TestLockFilesIsOnePerPath(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	unlock := LockFiles(b, a, filepath.Join(dir, "x", "..", "a"))
	done := make(chan struct{})
	go func() { LockFiles(a, b)(); close(done) }()
	select {
	case <-done:
		t.Fatal("a second lock on the same files was taken while the first was held")
	case <-time.After(100 * time.Millisecond):
	}
	unlock()
	<-done
}

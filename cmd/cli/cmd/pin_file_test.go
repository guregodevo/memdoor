package cmd

import (
	"memdoor/pkg/shared"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWritePinFileRefusesAtHome pins the bug fix: running
// `memdoor workspace use <slug>` from $HOME would otherwise try to
// write `~/.memdoor/workspace`, which collides with the bootstrap
// directory `memdoor setup` creates. The helper must refuse with an
// actionable message before any disk side-effect.
func TestWritePinFileRefusesAtHome(t *testing.T) {
	// Fake $HOME to an isolated dir + chdir into it. UserHomeDir reads
	// $HOME on Unix, which t.Setenv populates without affecting other
	// tests.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(fakeHome); err != nil {
		t.Fatalf("chdir home: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	_, err = writePinFile(discoveryFileName, "demo")
	if err == nil {
		t.Fatalf("expected refusal when writing from $HOME, got nil")
	}
	if !strings.Contains(err.Error(), "$HOME") {
		t.Fatalf("expected error to mention $HOME, got: %v", err)
	}
	// Nothing should have been created.
	if _, statErr := os.Stat(filepath.Join(fakeHome, shared.MemdoorDirName, discoveryFileName)); !os.IsNotExist(statErr) {
		t.Fatalf("pin file should not exist after refusal, stat err=%v", statErr)
	}
}

// TestWritePinFileRefusesWhenTargetIsDir covers the broader collision:
// the install path created the pin path as a directory (e.g. someone
// ran setup with a non-default workspace dir, leaving ~/.memdoor/data/
// as a directory). The helper must refuse rather than crashing with
// EISDIR mid-write.
func TestWritePinFileRefusesWhenTargetIsDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	projectDir := filepath.Join(fakeHome, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	prev, _ := os.Getwd()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir project: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	// Pre-create the target as a directory.
	if err := os.MkdirAll(filepath.Join(projectDir, shared.MemdoorDirName, discoveryFileName), 0o755); err != nil {
		t.Fatalf("seed conflicting dir: %v", err)
	}

	_, err := writePinFile(discoveryFileName, "demo")
	if err == nil {
		t.Fatalf("expected refusal when target is a directory")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected error to mention directory, got: %v", err)
	}
}

// TestWritePinFileWritesInProjectDir is the success path: from a
// regular project subdir, the helper writes the pin file with the
// expected contents.
func TestWritePinFileWritesInProjectDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	projectDir := filepath.Join(fakeHome, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	prev, _ := os.Getwd()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	path, err := writePinFile(discoveryFileName, "my-workspace")
	if err != nil {
		t.Fatalf("writePinFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pin file: %v", err)
	}
	if strings.TrimSpace(string(got)) != "my-workspace" {
		t.Fatalf("pin file contents: got %q, want %q", string(got), "my-workspace")
	}
}

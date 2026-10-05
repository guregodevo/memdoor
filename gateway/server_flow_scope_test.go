package gateway

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestChangedGoFilesNonGit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := changedGoFiles(dir); got != nil {
		t.Errorf("non-git dir should yield nil, got %v", got)
	}
}

func TestScopedBuildArgsFallbackNonGit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := scopedBuildArgs(dir); !reflect.DeepEqual(got, []string{"build", "./..."}) {
		t.Errorf("non-git → full build ./..., got %v", got)
	}
}

func TestScopedBuildArgsScopesGitChanges(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Skipf("git unavailable: %v %s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "x", "a.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The new untracked file scopes the build to its package.
	got := scopedBuildArgs(dir)
	want := []string{"build", "./pkg/x/..."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("scoped args = %v, want %v", got, want)
	}
}

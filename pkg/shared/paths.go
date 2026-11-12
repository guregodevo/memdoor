package shared

import (
	"os"
	"path/filepath"
)

// MemdoorDirName is the directory Memdoor keeps its state in, under the home
// directory and, for what belongs to a project, under the project.
const MemdoorDirName = ".memdoor"

// IsolatedWorktreeLock is the reason a workflow's isolated copy is locked
// with in git (pkg/workflow/isolate.go). The write fence reads it from `git
// worktree list`: a turn in such a worktree writes only there, and no turn
// elsewhere writes into it (tools.InProject).
const IsolatedWorktreeLock = "memdoor isolation"

// MemdoorHome is ~/.memdoor joined with parts: where Memdoor keeps a person's
// state. Every path under it is built here, so the place is decided once
// (the maintainer, 2026-10-04: "DRY"). With no home directory it falls back to
// ./.memdoor rather than to the filesystem root.
func MemdoorHome(parts ...string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return filepath.Join(append([]string{home, MemdoorDirName}, parts...)...)
}

// ProjectOrHome is <dir>/.memdoor/<parts> when it exists, else
// ~/.memdoor/<parts>: the project's own copy first, the person's next — a
// workflow kept in the repo beside one kept in the library. An empty dir means
// the current directory.
func ProjectOrHome(dir string, parts ...string) string {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if dir != "" {
		p := filepath.Join(append([]string{dir, MemdoorDirName}, parts...)...)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return MemdoorHome(parts...)
}

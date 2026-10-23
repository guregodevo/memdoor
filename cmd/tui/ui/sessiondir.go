package ui

import (
	"os"
	"path/filepath"

	"memdoor/pkg/filelist"
)

// THE SESSION'S DIRECTORY IS WHERE THE GATEWAY WORKS, which is this process's
// directory when the gateway is on this machine (os.Chdir on /dir, the
// process directory the truth), and a directory on the gateway's machine
// when the window is on a laptop and the project on a box that stays on
// (option 1, 2026-10-23). Everything that reads "the project" asks here and
// goes through FS, so the panel, the mentions and /dir work the same against
// either; the one thing that differs is where the reads run.

// sessionDirOverride is the directory on the gateway's machine, set when the
// gateway is remote; "" means the process directory.
var sessionDirOverride string

// SetSessionDir makes dir, a directory on the gateway's machine, the
// session's directory; "" goes back to the process directory.
func SetSessionDir(dir string) { sessionDirOverride = dir }

// SessionDir is the directory the next turn works in.
func SessionDir() string {
	if sessionDirOverride != "" {
		return sessionDirOverride
	}
	wd, _ := os.Getwd()
	return wd
}

// RemoteDir says whether the session's directory is on another machine.
func RemoteDir() bool { return sessionDirOverride != "" }

// FS is what the window reads of the project: a listing for the panel and
// the mentions, a preview for the panel's right column, a stat for /dir.
// localFS reads this machine; the command wires a remote one over the
// gateway's /api/fs when the project is elsewhere.
type FS interface {
	List(root string) ([]string, error)
	Preview(root, rel string, diff bool) ([]string, error)
	Stat(path string) (abs string, isDir, exists bool, err error)
}

type localFS struct{}

func (localFS) List(root string) ([]string, error) { return filelist.List(root), nil }
func (localFS) Preview(root, rel string, diff bool) ([]string, error) {
	return filelist.Preview(root, rel, diff), nil
}
func (localFS) Stat(path string) (string, bool, bool, error) {
	abs, isDir, exists := filelist.Stat(path)
	return abs, isDir, exists, nil
}

// fsOf is the page's FS, local when none was wired.
func (m *Model) fsOf() FS {
	if m.fs == nil {
		return localFS{}
	}
	return m.fs
}

// dirBase is the base name of the session's directory for the header and
// the terminal title.
func dirBase() string {
	d := SessionDir()
	if d == "" {
		return ""
	}
	return filepath.Base(d)
}

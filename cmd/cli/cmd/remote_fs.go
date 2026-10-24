package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"memdoor/cmd/tui/ui"
)

// remoteFS is the window's view of a project on the gateway's machine:
// every read goes to GET /api/fs (gateway/fs_handlers.go) on the authed
// client, so the files panel, the @-mentions and /dir show that machine's
// directory, not this laptop's (option 1, 2026-10-23).
type remoteFS struct{ c *Client }

func (r remoteFS) List(root string) ([]string, error) {
	var out struct {
		Files []string `json:"files"`
	}
	q := url.Values{"op": {"list"}, "root": {root}}
	if err := r.c.GetJSON("/api/fs?"+q.Encode(), &out); err != nil {
		return nil, fsError(err)
	}
	return out.Files, nil
}

func (r remoteFS) Preview(root, rel string, diff bool) ([]string, error) {
	var out struct {
		Lines []string `json:"lines"`
	}
	q := url.Values{"op": {"read"}, "root": {root}, "path": {rel}}
	if diff {
		q.Set("diff", "1")
	}
	if err := r.c.GetJSON("/api/fs?"+q.Encode(), &out); err != nil {
		return nil, fsError(err)
	}
	return out.Lines, nil
}

func (r remoteFS) Stat(path string) (string, bool, bool, error) {
	var out struct {
		Path   string `json:"path"`
		Dir    bool   `json:"dir"`
		Exists bool   `json:"exists"`
	}
	q := url.Values{"op": {"stat"}, "path": {path}}
	if err := r.c.GetJSON("/api/fs?"+q.Encode(), &out); err != nil {
		return "", false, false, fsError(err)
	}
	return out.Path, out.Dir, out.Exists, nil
}

// fsError names the gateway as the place the read failed.
func fsError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "404") {
		return fmt.Errorf("the gateway at %s has no such directory", gatewayShown())
	}
	return fmt.Errorf("the gateway at %s: %v", gatewayShown(), err)
}

var _ ui.FS = remoteFS{}

// remoteProjectDir settles the session's directory for a window whose
// gateway is on another machine: the --dir given, checked there. Without
// one the window cannot know where that machine's project is.
func remoteProjectDir(c *Client, dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		abs, why, err := remoteDefaultDir(c)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(os.Stderr, "  on %s, working in %s (%s; --dir names another)\n", gatewayHostOnly(), abs, why)
		return abs, nil
	}
	abs, isDir, exists, err := remoteFS{c}.Stat(dir)
	if err != nil {
		return "", err
	}
	if !exists || !isDir {
		return "", fmt.Errorf("%s is not a directory on the gateway's machine (%s)", abs, gatewayShown())
	}
	return abs, nil
}

// remoteDefaultDir is where a turn on another machine works when no --dir
// says (Greg, 2026-10-24: "good default please"; before, the laptop's own
// directory was sent and refused by name there). The same path as here,
// relative to home, when that machine has it: a repo cloned at the same
// place on both. Else that machine's home, where ssh itself lands you.
func remoteDefaultDir(c *Client) (abs, why string, err error) {
	fs := remoteFS{c}
	if cwd, cerr := os.Getwd(); cerr == nil {
		if home, herr := os.UserHomeDir(); herr == nil {
			if h, err := filepath.EvalSymlinks(home); err == nil {
				home = h // a home behind a symlink (/var → /private/var on macOS)
			}
			if rel, rerr := filepath.Rel(home, cwd); rerr == nil && rel != "." && !strings.HasPrefix(rel, "..") {
				if abs, isDir, exists, serr := fs.Stat("~/" + filepath.ToSlash(rel)); serr == nil && exists && isDir {
					return abs, "the same path as here", nil
				}
			}
		}
	}
	abs, isDir, exists, serr := fs.Stat("~")
	if serr != nil {
		return "", "", serr
	}
	if !exists || !isDir {
		return "", "", fmt.Errorf("no home directory on the gateway's machine (%s): --dir names where to work", gatewayShown())
	}
	return abs, "its home", nil
}

// gatewayHostOnly is the gateway's host for a line a person retypes.
func gatewayHostOnly() string {
	if sshDoorUp != nil {
		return sshDoorUp.name
	}
	if u, err := url.Parse(gatewayAddr); err == nil && u.Host != "" {
		return u.Host
	}
	return gatewayAddr
}

// expandHomeDir is "~" and "~/x" as this user's home, for a --dir typed by
// hand on this machine.
func expandHomeDir(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

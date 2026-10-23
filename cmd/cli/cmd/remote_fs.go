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
		return "", fmt.Errorf("the gateway at %s is on another machine: say which directory there the window works in\n  memdoor --gateway %s --dir /path/on/that/machine\n  or open a /remote link from a window on that machine: memdoor join <link>", gatewayShown(), gatewayHostOnly())
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

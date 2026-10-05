package ui

import (
	"fmt"
	"memdoor/pkg/shared"
	"memdoor/pkg/workflow"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Image paste (docs/roadmap/MUST.md, "TUI table stakes"): ctrl+v with an
// image on the clipboard saves it under the working directory's .memdoor
// folder and mentions it in the prompt, the way a file is mentioned. The
// coder's prompt says an @…png is a picture to look at with see; the
// .memdoor folder is ignored by the project's git and by the coder's grep.
//
// macOS only for now: the clipboard is read with osascript, which needs no
// extra tool. A clipboard without an image is left to the terminal's usual
// paste.

// pasteTarget names the file a pasted image goes to.
func pasteTarget(dir string, now time.Time) string {
	return filepath.Join(dir, shared.MemdoorDirName, "paste-"+now.Format("20060102-150405")+".png")
}

// pasteImage writes the clipboard's image to a file under dir and returns
// its absolute path. It errors when there is no image (or not on macOS).
func pasteImage(dir string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("image paste is macOS only for now")
	}
	target := pasteTarget(dir, time.Now())
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	// The pasted image stays out of commits, and .memdoor/workflows stays in
	// them: the standard list, not "*" (which hid every workflow, 2026-10-04).
	workflow.EnsureIgnored(dir)
	script := fmt.Sprintf(`set f to open for access POSIX file %q with write permission
try
	set eof f to 0
	write (the clipboard as «class PNGf») to f
	close access f
on error m
	close access f
	error m
end try`, target)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		_ = os.Remove(target)
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "-1700") || strings.Contains(msg, "-2700") || strings.Contains(msg, "expected type") {
			return "", fmt.Errorf("no image on the clipboard")
		}
		return "", fmt.Errorf("clipboard: %s", msg)
	}
	if fi, err := os.Stat(target); err != nil || fi.Size() == 0 {
		_ = os.Remove(target)
		return "", fmt.Errorf("no image on the clipboard")
	}
	return target, nil
}

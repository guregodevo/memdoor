//go:build windows

package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gitBashPath spells a Windows path as Git Bash prints it: C:\x\y → /c/x/y.
func gitBashPath(p string) string {
	p = filepath.ToSlash(p)
	return "/" + strings.ToLower(p[:1]) + p[2:]
}

// A patch naming a file the Git Bash way writes that file, and the fence
// still holds for a Git Bash path outside the project.
func TestAGitBashPathInAPatchIsTheDrivesPath(t *testing.T) {
	project, outside := t.TempDir(), t.TempDir()
	target := filepath.Join(project, "x.txt")
	in, _ := json.Marshal(map[string]string{"cwd": project,
		"input": "*** Begin Patch\n*** Add File: " + gitBashPath(target) + "\n+hi\n*** End Patch"})
	if _, err := ApplyPatch(in); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(target); err != nil || string(b) != "hi\n" {
		t.Fatalf("%q %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(project, strings.TrimPrefix(filepath.FromSlash(gitBashPath(target)), `\`))); err == nil {
		t.Fatal("the Git Bash path was joined under the project")
	}
	away := filepath.Join(outside, "y.txt")
	in, _ = json.Marshal(map[string]string{"cwd": project,
		"input": "*** Begin Patch\n*** Add File: " + gitBashPath(away) + "\n+hi\n*** End Patch"})
	if _, err := ApplyPatch(in); err == nil || !strings.Contains(err.Error(), "outside the project folder") {
		t.Fatalf("%v", err)
	}
}

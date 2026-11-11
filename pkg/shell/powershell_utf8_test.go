package shell

import (
	"context"
	"strings"
	"testing"
)

// A PowerShell command runs after the line that makes Windows PowerShell 5.1
// write UTF-8 (#282); bash and the other shells get the line as it is.
func TestAPowerShellCommandWritesUTF8(t *testing.T) {
	ps := choice{name: PowerShell, prog: "powershell.exe", args: []string{"-NoProfile", "-NonInteractive", "-Command"}}
	cmd := ps.command(context.Background(), "echo hello > notes.txt")
	last := cmd.Args[len(cmd.Args)-1]
	if !strings.HasPrefix(last, powerShellUTF8) || !strings.HasSuffix(last, "; echo hello > notes.txt") {
		t.Fatalf("%q", last)
	}
	if !strings.Contains(powerShellUTF8, "$PSVersionTable.PSVersion.Major -lt 6") {
		t.Fatal("PowerShell 7 already writes UTF-8 and is left alone")
	}
	bash := choice{name: Bash, prog: "bash", args: []string{"-c"}}
	if got := bash.command(context.Background(), "echo hi").Args; got[len(got)-1] != "echo hi" {
		t.Fatalf("bash got %q", got)
	}
	if h := ps.hint(); !strings.Contains(h, "apply_patch") || !strings.Contains(h, "byte-order mark") {
		t.Fatalf("hint %q", h)
	}
}

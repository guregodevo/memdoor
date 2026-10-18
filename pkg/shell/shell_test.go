package shell

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func lookup(found map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if p, ok := found[name]; ok {
			return p, nil
		}
		return "", errors.New("not found")
	}
}

func TestResolvePicksBashWhereItIsAndPowerShellOnAWindowsWithoutIt(t *testing.T) {
	cases := []struct {
		goos  string
		found map[string]string
		want  string
		prog  string
	}{
		{"darwin", map[string]string{"bash": "/bin/bash"}, Bash, "bash"},
		{"linux", map[string]string{}, Sh, "sh"},
		{"windows", map[string]string{"bash": `C:\Program Files\Git\bin\bash.exe`, "pwsh": `C:\pwsh.exe`}, Bash, `C:\Program Files\Git\bin\bash.exe`},
		// The WSL launcher is not a bash: without a distribution it answers with an error.
		{"windows", map[string]string{"bash": `C:\Windows\System32\bash.exe`, "pwsh": `C:\pwsh.exe`}, PowerShell, `C:\pwsh.exe`},
		{"windows", map[string]string{"powershell": `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`}, PowerShell, `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`},
		{"windows", map[string]string{}, Cmd, "cmd"},
	}
	for _, c := range cases {
		got := resolve(c.goos, lookup(c.found))
		if got.name != c.want || got.prog != c.prog {
			t.Errorf("%s %v: got %s %s, want %s %s", c.goos, c.found, got.name, got.prog, c.want, c.prog)
		}
		if got.name == PowerShell && !strings.Contains(strings.Join(got.args, " "), "-NonInteractive") {
			t.Errorf("powershell must be non-interactive: %v", got.args)
		}
	}
}

func TestCommandRunsALineOnThisMachine(t *testing.T) {
	out, err := Command(context.Background(), "echo shell-ok").Output()
	if err != nil || !strings.Contains(string(out), "shell-ok") {
		t.Fatalf("%s: %q %v", Name(), out, err)
	}
	if Name() == Bash && Hint() != "" {
		t.Fatalf("bash needs no hint, got %q", Hint())
	}
}

package shell

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// machine is a host made of what is on PATH, what files exist and what the
// environment says; paths use / so the table reads the same on every OS.
func machine(goos string, onPath map[string]string, files []string, env map[string]string) host {
	return host{
		goos: goos,
		look: func(name string) (string, error) {
			if p, ok := onPath[name]; ok {
				return p, nil
			}
			return "", errors.New("not found")
		},
		exists: func(p string) bool {
			for _, f := range files {
				if f == p {
					return true
				}
			}
			return false
		},
		getenv: func(k string) string { return env[k] },
	}
}

func TestResolvePicksBashWhereItIsAndPowerShellOnAWindowsWithoutIt(t *testing.T) {
	const gitBash = "C:/Program Files/Git/bin/bash.exe"
	wsl := map[string]string{"bash": "C:/Windows/System32/bash.exe", "pwsh": "C:/pwsh.exe"}
	cases := []struct {
		why  string
		h    host
		want string
		prog string
	}{
		{"darwin, bash by its bare name", machine("darwin", map[string]string{"bash": "/bin/bash"}, nil, nil), Bash, "bash"},
		{"linux without bash", machine("linux", nil, nil, nil), Sh, "sh"},
		{"Git Bash on PATH", machine("windows", map[string]string{"bash": gitBash, "pwsh": "C:/pwsh.exe"}, nil, nil), Bash, gitBash},
		{"the WSL launcher is not a bash", machine("windows", wsl, nil, nil), PowerShell, "C:/pwsh.exe"},
		{"WSL launcher first on PATH, Git Bash behind git.exe", machine("windows", map[string]string{"bash": "C:/Windows/System32/bash.exe", "git": "C:/Program Files/Git/cmd/git.exe", "pwsh": "C:/pwsh.exe"}, []string{gitBash}, nil), Bash, gitBash},
		{"the default Git install: only git.exe on PATH", machine("windows", map[string]string{"git": "C:/Program Files/Git/cmd/git.exe", "powershell": "C:/ps.exe"}, []string{gitBash}, nil), Bash, gitBash},
		{"Git installed and on no PATH at all", machine("windows", map[string]string{"powershell": "C:/ps.exe"}, []string{gitBash}, map[string]string{"ProgramFiles": "C:/Program Files"}), Bash, gitBash},
		{"a per-user Git", machine("windows", map[string]string{"powershell": "C:/ps.exe"}, []string{"C:/Users/me/AppData/Local/Programs/Git/bin/bash.exe"}, map[string]string{"LocalAppData": "C:/Users/me/AppData/Local"}), Bash, "C:/Users/me/AppData/Local/Programs/Git/bin/bash.exe"},
		{"git on PATH but its bash gone", machine("windows", map[string]string{"git": "C:/Program Files/Git/cmd/git.exe", "powershell": "C:/ps.exe"}, nil, nil), PowerShell, "C:/ps.exe"},
		{"windows powershell only", machine("windows", map[string]string{"powershell": "C:/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"}, nil, nil), PowerShell, "C:/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"},
		{"windows with nothing", machine("windows", nil, nil, nil), Cmd, "cmd"},
	}
	for _, c := range cases {
		got := resolve(c.h)
		if got.name != c.want || got.prog != c.prog {
			t.Errorf("%s: got %s %s, want %s %s", c.why, got.name, got.prog, c.want, c.prog)
		}
		if got.name == PowerShell && !strings.Contains(strings.Join(got.args, " "), "-NonInteractive") {
			t.Errorf("%s: powershell must be non-interactive: %v", c.why, got.args)
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

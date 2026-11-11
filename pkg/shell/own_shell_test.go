package shell

import (
	"strings"
	"testing"
)

// MEMDOOR_SHELL picks the person's own shell as a login shell; unset, or
// naming nothing that exists, the choice is the one made without it.
func TestMemdoorShellPicksThePersonsOwnShell(t *testing.T) {
	bash := map[string]string{"bash": "/bin/bash", "fish": "/opt/homebrew/bin/fish"}
	cases := []struct {
		why  string
		h    host
		name string
		prog string
		args string
		own  bool
	}{
		{"unset: bash as before", machine("darwin", bash, []string{"/bin/zsh"}, map[string]string{"SHELL": "/bin/zsh"}), Bash, "bash", "-c", false},
		{"zsh by path: login and interactive", machine("darwin", bash, []string{"/bin/zsh"}, map[string]string{ShellEnv: "/bin/zsh"}), Zsh, "/bin/zsh", "-l -i -c", true},
		{"fish by name on PATH: login only", machine("linux", bash, []string{"/opt/homebrew/bin/fish"}, map[string]string{ShellEnv: "fish"}), "fish", "/opt/homebrew/bin/fish", "-l -c", true},
		{"a path that does not exist", machine("darwin", bash, nil, map[string]string{ShellEnv: "/bin/nope"}), Bash, "bash", "-c", false},
		{"Windows keeps its own choice", machine("windows", map[string]string{"pwsh": "C:/pwsh.exe"}, []string{"/bin/zsh"}, map[string]string{ShellEnv: "/bin/zsh"}), PowerShell, "C:/pwsh.exe", "-NoProfile -NonInteractive -Command", false},
	}
	for _, c := range cases {
		got := resolve(c.h)
		if got.name != c.name || got.prog != c.prog || strings.Join(got.args, " ") != c.args || got.own != c.own {
			t.Errorf("%s: got %+v", c.why, got)
		}
	}
}

func TestTheModelIsToldWhenItsShellIsZsh(t *testing.T) {
	zsh := choice{name: Zsh, prog: "/bin/zsh", own: true}
	if h := zsh.hint(); !strings.Contains(h, "zsh is not bash") || !strings.Contains(h, "/bin/zsh") {
		t.Fatalf("%q", h)
	}
	if h := (choice{name: Bash, prog: "/bin/bash", own: true}).hint(); h != "" {
		t.Fatalf("bash needs no hint: %q", h)
	}
	if h := (choice{name: Bash, prog: "bash"}).hint(); h != "" {
		t.Fatalf("%q", h)
	}
}

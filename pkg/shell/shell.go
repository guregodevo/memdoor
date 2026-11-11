// Package shell is the one answer to "which shell runs a command line on
// this machine". The agent's bash tool, a task's `command:` target, a
// provider key given as `!cmd` and a secret read by command all ask it.
//
// bash when there is one: every macOS and Linux, Git for Windows, WSL. On a
// Windows machine without one, PowerShell (pwsh, else powershell), which the
// system prompt then names so the model writes PowerShell (the maintainer, 2026-10-18:
// "it should work on windows too"). The launcher Windows ships in System32
// as bash.exe is WSL's and fails with no distribution installed, so it does
// not count.
package shell

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	Bash       = "bash"
	PowerShell = "powershell"
	Cmd        = "cmd"
	Sh         = "sh"
	Zsh        = "zsh"
)

// ShellEnv names the person's own shell for the agent's commands: a path or
// a name on PATH (MEMDOOR_SHELL=$SHELL). Unset, the choice below is made.
const ShellEnv = "MEMDOOR_SHELL"

type choice struct {
	name string
	prog string
	args []string
	own  bool // the person's shell from MEMDOOR_SHELL, run as a login shell
}

// host is what resolve asks of the machine: its OS, its PATH, its files and
// its environment, so a test can be any machine.
type host struct {
	goos   string
	look   func(string) (string, error)
	exists func(string) bool
	getenv func(string) string
}

func thisHost() host {
	return host{
		goos:   runtime.GOOS,
		look:   exec.LookPath,
		exists: func(p string) bool { _, err := os.Stat(p); return err == nil },
		getenv: os.Getenv,
	}
}

var (
	once   sync.Once
	chosen choice
)

func current() choice {
	once.Do(func() { chosen = resolve(thisHost()) })
	return chosen
}

// Command is the shell running line, with ctx as its lifetime and Env as
// its environment (no model key).
func Command(ctx context.Context, line string) *exec.Cmd {
	c := current()
	args := append(append([]string{}, c.args...), line)
	cmd := exec.CommandContext(ctx, c.prog, args...)
	cmd.Env = Env()
	return cmd
}

// Name is the shell in use: bash, powershell, cmd or sh, or the base name of
// the shell MEMDOOR_SHELL names (zsh, fish…).
func Name() string { return current().name }

// Detached says whether a command must run in a session of its own, with no
// controlling terminal: an interactive zsh started under a gateway that holds
// a window's terminal takes that terminal, gets SIGTTOU and stays stopped.
func Detached() bool { return current().own }

// Hint is what the model is told when its bash tool is not bash; "" when it is.
func Hint() string { return current().hint() }

func (c choice) hint() string {
	if c.own && c.name != Bash {
		h := "The bash tool runs commands in the person's own shell, " + c.name + " (" + c.prog + "), with their config loaded: write commands " + c.name + " accepts."
		if c.name == Zsh {
			h += " zsh is not bash: an unquoted glob that matches nothing is an error, and an unquoted $var is not split into words."
		}
		return h
	}
	switch c.name {
	case PowerShell:
		return "The bash tool runs commands with PowerShell on this machine (no bash on PATH): write PowerShell, not POSIX sh; a native command's exit code is $LASTEXITCODE."
	case Cmd:
		return "The bash tool runs commands with cmd.exe on this machine (no bash or PowerShell on PATH): write cmd syntax, not POSIX sh."
	case Sh:
		return "The bash tool runs commands with /bin/sh on this machine (no bash on PATH): write POSIX sh."
	}
	return ""
}

func resolve(h host) choice {
	if c, ok := h.ownShell(); ok {
		return c
	}
	if p, ok := h.bash(); ok {
		return choice{name: Bash, prog: p, args: []string{"-c"}}
	}
	if h.goos == "windows" {
		for _, ps := range []string{"pwsh", "powershell"} {
			if p, err := h.look(ps); err == nil {
				return choice{name: PowerShell, prog: p, args: []string{"-NoProfile", "-NonInteractive", "-Command"}}
			}
		}
		return choice{name: Cmd, prog: "cmd", args: []string{"/c"}}
	}
	return choice{name: Sh, prog: "sh", args: []string{"-c"}}
}

// ownShell is the shell MEMDOOR_SHELL names, off Windows, when it can be
// found. It runs as a login shell, so a command sees the PATH and the rest
// of the person's profile. zsh also runs interactive: only an interactive
// zsh reads ~/.zshrc, and without a terminal it prints nothing extra. Any
// other shell runs as a login shell only: an interactive bash with no
// terminal writes "no job control in this shell" into the output.
func (h host) ownShell() (choice, bool) {
	p := strings.TrimSpace(h.getenv(ShellEnv))
	if h.goos == "windows" || p == "" {
		return choice{}, false
	}
	// A POSIX path: ownShell runs off Windows only, whatever OS reads it.
	if !strings.HasPrefix(p, "/") {
		found, err := h.look(p)
		if err != nil {
			return choice{}, false
		}
		p = found
	}
	if !h.exists(p) {
		return choice{}, false
	}
	name := filepath.Base(p)
	args := []string{"-l", "-c"}
	if name == Zsh {
		args = []string{"-l", "-i", "-c"}
	}
	return choice{name: name, prog: p, args: args, own: true}, true
}

// bash is the bash this machine runs: the one on PATH, by its bare name off
// Windows so it is looked up at each run as it always was; on Windows, the
// one Git for Windows installed when PATH does not say. The default Git
// installer puts only Git\cmd on PATH, which holds git.exe and not bash.exe,
// and where WSL is enabled System32's launcher comes first on PATH, so PATH
// alone misses most Git Bash installs.
func (h host) bash() (string, bool) {
	p, err := h.look("bash")
	if err == nil && !(h.goos == "windows" && isWSLLauncher(p)) {
		if h.goos != "windows" {
			return "bash", true
		}
		return p, true
	}
	if h.goos != "windows" {
		return "", false
	}
	for _, c := range h.gitBashCandidates() {
		if h.exists(c) {
			return c, true
		}
	}
	return "", false
}

// gitBashCandidates is where Git for Windows keeps bash.exe: beside the
// git.exe on PATH (Git\cmd\git.exe or Git\bin\git.exe, bash in Git\bin),
// then the per-machine and per-user install folders.
func (h host) gitBashCandidates() []string {
	var out []string
	if git, err := h.look("git"); err == nil {
		root := filepath.Dir(filepath.Dir(git))
		out = append(out, filepath.Join(root, "bin", "bash.exe"))
	}
	for _, in := range []struct{ env, sub string }{{"ProgramFiles", ""}, {"ProgramFiles(x86)", ""}, {"LocalAppData", "Programs"}} {
		if base := h.getenv(in.env); base != "" {
			out = append(out, filepath.Join(base, in.sub, "Git", "bin", "bash.exe"))
		}
	}
	return out
}

func isWSLLauncher(path string) bool {
	p := strings.ToLower(strings.ReplaceAll(path, "/", `\`))
	return strings.Contains(p, `\windows\system32\`) || strings.Contains(p, `\windows\sysnative\`)
}

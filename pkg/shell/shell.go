// Package shell is the one answer to "which shell runs a command line on
// this machine". The agent's bash tool, a task's `command:` target, a
// provider key given as `!cmd` and a secret read by command all ask it.
//
// bash when there is one: every macOS and Linux, Git for Windows, WSL. On a
// Windows machine without one, PowerShell (pwsh, else powershell), which the
// system prompt then names so the model writes PowerShell (Greg, 2026-10-18:
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
)

type choice struct {
	name string
	prog string
	args []string
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

// Command is the shell running line, with ctx as its lifetime.
func Command(ctx context.Context, line string) *exec.Cmd {
	c := current()
	args := append(append([]string{}, c.args...), line)
	return exec.CommandContext(ctx, c.prog, args...)
}

// Name is the family of the shell in use: bash, powershell, cmd or sh.
func Name() string { return current().name }

// Hint is what the model is told when its bash tool is not bash; "" when it is.
func Hint() string {
	switch current().name {
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
	if p, ok := h.bash(); ok {
		return choice{Bash, p, []string{"-c"}}
	}
	if h.goos == "windows" {
		for _, ps := range []string{"pwsh", "powershell"} {
			if p, err := h.look(ps); err == nil {
				return choice{PowerShell, p, []string{"-NoProfile", "-NonInteractive", "-Command"}}
			}
		}
		return choice{Cmd, "cmd", []string{"/c"}}
	}
	return choice{Sh, "sh", []string{"-c"}}
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

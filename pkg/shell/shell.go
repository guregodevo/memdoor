// Package shell is the one answer to "which shell runs a command line on
// this machine". The agent's bash tool, a task's `command:` target, a
// provider key given as `!cmd` and a secret read by command all ask it.
//
// bash when it is on PATH: every macOS and Linux, Git for Windows, WSL. On a
// Windows machine without one, PowerShell (pwsh, else powershell), which the
// system prompt then names so the model writes PowerShell (Greg, 2026-10-18:
// "it should work on windows too"). The launcher Windows ships in System32
// as bash.exe is WSL's and fails with no distribution installed, so it does
// not count.
package shell

import (
	"context"
	"os/exec"
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

var (
	once   sync.Once
	chosen choice
)

func current() choice {
	once.Do(func() { chosen = resolve(runtime.GOOS, exec.LookPath) })
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
	case Bash:
		return ""
	case PowerShell:
		return "The bash tool runs commands with PowerShell on this machine (no bash on PATH): write PowerShell, not POSIX sh; a native command's exit code is $LASTEXITCODE."
	case Cmd:
		return "The bash tool runs commands with cmd.exe on this machine (no bash or PowerShell on PATH): write cmd syntax, not POSIX sh."
	}
	return "The bash tool runs commands with /bin/sh on this machine (no bash on PATH): write POSIX sh."
}

func resolve(goos string, look func(string) (string, error)) choice {
	if p, err := look("bash"); err == nil && !(goos == "windows" && isWSLLauncher(p)) {
		if goos != "windows" {
			// The bare name: bash then reports itself as "bash", as it
			// always did, not as /bin/bash (the apply_patch hint reads it).
			p = "bash"
		}
		return choice{Bash, p, []string{"-c"}}
	}
	if goos == "windows" {
		for _, ps := range []string{"pwsh", "powershell"} {
			if p, err := look(ps); err == nil {
				return choice{PowerShell, p, []string{"-NoProfile", "-NonInteractive", "-Command"}}
			}
		}
		return choice{Cmd, "cmd", []string{"/c"}}
	}
	return choice{Sh, "sh", []string{"-c"}}
}

func isWSLLauncher(path string) bool {
	p := strings.ToLower(strings.ReplaceAll(path, "/", `\`))
	return strings.Contains(p, `\windows\system32\`) || strings.Contains(p, `\windows\sysnative\`)
}

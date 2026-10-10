package tools

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// bash_plain: running a shell command so its output is TEXT.
//
// Tool output is read by a model and drawn in a frame; it never reaches a
// colour terminal. Programs that colourise anyway put escape bytes into the
// model's context — attention spent on nothing. Python 3.13+ colourises
// tracebacks, and git, pytest and cargo all do it when they think they are on a
// tty. Measured live 2026-08-30, a traceback arriving in the transcript as
// `File  [35m"<string>" [0m, line  [35m1 [0m`.
//
// Both halves are here because neither is sufficient. The environment stops
// well-behaved programs from emitting the bytes, which is cheaper than cleaning
// up after them; the strip catches everything that colourises regardless.

// plainEnv is the inherited environment plus the conventional "do not
// colourise" signals. NO_COLOR is the cross-tool convention, TERM=dumb is what
// anything using terminfo checks, and the two specific ones cover the
// offenders measured in practice.
func plainEnv() []string {
	return append(os.Environ(),
		"NO_COLOR=1",
		"TERM=dumb",
		"PYTHON_COLORS=0",
		"CLICOLOR=0",
		"CLICOLOR_FORCE=0",
	)
}

// runPlainBash runs a command through bash and returns its combined output with
// terminal escapes removed. Output is returned even on failure — a command's
// error message IS the useful part — so callers report it rather than discard
// it.
//
// A command gets BashTimeout() to finish. Then its whole process group is
// killed — bash and everything it started — and the output so far comes back
// with the reason. Without this a turn hangs for good on one runaway command:
// measured 2026-09-25, the coder ran `find / -name "*.log"` and `find ~
// -maxdepth 4 …` looking for logs; neither returned, the turn never ended, and
// the finds outlived it by over an hour.
func runPlainBash(command, dir string) (string, error) {
	timeout := BashTimeout()
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = dir
	cmd.Env = plainEnv()
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	// Own process group, so the kill reaches the children, not only bash.
	ownProcessGroup(cmd)
	// A grandchild holding the output pipe must not keep Wait blocked.
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		return "", err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		// Exit 0 with a backgrounded child still on the pipe is a launch,
		// not a failure (see gateway/bash_stream.go).
		if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
			err = nil
		}
		return stripTerminalEscapes([]byte(buf.String())), err
	case <-time.After(timeout):
		_ = killProcessGroup(cmd)
		<-done
		return stripTerminalEscapes([]byte(buf.String())),
			fmt.Errorf("timed out after %s and was killed; narrow it (a path, -maxdepth, head) or run it in the background", timeout)
	}
}

// BashTimeoutLimit is how long a command may run: long enough for a
// race-enabled test build, short of a hung turn. A var so a test can lower it.
var BashTimeoutLimit = 300 * time.Second

// BashTimeout is the limit a command runs under.
func BashTimeout() time.Duration { return BashTimeoutLimit }

// stripTerminalEscapes removes ANSI escape sequences, leaving the text.
//
// Written as a scanner rather than a regexp because it runs on every byte of
// every command's output. It handles the two forms that actually appear: CSI
// (ESC [ … final byte in @-~), which is colour and cursor movement, and OSC
// (ESC ] … BEL or ESC \), which is titles and hyperlinks. A lone ESC before
// anything else is dropped with the byte that follows it.
func stripTerminalEscapes(b []byte) string {
	if !strings.ContainsRune(string(b), 0x1b) {
		return string(b) // the common case: nothing to do, no copy
	}
	var out strings.Builder
	out.Grow(len(b))
	for i := 0; i < len(b); i++ {
		if b[i] != 0x1b {
			out.WriteByte(b[i])
			continue
		}
		if i+1 >= len(b) {
			break // trailing ESC: nothing to keep
		}
		switch b[i+1] {
		case '[': // CSI — runs to a final byte in @-~
			i += 2
			for i < len(b) && (b[i] < '@' || b[i] > '~') {
				i++
			}
		case ']': // OSC — runs to BEL or ST (ESC \)
			i += 2
			for i < len(b) {
				if b[i] == 0x07 {
					break
				}
				if b[i] == 0x1b && i+1 < len(b) && b[i+1] == '\\' {
					i++
					break
				}
				i++
			}
		default:
			i++ // two-byte escape
		}
	}
	return out.String()
}

package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

// Tool output is read by a model and drawn in a frame — never by a colour
// terminal. Programs that colourise anyway (Python 3.13+ colourises
// tracebacks, and git, pytest and cargo all do it) put escape bytes into the
// model's context, where they are noise it has to spend attention on.
// Measured live 2026-08-30, a traceback reaching the transcript as:
//
//	File  [35m"<string>" [0m, line  [35m1 [0m, in  [35m<module> [0m
func TestBashOutputCarriesNoTerminalEscapes(t *testing.T) {
	for _, tc := range []struct{ name, cmd, want string }{
		{"colour codes", `printf '\033[35mmagenta\033[0m plain'`, "magenta plain"},
		{"bold and reset", `printf '\033[1mbold\033[22m done'`, "bold done"},
		{"cursor move", `printf 'a\033[2Kb'`, "ab"},
		{"osc hyperlink", `printf '\033]8;;http://x\033\\link\033]8;;\033\\'`, "link"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// NOT stripped again here: the point is that what runPlainBash
			// RETURNS is already clean.
			got := mustRunBash(t, tc.cmd)
			if !strings.Contains(got, tc.want) {
				t.Errorf("output %q does not contain %q", got, tc.want)
			}
			if strings.ContainsRune(got, 0x1b) {
				t.Errorf("escape byte survived in %q", got)
			}
		})
	}
}

// The environment is the other half: a program told not to colourise never
// emits the bytes in the first place, which is cheaper than cleaning them up
// and is what well-behaved tools honour.
func TestPlainEnvironmentDisablesColour(t *testing.T) {
	env := strings.Join(plainEnv(), "\n")
	for _, want := range []string{"NO_COLOR=1", "TERM=dumb", "PYTHON_COLORS=0", "CLICOLOR=0"} {
		if !strings.Contains(env, want) {
			t.Errorf("plainEnv() does not set %s", want)
		}
	}
	if !strings.Contains(env, "PATH=") {
		t.Error("plainEnv() dropped the inherited environment — commands would not resolve")
	}
}

// And it has to REACH the command: a plainEnv that is built and not applied
// leaves every well-behaved program colourising exactly as before.
func TestTheCommandActuallySeesThePlainEnvironment(t *testing.T) {
	got := mustRunBash(t, `echo "NO_COLOR=$NO_COLOR TERM=$TERM PYTHON_COLORS=$PYTHON_COLORS"`)
	for _, want := range []string{"NO_COLOR=1", "TERM=dumb", "PYTHON_COLORS=0"} {
		if !strings.Contains(got, want) {
			t.Errorf("the command did not see %s — plainEnv() is not applied (saw %q)", want, strings.TrimSpace(got))
		}
	}
	if out := mustRunBash(t, "echo $PATH"); strings.TrimSpace(out) == "" {
		t.Error("the command inherited no PATH")
	}
}

// Real text must survive untouched: stripping is not licence to mangle output.
func TestOrdinaryOutputIsUnchanged(t *testing.T) {
	in := "package main\n\nfunc main() {\n\tprintln(\"hi [1] a[0]b\")\n}\n"
	if got := stripTerminalEscapes([]byte(in)); got != in {
		t.Errorf("ordinary text was altered:\n%q\n%q", in, got)
	}
}

func mustRunBash(t *testing.T, cmd string) string {
	t.Helper()
	out, err := runPlainBash(cmd, "", BashTimeout())
	if err != nil {
		t.Fatalf("bash %q: %v", cmd, err)
	}
	return out
}

// A binary file catted into a conversation (a git index, live 2026-11-23)
// made a 27B model answer in imitation of it until the step timed out. The
// text around it stays; each run of binary lines becomes one note.
func TestBashOutputNamesBinaryInsteadOfShowingIt(t *testing.T) {
	for _, tc := range []struct{ name, cmd, want string }{
		{"a git index between text", `printf 'before\nDIRC\000\000\000\002k\004\n\377\376TREE\n---\nafter\n'`,
			"before\n[binary data, 18 bytes, not shown]\n---\nafter"},
		{"binary at the end, no newline", `printf 'ok\n\000\001'`, "ok\n[binary data, 2 bytes, not shown]"},
		{"text in another script", `printf 'héllo — 日本語\n'`, "héllo — 日本語"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Bash([]byte(`{"command":` + jsonString(tc.cmd) + `}`))
			if err != nil {
				t.Fatal(err)
			}
			if out != tc.want {
				t.Errorf("got %q, want %q", out, tc.want)
			}
		})
	}
}

func jsonString(s string) string { b, _ := json.Marshal(s); return string(b) }

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// localRuntimeRow is one model server the gateway found running.
type localRuntimeRow struct {
	Name   string `json:"name"`
	Base   string `json:"base"`
	Models int    `json:"models"`
}

type localRuntimeList struct {
	Runtimes []localRuntimeRow `json:"runtimes"`
}

// runtimeWhere says where the gateway looked: this computer when the gateway
// is here, else the machine it runs on.
func runtimeWhere(gateway string) string {
	for _, here := range []string{"localhost", "127.0.0.1", "[::1]"} {
		if strings.Contains(gateway, here) {
			return "this computer"
		}
	}
	return "the gateway's machine"
}

// localRuntimeMenu is the numbered choice `memdoor connect local` shows: each
// runtime that answers, then an address of one's own.
func localRuntimeMenu(rs []localRuntimeRow, where string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Model servers running on %s:\n", where)
	width := 0
	for _, r := range rs {
		width = max(width, len(r.Name))
	}
	for i, r := range rs {
		noun := "models"
		if r.Models == 1 {
			noun = "model"
		}
		fmt.Fprintf(&b, "  %d. %-*s  %s  %d %s\n", i+1, width, r.Name, r.Base, r.Models, noun)
	}
	fmt.Fprintf(&b, "  %d. Another address\n", len(rs)+1)
	return b.String()
}

// janisInstall is the one-line installer of janis's release, for Linux and
// macOS; janis serve --detach pulls the model and returns once it has answered
// (guregodevo/janis README).
const janisInstall = "curl -fsSL " + janisReleaseURL + "install.sh | sh"

// janisHardware says what janis runs on, by OS: its release is Linux and macOS.
var janisHardware = map[string]string{
	"linux":  "no GPU needed, slow without one",
	"darwin": "MLX on Apple silicon",
}

// localRuntimeChoice is one runtime a person can run when none answers: its
// name and how to start it.
type localRuntimeChoice struct{ name, how string }

// otherRuntimes are the runtimes after janis, with the command that starts them.
var otherRuntimes = []localRuntimeChoice{
	{"Ollama", "ollama pull qwen3:8b"},
	{"llama.cpp", "llama-server -m model.gguf -c 32768"},
	{"LM Studio", "load a model, start the server"},
	{"vLLM", "vllm serve <model>"},
}

// noLocalRuntimeMenu is the numbered choice when nothing answers: janis first
// and by default, installed from here when the gateway is on this machine and
// janis's release runs on its OS (canInstall), the other runtimes with the
// command that starts them, then an address of one's own. goos is the OS of
// the machine the gateway runs on; "" when that is not this one.
func noLocalRuntimeMenu(where, goos string, canInstall bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "No model is running on %s. Run one:\n", where)
	janis := "github.com/guregodevo/janis, then run this again"
	if canInstall {
		janis = "installed now, with the model that fits this machine (" + janisHardware[goos] + ")"
	}
	rows := append([]localRuntimeChoice{{"janis", janis}}, otherRuntimes...)
	for i, r := range rows {
		fmt.Fprintf(&b, "  %d. %-10s %s\n", i+1, r.name, r.how)
	}
	fmt.Fprintf(&b, "  %d. Another address\n", len(rows)+1)
	b.WriteString("  No model on this machine: a key instead, memdoor connect openrouter\n")
	return b.String()
}

// hereOS is this machine's OS when the gateway runs on it, else "".
func hereOS(where string) string {
	if where == "this computer" {
		return runtime.GOOS
	}
	return ""
}

var errNoRuntimeChosen = errors.New("nothing connected: no address given")

// chooseLocalRuntime asks the gateway which runtimes answer and lets the
// person pick by number. It returns the base to connect to. A gateway that
// cannot list them (an older one) gets the plain address prompt.
func chooseLocalRuntime(ask func(prompt, def string) string, say func(string), offer func() error) (string, error) {
	list, err := listLocalRuntimes()
	if err != nil {
		return ask("Address of the model server (blank: look for one)", ""), nil
	}
	where := runtimeWhere(gatewayAddr)
	if len(list.Runtimes) == 0 {
		say(noLocalRuntimeMenu(where, hereOS(where), offer != nil))
		n, err := strconv.Atoi(ask("Number", "1"))
		switch {
		case err != nil || n < 1 || n > len(otherRuntimes)+2:
			return "", fmt.Errorf("pick a number from the list")
		case n == 1 && offer != nil:
			if err := offer(); err != nil {
				say("✗ " + err.Error() + "\n")
			} else if again, err := listLocalRuntimes(); err == nil && len(again.Runtimes) > 0 {
				say("✓ " + again.Runtimes[0].Name + " is running at " + again.Runtimes[0].Base + "\n")
				return again.Runtimes[0].Base, nil
			}
		case n == 1:
			return "", fmt.Errorf("install janis on %s (%s), then run memdoor connect local again", where, janisInstall)
		case n <= len(otherRuntimes)+1:
			r := otherRuntimes[n-2]
			return "", fmt.Errorf("start %s (%s), then run memdoor connect local again", r.name, r.how)
		}
		if base := ask("Address (Enter to cancel)", ""); base != "" {
			return base, nil
		}
		return "", errNoRuntimeChosen
	}
	say(localRuntimeMenu(list.Runtimes, where))
	n, err := strconv.Atoi(ask("Number", "1"))
	if err != nil || n < 1 || n > len(list.Runtimes)+1 {
		return "", fmt.Errorf("pick a number from the list")
	}
	if n <= len(list.Runtimes) {
		return list.Runtimes[n-1].Base, nil
	}
	if base := ask("Address, e.g. http://localhost:11434/v1", ""); base != "" {
		return base, nil
	}
	return "", errNoRuntimeChosen
}

func listLocalRuntimes() (localRuntimeList, error) {
	var list localRuntimeList
	err := NewClient().GetJSON("/api/providers/connect", &list)
	return list, err
}

// janisHome is where janis's installer puts the binary; the shell expands it.
const janisHome = "$HOME/.local/bin/janis"

// janisPulled reads the model name from `janis pull`'s last line, "✓ NAME
// installed" (or "is installed" when it was there already). Only name
// characters, since it goes back into a shell command.
var janisPulled = regexp.MustCompile(`✓ ([A-Za-z0-9._-]+) (?:is )?installed`)

// runShell runs a command line with the person's terminal, copying its output
// to tee when given.
type runShell func(script string, tee io.Writer) error

func shellRunner(script string, tee io.Writer) error {
	cmd := exec.Command("sh", "-c", script)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if tee != nil {
		cmd.Stdout = io.MultiWriter(os.Stdout, tee)
	}
	return cmd.Run()
}

// installJanis is what the "nothing running" text shows: the installer unless
// janis is there (janis is its path, "" when it is not), an update when the
// janis there is older than the latest release (rel), then the model that
// fits this machine, then the server in the background. It returns once the
// server answered, so the connect that follows finds it.
func installJanis(run runShell, janis string, rel janisRelease) error {
	if janis == "" {
		if err := run(janisInstall, nil); err != nil {
			return fmt.Errorf("the janis installer did not finish: %w", err)
		}
		rel.keep("")
		janis = janisHome
	} else {
		rel.renew(run, janis)
	}
	var pulled bytes.Buffer
	if err := run(janis+" pull", &pulled); err != nil {
		return fmt.Errorf("janis could not fetch a model: %w", err)
	}
	serve := janis + " serve --detach"
	if m := janisPulled.FindAllStringSubmatch(pulled.String(), -1); len(m) > 0 {
		serve += " --model " + m[len(m)-1][1]
	}
	if err := run(serve, nil); err != nil {
		return fmt.Errorf("janis did not start: %w", err)
	}
	return nil
}

// janisOffer is the install on offer, or nil: only with the gateway on this
// machine and an OS the janis release runs on.
func janisOffer() func() error {
	if _, ok := janisHardware[hereOS(runtimeWhere(gatewayAddr))]; !ok {
		return nil
	}
	return func() error {
		path, _ := exec.LookPath("janis")
		return installJanis(shellRunner, path, thisJanisRelease(func(s string) { fmt.Print(s) }))
	}
}

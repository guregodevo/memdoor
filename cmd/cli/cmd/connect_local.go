package cmd

import (
	"errors"
	"fmt"
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

// janisInstall is the one-line Linux installer; janis serve --detach pulls the
// model and returns once it has answered (guregodevo/janis README).
const janisInstall = "curl -fsSL https://github.com/guregodevo/janis/releases/latest/download/install.sh | sh"

// noLocalRuntimeText is said when nothing answers. It names janis with the
// command for this OS, and a key for a machine that cannot run a model well.
// goos is the OS of the machine the gateway runs on; "" when that is not this one.
func noLocalRuntimeText(where, goos string) string {
	var b strings.Builder
	b.WriteString("No model is running on " + where + ".\n")
	switch goos {
	case "linux":
		b.WriteString("  Run one with janis (Linux, no GPU needed, slow without one):\n")
		b.WriteString("    " + janisInstall + "\n")
		b.WriteString("    janis serve --detach --model Qwen3-8B-4bit\n")
		b.WriteString("  Then run this again. Ollama, LM Studio, llama.cpp and vLLM work too.\n")
	case "darwin":
		b.WriteString("  Run one with Ollama:  ollama pull qwen3:8b   (ollama.com)\n")
		b.WriteString("  or build janis for MLX: https://github.com/guregodevo/janis#build\n")
		b.WriteString("  Then run this again. LM Studio, llama.cpp and vLLM work too.\n")
	default:
		b.WriteString("  Start janis, Ollama, LM Studio, llama.cpp or vLLM on that machine and run this again.\n")
	}
	b.WriteString("  Or type its address below. No model here? Use a key: memdoor connect openrouter\n")
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
func chooseLocalRuntime(ask func(prompt, def string) string, say func(string)) (string, error) {
	var list localRuntimeList
	if err := NewClient().GetJSON("/api/providers/connect", &list); err != nil {
		return ask("Address of the model server (blank: look for one)", ""), nil
	}
	where := runtimeWhere(gatewayAddr)
	if len(list.Runtimes) == 0 {
		say(noLocalRuntimeText(where, hereOS(where)))
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

package cmd

import (
	"errors"
	"fmt"
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

// noLocalRuntimeText is said when nothing answers. A key is the way out for a
// machine that cannot run a model.
func noLocalRuntimeText(where string) string {
	return "No model is running on " + where + ".\n" +
		"  Start one (Ollama, LM Studio, llama.cpp, vLLM, janis) and run this again, or type its address.\n" +
		"  No model here? Use a key: memdoor connect openrouter\n"
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
		say(noLocalRuntimeText(where))
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

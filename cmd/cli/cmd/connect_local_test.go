package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalRuntimeMenuListsEachAndAnotherAddress(t *testing.T) {
	got := localRuntimeMenu([]localRuntimeRow{
		{Name: "Ollama", Base: "http://localhost:11434/v1", Models: 3},
		{Name: "vLLM", Base: "http://localhost:8000/v1", Models: 1},
	}, "this computer")
	for _, want := range []string{"running on this computer", "1. Ollama", "http://localhost:11434/v1  3 models", "2. vLLM", "1 model\n", "3. Another address"} {
		if !strings.Contains(got, want) {
			t.Errorf("menu lacks %q:\n%s", want, got)
		}
	}
}

func TestNothingRunningIsSaidInWordsWithAWayOut(t *testing.T) {
	for goos, wants := range map[string][]string{
		"linux":  {"No model is running on this computer.", janisInstall, "janis serve --detach --model Qwen3-8B-4bit", "memdoor connect openrouter"},
		"darwin": {"ollama pull qwen3:8b", "janis#build", "memdoor connect openrouter"},
		"":       {"Start janis, Ollama", "memdoor connect openrouter"},
	} {
		got := noLocalRuntimeText("this computer", goos)
		for _, want := range wants {
			if !strings.Contains(got, want) {
				t.Errorf("%q text lacks %q:\n%s", goos, want, got)
			}
		}
	}
	if strings.Contains(noLocalRuntimeText("the gateway's machine", hereOS("the gateway's machine")), "curl -fsSL") {
		t.Error("a gateway elsewhere gets no command for this machine's OS")
	}
	if runtimeWhere("http://localhost:18789") != "this computer" || runtimeWhere("http://box.example:18789") != "the gateway's machine" {
		t.Error("a gateway here is this computer, any other is its own machine")
	}
}

func TestChooseLocalRuntime(t *testing.T) {
	var body string
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	saved := gatewayAddr
	gatewayAddr = srv.URL
	defer func() { gatewayAddr = saved }()

	answers := func(a ...string) func(string, string) string {
		return func(prompt, def string) string {
			if len(a) == 0 {
				return def
			}
			v := a[0]
			a = a[1:]
			if v == "" {
				return def
			}
			return v
		}
	}
	var said strings.Builder
	say := func(s string) { said.WriteString(s) }

	body = `{"runtimes":[{"name":"Ollama","base":"http://localhost:11434/v1","models":3}]}`
	if base, err := chooseLocalRuntime(answers(""), say); err != nil || base != "http://localhost:11434/v1" {
		t.Fatalf("Enter takes the first runtime: %q %v", base, err)
	}
	if base, err := chooseLocalRuntime(answers("2", "http://box:9000/v1"), say); err != nil || base != "http://box:9000/v1" {
		t.Fatalf("Another address asks for one: %q %v", base, err)
	}
	if _, err := chooseLocalRuntime(answers("7"), say); err == nil {
		t.Fatal("a number off the list is refused")
	}

	said.Reset()
	body = `{"runtimes":[]}`
	if _, err := chooseLocalRuntime(answers(""), say); err != errNoRuntimeChosen {
		t.Fatalf("nothing running and no address cancels: %v", err)
	}
	if !strings.Contains(said.String(), "No model is running") {
		t.Fatalf("nothing running is said: %q", said.String())
	}

	status = http.StatusMethodNotAllowed // a gateway from before the list
	if base, err := chooseLocalRuntime(answers("http://x:1/v1"), say); err != nil || base != "http://x:1/v1" {
		t.Fatalf("an older gateway gets the plain address prompt: %q %v", base, err)
	}
}

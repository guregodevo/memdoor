package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
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
		"linux":  {"No model is running on this computer. Run one:", "1. janis", "installed now", "no GPU needed", "2. Ollama", "ollama pull qwen3:8b", "5. vLLM", "6. Another address", "memdoor connect openrouter"},
		"darwin": {"1. janis", "installed now", "MLX on Apple silicon", "6. Another address"},
	} {
		got := noLocalRuntimeMenu("this computer", goos, true)
		for _, want := range wants {
			if !strings.Contains(got, want) {
				t.Errorf("%q menu lacks %q:\n%s", goos, want, got)
			}
		}
	}
	elsewhere := noLocalRuntimeMenu("the gateway's machine", "", false)
	if !strings.Contains(elsewhere, "1. janis") || !strings.Contains(elsewhere, "github.com/guregodevo/janis") || strings.Contains(elsewhere, "installed now") {
		t.Errorf("a gateway elsewhere gets janis to install there, no install from here:\n%s", elsewhere)
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
	if base, err := chooseLocalRuntime(answers(""), say, nil); err != nil || base != "http://localhost:11434/v1" {
		t.Fatalf("Enter takes the first runtime: %q %v", base, err)
	}
	if base, err := chooseLocalRuntime(answers("2", "http://box:9000/v1"), say, nil); err != nil || base != "http://box:9000/v1" {
		t.Fatalf("Another address asks for one: %q %v", base, err)
	}
	if _, err := chooseLocalRuntime(answers("7"), say, nil); err == nil {
		t.Fatal("a number off the list is refused")
	}

	said.Reset()
	body = `{"runtimes":[]}`
	if _, err := chooseLocalRuntime(answers("6", ""), say, nil); err != errNoRuntimeChosen {
		t.Fatalf("nothing running, another address, none given: cancels: %v", err)
	}
	if _, err := chooseLocalRuntime(answers(""), say, nil); err == nil || !strings.Contains(err.Error(), "install janis on this computer") {
		t.Fatalf("Enter is janis; with no install from here it says where to install it: %v", err)
	}
	if !strings.Contains(said.String(), "No model is running") {
		t.Fatalf("nothing running is said: %q", said.String())
	}

	status = http.StatusMethodNotAllowed // a gateway from before the list
	if base, err := chooseLocalRuntime(answers("http://x:1/v1"), say, nil); err != nil || base != "http://x:1/v1" {
		t.Fatalf("an older gateway gets the plain address prompt: %q %v", base, err)
	}
}

func TestInstallJanisRunsTheInstallerThePullAndTheServer(t *testing.T) {
	var ran []string
	run := func(script string, tee io.Writer) error {
		ran = append(ran, script)
		if strings.HasSuffix(script, " pull") && tee != nil {
			_, _ = io.WriteString(tee, "no model named: Qwen3-1.7B-4bit fits this machine (8 GiB)\n✓ Qwen3-1.7B-4bit installed · janis serve --model Qwen3-1.7B-4bit\n")
		}
		return nil
	}
	if err := installJanis(run, "", janisRelease{}); err != nil {
		t.Fatal(err)
	}
	want := []string{janisInstall, janisHome + " pull", janisHome + " serve --detach --model Qwen3-1.7B-4bit"}
	if fmt.Sprint(ran) != fmt.Sprint(want) {
		t.Fatalf("no janis yet: installer, pull, serve the model it pulled:\n got %q\nwant %q", ran, want)
	}

	ran = nil
	if err := installJanis(run, "/usr/local/bin/janis", janisRelease{}); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 2 || ran[0] != "/usr/local/bin/janis pull" {
		t.Fatalf("janis already there: no installer, its own path used: %q", ran)
	}

	ran = nil
	failPull := func(script string, tee io.Writer) error {
		ran = append(ran, script)
		if strings.HasSuffix(script, " pull") {
			return errors.New("exit status 1")
		}
		return nil
	}
	if err := installJanis(failPull, "janis", janisRelease{}); err == nil || len(ran) != 1 {
		t.Fatalf("a failed pull stops before the server: %v %q", err, ran)
	}

	ran = nil
	noName := func(script string, tee io.Writer) error { ran = append(ran, script); return nil }
	_ = installJanis(noName, "janis", janisRelease{})
	if ran[len(ran)-1] != "janis serve --detach" {
		t.Fatalf("a pull that names no model serves without one: %q", ran)
	}
}

func TestOfferedInstallIsAskedAndTakenOnYes(t *testing.T) {
	body := `{"runtimes":[]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) }))
	defer srv.Close()
	saved := gatewayAddr
	gatewayAddr = srv.URL
	defer func() { gatewayAddr = saved }()
	var said strings.Builder
	say := func(s string) { said.WriteString(s) }

	calls := 0
	install := func() error {
		calls++
		body = `{"runtimes":[{"name":"janis","base":"http://localhost:8090/v1","models":1}]}`
		return nil
	}
	ask := func(prompt, def string) string { return def }
	base, err := chooseLocalRuntime(ask, say, install)
	if err != nil || base != "http://localhost:8090/v1" || calls != 1 {
		t.Fatalf("Enter at the offer installs and connects to what now runs: %q %v (%d calls)", base, err, calls)
	}

	calls, body = 0, `{"runtimes":[]}`
	another := func(prompt, def string) string {
		if prompt == "Number" {
			return "6"
		}
		return "http://x:1/v1"
	}
	if base, _ := chooseLocalRuntime(another, say, install); base != "http://x:1/v1" || calls != 0 {
		t.Fatalf("6 installs nothing and asks for an address: %q (%d calls)", base, calls)
	}
	ollama := func(prompt, def string) string { return "2" }
	if _, err := chooseLocalRuntime(ollama, say, install); err == nil || !strings.Contains(err.Error(), "start Ollama (ollama pull qwen3:8b)") || calls != 0 {
		t.Fatalf("2 says how to start Ollama and installs nothing: %v (%d calls)", err, calls)
	}

	said.Reset()
	bad := func() error { return errors.New("the janis installer did not finish") }
	chooseLocalRuntime(func(p, d string) string { return d }, say, bad)
	if !strings.Contains(said.String(), "✗ the janis installer did not finish") {
		t.Fatalf("a failed install is said: %q", said.String())
	}
	if janisOffer() != nil && (runtimeWhere(gatewayAddr) != "this computer") {
		t.Fatal("no offer for a gateway elsewhere")
	}
}

func TestJanisIsOfferedOnLinuxAndMacOnly(t *testing.T) {
	saved := gatewayAddr
	defer func() { gatewayAddr = saved }()
	gatewayAddr = "http://localhost:18789"
	_, known := janisHardware[runtime.GOOS]
	if (janisOffer() != nil) != known {
		t.Fatalf("on %s the offer is %v, janis has a release for it: %v", runtime.GOOS, janisOffer() != nil, known)
	}
	gatewayAddr = "http://box.example:18789"
	if janisOffer() != nil {
		t.Fatal("no offer for a gateway elsewhere")
	}
}

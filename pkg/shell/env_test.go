package shell

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

// A command the agent runs sees no model key, whatever it does with its
// environment, and keeps everything else.
func TestCommandHidesModelKeys(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("printenv is POSIX")
	}
	t.Setenv("OPEN_ROUTER_API_KEY", "sk-or-test-should-not-leak")
	t.Setenv("ACME_AI_GATEWAY_TOKEN", "gw-test-should-not-leak")
	t.Setenv("MEMDOOR_ENV_TEST_KEPT", "kept")
	out, err := Command(context.Background(), "printenv").Output()
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, leaked := range []string{"should-not-leak", "OPEN_ROUTER_API_KEY", "ACME_AI_GATEWAY_TOKEN"} {
		if strings.Contains(s, leaked) {
			t.Fatalf("the command saw %s", leaked)
		}
	}
	if !strings.Contains(s, "MEMDOOR_ENV_TEST_KEPT=kept") || !strings.Contains(s, "PATH=") {
		t.Fatal("the command lost a variable that is not a model key")
	}
}

func TestIsModelKey(t *testing.T) {
	for _, k := range []string{"OPEN_ROUTER_API_KEY", "ANTHROPIC_API_KEY", "MEMDOOR_SYSTEMONE_API_KEY", "CORP_AI_GATEWAY_TOKEN", "AI_GATEWAY_TOKEN"} {
		if !IsModelKey(k) {
			t.Errorf("%s is a model key", k)
		}
	}
	for _, k := range []string{"PATH", "HOME", "GITHUB_TOKEN", "STRIPE_API_KEY", "MEMDOOR_TEST_COMMAND"} {
		if IsModelKey(k) {
			t.Errorf("%s is not a model key", k)
		}
	}
}

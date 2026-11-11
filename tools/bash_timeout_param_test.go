package tools

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func restoreBashTimeouts(t *testing.T) {
	limit, most := BashTimeoutLimit, BashTimeoutMax
	t.Cleanup(func() { BashTimeoutLimit, BashTimeoutMax = limit, most; SetBashTimeouts(0, 0) })
}

// With no config a call cannot ask for longer, and the tool is as it was.
func TestABashCallCannotAskForLongerByDefault(t *testing.T) {
	restoreBashTimeouts(t)
	SetBashTimeouts(0, 0)
	if got := ParseBashTimeout(json.RawMessage(`{"command":"x","timeout":3600}`)); got != 300*time.Second {
		t.Fatalf("got %s", got)
	}
	schema, _ := json.Marshal(BashDefinition.InputSchema)
	if strings.Contains(string(schema), "timeout") || BashDefinition.Description != bashDescription || BashLongerHint() != "" {
		t.Fatalf("the default tool offers a timeout: %s / %q", schema, BashDefinition.Description)
	}
}

// With a maximum in the config a call may ask for up to it, and the tool says so.
func TestABashCallAsksForLongerUpToTheConfiguredMaximum(t *testing.T) {
	restoreBashTimeouts(t)
	SetBashTimeouts(0, 1200*time.Second)
	for in, want := range map[string]time.Duration{
		`{"command":"x"}`:                300 * time.Second,
		`{"command":"x","timeout":900}`:  900 * time.Second,
		`{"command":"x","timeout":9000}`: 1200 * time.Second,
		`{"command":"x","timeout":-5}`:   300 * time.Second,
	} {
		if got := ParseBashTimeout(json.RawMessage(in)); got != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
	schema, _ := json.Marshal(BashDefinition.InputSchema)
	if !strings.Contains(string(schema), `"timeout"`) || !strings.Contains(string(schema), `"command"`) {
		t.Fatalf("schema %s", schema)
	}
	if !strings.Contains(BashDefinition.Description, "at most 1200") || !strings.Contains(BashLongerHint(), "at most 1200") {
		t.Fatalf("description %q", BashDefinition.Description)
	}
}

// A configured default below the maximum is the limit of a call that asks for none.
func TestTheConfiguredDefaultIsTheLimitOfACallThatAsksForNone(t *testing.T) {
	restoreBashTimeouts(t)
	SetBashTimeouts(60*time.Second, 0)
	if got := ParseBashTimeout(json.RawMessage(`{"command":"x","timeout":500}`)); got != 60*time.Second {
		t.Fatalf("got %s", got)
	}
}

// The asked-for limit is the one the command runs under.
func TestABashCallRunsUnderTheLimitItAskedFor(t *testing.T) {
	restoreBashTimeouts(t)
	BashTimeoutLimit = 200 * time.Millisecond
	SetBashTimeouts(0, 5*time.Second)
	out, err := Bash(json.RawMessage(`{"command":"sleep 1; echo finished","timeout":3}`))
	if err != nil || !strings.Contains(out, "finished") {
		t.Fatalf("%q %v", out, err)
	}
	out, err = Bash(json.RawMessage(`{"command":"sleep 1; echo finished"}`))
	if !strings.Contains(out+errText(err), "timed out after 200ms") {
		t.Fatalf("a call that asked for nothing runs under the default: %q %v", out, err)
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

package ui

import (
	"strings"
	"testing"
)

func promptModel(got *map[string]string) *Model {
	return &Model{mcp: MCPOps{GetPrompt: func(server, name string, args map[string]string) (string, error) {
		*got = args
		return "filled", nil
	}}}
}

// A prompt's arguments: in the order it declares them, or name=value; words
// past the last fill it; a missing required one says the usage instead.
func TestMCPPromptArguments(t *testing.T) {
	p := MCPPrompt{Server: "everything", Name: "args-prompt", Args: []MCPPromptArg{{Name: "city", Required: true}, {Name: "state"}}}
	for _, c := range []struct {
		words []string
		want  string
	}{
		{[]string{"Paris", "TX"}, "city=Paris state=TX"},
		{[]string{"state=TX", "city=Paris"}, "city=Paris state=TX"},
		{[]string{"New", "York", "City"}, "city=New state=York City"},
		{[]string{"state=TX", "Lyon"}, "city=Lyon state=TX"},
	} {
		var got map[string]string
		m := promptModel(&got)
		cmd := m.runMCPPrompt(p, c.words)
		if cmd == nil {
			t.Fatalf("%v: no command", c.words)
		}
		cmd()
		if s := "city=" + got["city"] + " state=" + got["state"]; s != c.want {
			t.Errorf("%v: %s, want %s", c.words, s, c.want)
		}
	}
	var got map[string]string
	m := promptModel(&got)
	if cmd := m.runMCPPrompt(p, []string{"state=TX"}); cmd != nil {
		t.Fatal("a missing required argument must not run")
	}
	if last := m.messages[len(m.messages)-1].Content; !strings.Contains(last, "/everything:args-prompt <city> [state]") {
		t.Fatalf("the usage must be shown: %q", last)
	}
}

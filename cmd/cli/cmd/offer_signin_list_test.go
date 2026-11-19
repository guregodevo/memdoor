package cmd

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
	"testing"

	"memdoor/cmd/tui/ui"
)

// A first `memdoor run` with no key offered two sign-ins and left every
// other provider behind "n" (2026-11-20, "Not just openrouter"). The question
// is now the list `memdoor connect` prints: every kind is on it, Enter is
// still Sign in with OpenRouter, and a number runs that kind's own connect.
func TestTheNoModelQuestionListsEveryProvider(t *testing.T) {
	var out bytes.Buffer
	menu := connectMenu(nil, nil, &out)
	if len(menu) != len(ui.ConnectKinds) {
		t.Fatalf("the list has %d entries for %d kinds", len(menu), len(ui.ConnectKinds))
	}
	if menu[0].kind.ID != ui.KindOpenRouterLogin {
		t.Fatalf("Enter picks %q, not the OpenRouter sign-in", menu[0].kind.ID)
	}
	for _, k := range ui.ConnectKinds {
		if !strings.Contains(out.String(), k.Name) {
			t.Fatalf("%q is not on the list:\n%s", k.Name, out.String())
		}
	}

	keyed := -1
	for i, e := range menu {
		if !signsIn(e.kind.ID) {
			keyed = i
			break
		}
	}
	var asked []string
	real := connectKind
	connectKind = func(kind string) error { asked = append(asked, kind); return nil }
	t.Cleanup(func() { connectKind = real })

	offerSignIn(bufio.NewReader(strings.NewReader("n\n")))
	if len(asked) != 0 {
		t.Fatalf("n connected %v", asked)
	}
	offerSignIn(bufio.NewReader(strings.NewReader("99\n")))
	if len(asked) != 0 {
		t.Fatalf("a number off the list connected %v", asked)
	}
	offerSignIn(bufio.NewReader(strings.NewReader(strconv.Itoa(keyed+1) + "\n")))
	if len(asked) != 1 || asked[0] != menu[keyed].kind.ID {
		t.Fatalf("number %d asked for %v, want %q", keyed+1, asked, menu[keyed].kind.ID)
	}
}

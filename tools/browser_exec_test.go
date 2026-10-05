package tools

import (
	"context"
	"errors"
	"testing"
)

// exec takes what the chrome skill shows, a bare expression, and runs it as
// the function evaluate_script wants (live 2026-09-30: "fn is not a
// function" on `exec document.body.innerText`).
func TestExecWrapsABareExpression(t *testing.T) {
	for script, want := range map[string]string{
		"document.body.innerText":             "() => (document.body.innerText)",
		"() => document.title":                "() => document.title",
		"async () => { await x(); return 1 }": "async () => { await x(); return 1 }",
		"function () { return 1 }":            "function () { return 1 }",
		"el => el.textContent":                "el => el.textContent",
	} {
		var got string
		_, _ = evaluateScript(context.Background(), func(_ context.Context, s string) (interface{}, error) {
			got = s
			return "ok", nil
		}, script)
		if got != want {
			t.Errorf("%q ran as %q, want %q", script, got, want)
		}
	}
}

// Several statements are not an expression: the browser's syntax error
// sends them again as a function body.
func TestExecRunsStatementsAsABody(t *testing.T) {
	var ran []string
	_, err := evaluateScript(context.Background(), func(_ context.Context, s string) (interface{}, error) {
		ran = append(ran, s)
		if len(ran) == 1 {
			return nil, errors.New("evaluate: SyntaxError: Unexpected token ';'")
		}
		return "ok", nil
	}, "const a = 1; return a")
	if err != nil || len(ran) != 2 || ran[1] != "() => { const a = 1; return a }" {
		t.Fatalf("ran %q, err %v", ran, err)
	}
}

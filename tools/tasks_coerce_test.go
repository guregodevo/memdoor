package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

// The shapes a small model actually sends when asked for a checklist. Each was a hard
// error before — one wasted round trip apiece, and no checklist for the user.
func TestCoerceTodosRescuesRealShapes(t *testing.T) {
	cases := map[string]string{
		"content as one string":  `{"content":"Read the file, add the function, run it"}`,
		"todos as one string":    `{"todos":"Read the file\nAdd the function\nRun it"}`,
		"todos as bare strings":  `{"todos":["Read the file","Add the function","Run it"]}`,
		"items instead of todos": `{"items":["Read the file","Add the function","Run it"]}`,
		"numbered in one string": `{"content":"1. Read the file\n2. Add the function\n3. Run it"}`,
		"bulleted in one string": `{"content":"- Read the file\n- Add the function\n- Run it"}`,
	}
	for name, raw := range cases {
		todos := CoerceTodos(json.RawMessage(raw))
		if len(todos) != 3 {
			t.Errorf("%s: got %d todos, want 3 (%s)", name, len(todos), raw)
			continue
		}
		if !strings.Contains(todos[0].Content, "Read the file") {
			t.Errorf("%s: first step is %q", name, todos[0].Content)
		}
		// A list with nothing in progress reads as "not started" — the first
		// step is the live one.
		if todos[0].Status != "in_progress" {
			t.Errorf("%s: first step status = %q, want in_progress", name, todos[0].Status)
		}
		if todos[2].Status != "pending" {
			t.Errorf("%s: last step status = %q, want pending", name, todos[2].Status)
		}
	}
}

// Coercion must not invent a list out of nothing — a genuinely empty call
// still deserves the error that teaches the schema.
func TestCoerceTodosRefusesNothing(t *testing.T) {
	for _, raw := range []string{`{}`, `{"todos":[]}`, `{"content":""}`, `{"content":"   "}`, `not json`} {
		if got := CoerceTodos(json.RawMessage(raw)); len(got) != 0 {
			t.Errorf("CoerceTodos(%s) invented %d todos", raw, len(got))
		}
	}
}

// A well-formed call is untouched by any of this.
func TestWellFormedTodosStillParse(t *testing.T) {
	raw := `{"todos":[{"content":"Read","status":"completed","active_form":"Reading"},
	                  {"content":"Write","status":"in_progress","active_form":"Writing"}]}`
	var p TodoWriteInput
	if err := json.Unmarshal([]byte(raw), &p); err != nil || len(p.Todos) != 2 {
		t.Fatalf("well-formed input should parse directly: %v", err)
	}
	if p.Todos[0].Status != "completed" {
		t.Errorf("status lost: %+v", p.Todos[0])
	}
}

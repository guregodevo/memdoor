package gateway

import (
	"encoding/json"
	"testing"
)

// A CALL WITH NO ARGUMENTS IS NOTHING, NOT A FAILURE: the brain emits
// {"arguments":{}} before the real call, and three of those tripped the
// loop-breaker and killed a turn. A tool that takes no input is
// untouched. Mutation check: say every tool needs input and todo_read's
// legitimate empty call is refused.
func TestACallWithNoArgumentsIsNothingNotAFailure(t *testing.T) {
	for _, name := range []string{"notes", "todo_write", "bash"} {
		if !toolNeedsInput(name) {
			t.Fatalf("%s cannot do anything without arguments", name)
		}
	}
	for _, name := range []string{"todo_read", "ask_user_question"} {
		if toolNeedsInput(name) {
			t.Fatalf("%s may legitimately be called with none", name)
		}
	}
	for _, in := range []string{"{}", "", "  {  }  "} {
		if !emptyToolInput(json.RawMessage(in)) {
			t.Fatalf("%q is no arguments", in)
		}
	}
	if emptyToolInput(json.RawMessage(`{"append":"x"}`)) {
		t.Fatal("a call with arguments is a call")
	}
}

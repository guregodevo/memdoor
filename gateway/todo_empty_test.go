package gateway

import (
	"encoding/json"
	"testing"
)

// AN EMPTY todo_write IS NOTHING, NOT A FAILURE. Mutation check: call
// nothing empty and {} is a failed call again.
func TestAnEmptyTodoWriteIsNothingNotAFailure(t *testing.T) {
	for _, in := range []string{"{}", "", " { } "} {
		if !emptyToolInput(json.RawMessage(in)) {
			t.Fatalf("%q is empty", in)
		}
	}
	if emptyToolInput(json.RawMessage(`{"todos":[]}`)) {
		t.Fatal("a shape with a key is not empty — it is wrong, and the tool says how")
	}
}

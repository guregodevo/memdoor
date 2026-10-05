package gateway

import (
	"testing"

	"memdoor/pkg/llm"
)

// The same call in one reply is found however it was spelled: a repeat with
// its fields reordered or respaced is the same call and runs once; a call
// with any value different is another call and runs.
func TestToolCallKeyIsByContent(t *testing.T) {
	a := llm.ToolCallKey("apply_patch", []byte(`{"input":"*** Add File: out.bin","path":"x"}`))
	for _, same := range []string{
		`{"path":"x","input":"*** Add File: out.bin"}`,
		`{ "input" : "*** Add File: out.bin",  "path": "x" }`,
		"{\n  \"path\": \"x\",\n  \"input\": \"*** Add File: out.bin\"\n}",
	} {
		if got := llm.ToolCallKey("apply_patch", []byte(same)); got != a {
			t.Errorf("%s is the same call", same)
		}
	}
	for name, other := range map[string]string{
		"apply_patch": `{"input":"*** Add File: out.bin","path":"y"}`,
		"bash":        `{"input":"*** Add File: out.bin","path":"x"}`,
	} {
		if llm.ToolCallKey(name, []byte(other)) == a {
			t.Errorf("%s %s is another call", name, other)
		}
	}
	// Not JSON: compared as bytes, never collapsed with something else.
	if llm.ToolCallKey("bash", []byte("not json")) == llm.ToolCallKey("bash", []byte("not  json")) {
		t.Error("non-JSON inputs that differ are different calls")
	}
}

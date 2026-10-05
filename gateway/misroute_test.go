package gateway

import (
	"encoding/json"
	"testing"
)

// TestMisroutedToolCallShape pins the detection contract: arguments addressing
// a different tool ({"command":"bash","parameters":{...}}) are distinguishable
// from a NORMAL bash call ({"command":"go test ./..."}) purely by whether
// "command" names a registered tool and "parameters" is present.
func TestMisroutedToolCallShape(t *testing.T) {
	var mis struct {
		Command    string          `json:"command"`
		Parameters json.RawMessage `json:"parameters"`
	}

	live := []byte(`{"command": "bash", "parameters": {"command": "go test ./..."}}`)
	if json.Unmarshal(live, &mis) != nil || mis.Command != "bash" || len(mis.Parameters) == 0 {
		t.Fatal("live misrouted shape must decode with command+parameters")
	}

	normal := []byte(`{"command": "go test ./..."}`)
	mis.Command, mis.Parameters = "", nil
	if json.Unmarshal(normal, &mis) != nil {
		t.Fatal("normal bash input must decode")
	}
	if len(mis.Parameters) != 0 {
		t.Fatal("normal bash call must NOT look misrouted (no parameters key)")
	}
}

package gateway

import (
	"encoding/json"
	"testing"

	"memdoor/pkg/llm"
)

func TestCoerceInputToSchemaTypesTheXMLStrings(t *testing.T) {
	schema := llm.ToolInputSchemaParam{Type: "object", Properties: map[string]any{
		"source":  map[string]any{"type": "string"},
		"start":   map[string]any{"type": "number"},
		"end":     map[string]any{"type": "number"},
		"count":   map[string]any{"type": "integer"},
		"reframe": map[string]any{"type": "boolean"},
	}}
	in := json.RawMessage(`{"source":"7","start":"16","end":"28.5","count":"3","reframe":"true","extra":"x"}`)
	var got struct {
		Source  string  `json:"source"`
		Start   float64 `json:"start"`
		End     float64 `json:"end"`
		Count   int     `json:"count"`
		Reframe bool    `json:"reframe"`
		Extra   string  `json:"extra"`
	}
	if err := json.Unmarshal(coerceInputToSchema(in, schema), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "7" || got.Start != 16 || got.End != 28.5 || got.Count != 3 || !got.Reframe || got.Extra != "x" {
		t.Errorf("coerced badly: %+v", got)
	}
	// A string that is not a number for a number field is left alone (the tool reports it).
	if out := coerceInputToSchema(json.RawMessage(`{"start":"sixteen"}`), schema); string(out) != `{"start":"sixteen"}` {
		t.Errorf("non-numeric text must be left for the tool: %s", out)
	}
	// Already-typed input passes through untouched.
	if out := coerceInputToSchema(json.RawMessage(`{"start":16}`), schema); string(out) != `{"start":16}` {
		t.Errorf("typed input must pass through: %s", out)
	}
}

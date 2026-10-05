package gateway

import (
	"encoding/json"
	"strconv"

	"memdoor/pkg/llm"
)

// coerceInputToSchema types a tool call's scalar arguments by the tool's own
// schema. Models quote numbers now and then — a number-typed field arrives
// as the string "16" and the tool refused it: "cannot unmarshal string into
// Go struct field … of type float64" (live 2026-09-03). The schema
// says what each field is; a string that parses as that type becomes it,
// anything else is left for the tool to judge.
func coerceInputToSchema(input json.RawMessage, schema llm.ToolInputSchemaParam) json.RawMessage {
	if schema.Properties == nil || len(input) == 0 {
		return input
	}
	props, err := json.Marshal(schema.Properties)
	if err != nil {
		return input
	}
	var types map[string]struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(props, &types) != nil {
		return input
	}
	var args map[string]json.RawMessage
	if json.Unmarshal(input, &args) != nil {
		return input
	}
	changed := false
	for k, raw := range args {
		var s string
		if json.Unmarshal(raw, &s) != nil { // not a string: already typed
			continue
		}
		switch types[k].Type {
		case "number", "integer":
			if _, err := strconv.ParseFloat(s, 64); err == nil {
				args[k] = json.RawMessage(s)
				changed = true
			}
		case "boolean":
			if b, err := strconv.ParseBool(s); err == nil {
				args[k] = json.RawMessage(strconv.FormatBool(b))
				changed = true
			}
		}
	}
	if !changed {
		return input
	}
	out, err := json.Marshal(args)
	if err != nil {
		return input
	}
	return out
}

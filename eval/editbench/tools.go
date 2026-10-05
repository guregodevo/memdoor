package main

import (
	"encoding/json"

	"memdoor/tools"
)

// OpenAI-style tool spec: {type:"function", function:{name, description, parameters}}.
type toolSpec struct {
	Type     string     `json:"type"`
	Function toolFnSpec `json:"function"`
}

type toolFnSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// toolsFor is read_file and apply_patch as the coder is offered them under
// edit_format=format: the descriptions and schemas come from the tools
// package itself (tools.BuildToolUnionParams, what the gateway submits).
func toolsFor(format string) []toolSpec {
	tools.SetEditFormat(func() string { return format })
	var out []toolSpec
	for _, u := range tools.BuildToolUnionParams([]tools.ToolDefinition{tools.ReadFileDefinition, tools.ApplyPatchDefinition}) {
		desc := ""
		if u.OfTool.Description != nil {
			desc = *u.OfTool.Description
		}
		out = append(out, toolSpec{Type: "function", Function: toolFnSpec{Name: u.OfTool.Name, Description: desc, Parameters: u.OfTool.InputSchema}})
	}
	return out
}

// runTool runs one call through the tools package in the run dir: read_file
// reads relative to the process's working directory (runOne moves into the
// run dir), apply_patch is told the dir as its cwd, the way the gateway's
// coder-workdir confinement tells it.
func runTool(dir, name, args string) (string, error) {
	switch name {
	case tools.ReadFileDefinition.Name:
		return tools.ReadFile(json.RawMessage(args))
	case tools.ApplyPatchDefinition.Name:
		var in map[string]any
		if err := json.Unmarshal([]byte(args), &in); err != nil {
			return "", err
		}
		in["cwd"] = dir
		b, _ := json.Marshal(in)
		return tools.ApplyPatch(b)
	}
	return "", errUnknownTool
}

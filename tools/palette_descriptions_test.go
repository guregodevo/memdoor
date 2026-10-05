package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

// A tool the model uses badly is often a tool the model was described badly.
//
// Measured 2026-08-30: asked to fix a file that did not exist,
// the model emitted ONE reply carrying 583 read_file calls, and a later reply
// with 381 bash calls. At the time read_file's entire guidance was "Read the
// contents of a given relative file path" with the parameter documented as
// "The relative path of a file in the working directory", and bash's was
// "Execute a bash command and return its output" / "The bash command to
// execute". Neither said the thing that would have stopped the loop: do not
// guess a filename, and run one command then READ it.
//
// This holds every tool in the coder's palette to a floor, so the next thin
// description fails here rather than in a paid turn.
func TestCoderPaletteToolsAreProperlyDescribed(t *testing.T) {
	// The palette from gateway/agent_seeding.go coderToolPalette.
	for _, d := range []ToolDefinition{
		BashDefinition, ReadFileDefinition, ApplyPatchDefinition, GrepDefinition,
		GlobDefinition, TodoWriteDefinition, TodoReadDefinition, LocateDefinition,
		SkillDefinition, AskUserQuestionDefinition,
	} {
		if len(d.Description) < 120 {
			t.Errorf("%s: tool description is %d chars — too thin to steer a small model. "+
				"Say what it does AND the mistake it must avoid.", d.Name, len(d.Description))
		}

		b, err := json.Marshal(d.InputSchema)
		if err != nil {
			t.Fatalf("%s: schema will not marshal: %v", d.Name, err)
		}
		var sch struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(b, &sch); err != nil {
			t.Fatalf("%s: schema shape unreadable: %v", d.Name, err)
		}
		for name, p := range sch.Properties {
			// Every parameter the MODEL can see must be described. A bare
			// {"type":"string"} is a parameter the model has to guess the
			// meaning of — and guessing is the failure this file exists for.
			if strings.TrimSpace(p.Description) == "" {
				t.Errorf("%s.%s: no description at all. Either document it or keep it out of the "+
					"model-facing schema with `json:\"-\"` (see BashInput.Cwd).", d.Name, name)
				continue
			}
			if len(p.Description) < 30 {
				t.Errorf("%s.%s: description is %d chars (%q) — say what a VALID value looks like",
					d.Name, name, len(p.Description), p.Description)
			}
		}
	}
}

// Cwd is harness-injected confinement, not a model choice. Until 2026-08-30 it
// carried `json:"cwd,omitempty"`, which put a bare {"type":"string"} with no
// description into every bash schema the model saw — inviting the model to set
// the one field that decides where its commands run.
func TestHarnessInjectedCwdIsNotInTheModelFacingSchema(t *testing.T) {
	for _, d := range []ToolDefinition{BashDefinition, ApplyPatchDefinition, LocateDefinition} {
		b, _ := json.Marshal(d.InputSchema)
		if strings.Contains(string(b), `"cwd"`) {
			t.Errorf("%s advertises cwd to the model: %s", d.Name, b)
		}
	}
}

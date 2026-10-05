package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// recall — an earlier tool output that compaction stubbed, word for word.
//
// A stub used to say "re-run the tool if you need it again", and re-running
// is not always safe or the same: a command with side effects runs them
// again, and a file read returns the file as it is now. The transcript keeps
// every original (compaction reshapes only the conversation as sent), so
// the output is recalled from there by the id of the call that made it.

type RecallInput struct {
	ID string `json:"id" jsonschema_description:"The id of the call whose output was stubbed, as the stub names it."`
	// Transcript is set by the harness, never by the model.
	Transcript string `json:"transcript,omitempty" jsonschema:"-"`
}

var recallSchema = GenerateSchema[RecallInput]()

var RecallDefinition = ToolDefinition{
	Name: "recall",
	Description: "Return, word for word, an earlier tool output that was stubbed to save room (the stub names the call's id). " +
		"Use it instead of running the call again: a command would run its side effects again, and a file read returns the file as it is now.",
	InputSchema: recallSchema,
	Function:    Recall,
}

// recaller finds a call's output in a transcript; wired at startup.
var recaller func(transcript, id string) (string, error)

// SetRecaller wires where recall reads earlier outputs from.
func SetRecaller(fn func(transcript, id string) (string, error)) { recaller = fn }

func Recall(input json.RawMessage) (string, error) {
	var in RecallInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	in.ID = strings.TrimSpace(in.ID)
	switch {
	case in.ID == "":
		return "", fmt.Errorf("recall: give the id the stub names")
	case recaller == nil || in.Transcript == "":
		return "", fmt.Errorf("recall: no conversation to recall from")
	}
	return recaller(in.Transcript, in.ID)
}

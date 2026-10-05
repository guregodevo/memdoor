package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ToolPrefix starts every MCP tool's name: mcp__<server>__<tool>, the
// convention models already know from Claude Code.
const ToolPrefix = "mcp__"

// maxToolName is the longest tool name the vendors accept.
const maxToolName = 64

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// ToolName is the name an MCP tool is offered to the model under. A name
// too long for the vendors keeps its start and ends in a hash of the whole.
func ToolName(server, tool string) string {
	name := ToolPrefix + server + "__" + unsafeName.ReplaceAllString(tool, "_")
	if len(name) <= maxToolName {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	return name[:maxToolName-9] + "_" + hex.EncodeToString(sum[:4])
}

// InputSchema is a tool's schema as the model's tool parameters take it:
// its properties and its required fields. An absent or odd schema is an
// object with no fields.
func InputSchema(t Tool) (properties map[string]interface{}, required []string) {
	var s struct {
		Properties map[string]interface{} `json:"properties"`
		Required   []string               `json:"required"`
	}
	if len(t.InputSchema) > 0 {
		_ = json.Unmarshal(t.InputSchema, &s)
	}
	if s.Properties == nil {
		s.Properties = map[string]interface{}{}
	}
	return s.Properties, s.Required
}

// MaxResultChars bounds a tool result handed to the model.
const MaxResultChars = 50000

// ResultText is a tool result as the model reads it. A result the server
// marked as an error comes back as an error with its text.
func ResultText(res *ToolResult) (string, error) {
	if res == nil {
		return "", errors.New("the server returned no result")
	}
	var parts []string
	for _, c := range res.Content {
		switch c.Type {
		case "text":
			parts = append(parts, c.Text)
		case "image", "audio":
			parts = append(parts, fmt.Sprintf("[%s: %s, %d KB, not shown]", c.Type, c.MimeType, len(c.Data)*3/4/1024))
		case "resource":
			if c.Resource != nil {
				parts = append(parts, "[Resource: "+c.Resource.URI+"]\n"+c.Resource.Text)
			}
		case "resource_link":
			parts = append(parts, "[Resource link]")
		}
	}
	if len(parts) == 0 && len(res.StructuredContent) > 0 {
		parts = append(parts, string(res.StructuredContent))
	}
	text := strings.Join(parts, "\n")
	if n := len(text); n > MaxResultChars {
		text = text[:MaxResultChars] + fmt.Sprintf("\n… (truncated: the result was %d characters)", n)
	}
	if res.IsError {
		if text == "" {
			text = "the tool reported an error"
		}
		return "", errors.New(text)
	}
	if text == "" {
		text = "(no output)"
	}
	return text, nil
}

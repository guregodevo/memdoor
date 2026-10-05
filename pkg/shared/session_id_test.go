package shared

import (
	"testing"
)

func TestParseSessionID_WorkspaceChannel(t *testing.T) {
	sessionID := "workspace:ws123:channel:ch456"
	parsed := ParseSessionID(sessionID)

	if parsed.Format != SessionFormatWorkspaceChannel {
		t.Errorf("Expected format %s, got %s", SessionFormatWorkspaceChannel, parsed.Format)
	}
	if parsed.WorkspaceID != "ws123" {
		t.Errorf("Expected workspace ID 'ws123', got '%s'", parsed.WorkspaceID)
	}
	if parsed.ChannelID != "ch456" {
		t.Errorf("Expected channel ID 'ch456', got '%s'", parsed.ChannelID)
	}
	if parsed.GetChannelID() != "ch456" {
		t.Errorf("GetChannelID() should return 'ch456', got '%s'", parsed.GetChannelID())
	}
	if parsed.IsSubagent() {
		t.Error("IsSubagent() should return false for workspace:channel format")
	}
}

func TestParseSessionID_WorkspaceChannelUUID(t *testing.T) {
	sessionID := "workspace:default:channel:2af5743f-c6e7-4a19-8e42-83f6dc871818"
	parsed := ParseSessionID(sessionID)

	if parsed.Format != SessionFormatWorkspaceChannel {
		t.Errorf("Expected format %s, got %s", SessionFormatWorkspaceChannel, parsed.Format)
	}
	if parsed.WorkspaceID != "default" {
		t.Errorf("Expected workspace ID 'default', got '%s'", parsed.WorkspaceID)
	}
	if parsed.ChannelID != "2af5743f-c6e7-4a19-8e42-83f6dc871818" {
		t.Errorf("Expected channel ID with UUID, got '%s'", parsed.ChannelID)
	}
}

func TestParseSessionID_Subagent(t *testing.T) {
	sessionID := "agent:main:subagent:run-023b1f66"
	parsed := ParseSessionID(sessionID)

	if parsed.Format != SessionFormatSubagent {
		t.Errorf("Expected format %s, got %s", SessionFormatSubagent, parsed.Format)
	}
	if parsed.AgentID != "main" {
		t.Errorf("Expected agent ID 'main', got '%s'", parsed.AgentID)
	}
	if parsed.SubagentRun != "run-023b1f66" {
		t.Errorf("Expected subagent run 'run-023b1f66', got '%s'", parsed.SubagentRun)
	}
	if parsed.GetChannelID() != "" {
		t.Errorf("GetChannelID() should return empty string for subagent format, got '%s'", parsed.GetChannelID())
	}
	if !parsed.IsSubagent() {
		t.Error("IsSubagent() should return true for subagent format")
	}
}

func TestParseSessionID_Unknown(t *testing.T) {
	sessionID := "some:random:format"
	parsed := ParseSessionID(sessionID)

	if parsed.Format != SessionFormatUnknown {
		t.Errorf("Expected format %s, got %s", SessionFormatUnknown, parsed.Format)
	}
	if parsed.GetChannelID() != "" {
		t.Errorf("GetChannelID() should return empty string for unknown format, got '%s'", parsed.GetChannelID())
	}
}

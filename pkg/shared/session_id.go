package shared

import (
	"fmt"
	"strings"
)

// SessionID represents a parsed session identifier
// Supports multiple session ID formats used across the system
type SessionID struct {
	Format      SessionIDFormat
	WorkspaceID string // For workspace:xxx:channel:yyy format
	ChannelID   string // For workspace:xxx:channel:yyy format
	AgentID     string // For agent:xxx:subagent:run-yyy format
	SubagentRun string // For agent:xxx:subagent:run-yyy format
	Raw         string // Original unparsed session ID
}

// SessionIDFormat identifies the type of session ID
type SessionIDFormat string

const (
	SessionFormatWorkspaceChannel SessionIDFormat = "workspace_channel" // workspace:{id}:channel:{id}
	SessionFormatSubagent         SessionIDFormat = "subagent"          // agent:main:subagent:run-{id}
	SessionFormatUnknown          SessionIDFormat = "unknown"
)

// NewChannelSessionID builds the conversation-session key for an agent run in
// a channel: `workspace` is the primary scope, `channel` is the channel id.
//
// This is the SINGLE place the workspace:{scope}:channel:{id} format is
// produced — both pkg/message (the run) and the /ws endpoint (the subscribing
// client) build the key here, so the key a run emits on and the key a client
// subscribes to can never drift. The reverse is ParseSessionID.
func NewChannelSessionID(workspace, channel string) string {
	return fmt.Sprintf("workspace:%s:channel:%s", workspace, channel)
}

// ParseSessionID parses a session ID string into its components
func ParseSessionID(sessionID string) SessionID {
	sid := SessionID{Raw: sessionID}

	// Check for subagent format: agent:main:subagent:run-xxx
	if strings.Contains(sessionID, ":subagent:") {
		sid.Format = SessionFormatSubagent
		parts := strings.Split(sessionID, ":")
		if len(parts) >= 2 {
			sid.AgentID = parts[1] // "main"
		}
		if len(parts) >= 4 {
			sid.SubagentRun = parts[3] // "run-xxx"
		}
		return sid
	}

	// Check for workspace:channel format: workspace:{id}:channel:{id}
	if strings.Contains(sessionID, ":channel:") {
		sid.Format = SessionFormatWorkspaceChannel
		parts := strings.Split(sessionID, ":channel:")
		if len(parts) == 2 {
			sid.ChannelID = parts[1]
			// Extract workspace ID from first part (workspace:{id})
			workspaceParts := strings.Split(parts[0], ":")
			if len(workspaceParts) >= 2 {
				sid.WorkspaceID = workspaceParts[1]
			}
		}
		return sid
	}

	// Unknown format
	sid.Format = SessionFormatUnknown
	return sid
}

// GetChannelID returns the channel ID if available, empty string otherwise
func (s SessionID) GetChannelID() string {
	return s.ChannelID
}

// GetAgentID returns the agent ID if available, empty string otherwise
func (s SessionID) GetAgentID() string {
	return s.AgentID
}

// IsSubagent returns true if this is a subagent session
func (s SessionID) IsSubagent() bool {
	return s.Format == SessionFormatSubagent
}

package protocol

// Message represents a WebSocket message
// Pattern: OpenClaw's protocol definitions (gateway/protocol/index.ts)
type Message struct {
	Type      string                 `json:"type"`
	SessionID string                 `json:"session_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// MessageType constants
const (
	MessageTypeConnected     = "connected"
	MessageTypePing          = "ping"
	MessageTypePong          = "pong"
	MessageTypeChat          = "chat"
	MessageTypeCancel        = "cancel" // client → server: interrupt the session's running turn
	MessageTypeAnswer        = "answer" // client → server: answer to an interactive ask_user_question
	MessageTypeChatResponse  = "chat_response"
	MessageTypeAgentResponse = "agent_response"
	MessageTypeThinking      = "thinking"
	MessageTypeToolExecution = "tool_execution"
	MessageTypeAgentEvent    = "agent_event"
	MessageTypeError         = "error"
)

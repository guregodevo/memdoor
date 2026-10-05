package adapters

import (
	"context"
)

// ChannelAdapter defines the interface that all channel implementations must satisfy
// Pattern: OpenClaw multi-channel routing (WhatsApp, Telegram, Discord, etc.)
type ChannelAdapter interface {
	// GetID returns a unique identifier for this channel (e.g., "whatsapp", "telegram", "tui")
	GetID() string

	// GetType returns the channel type (e.g., "messaging", "terminal", "websocket")
	GetType() string

	// Start initializes and starts the channel adapter
	Start(ctx context.Context) error

	// Stop gracefully shuts down the channel adapter
	Stop() error

	// IsRunning returns true if the channel is currently active
	IsRunning() bool

	// SendMessage sends a message to the channel
	// channelUserID: the recipient ID on this channel (e.g., WhatsApp phone number)
	// message: the text message to send
	SendMessage(channelUserID string, message string) error

	// SendEvent sends a structured event to the channel (for real-time updates)
	// channelUserID: the recipient ID on this channel
	// event: structured event data (lifecycle, tool, assistant events)
	SendEvent(channelUserID string, event map[string]interface{}) error

	// RegisterHandler registers a callback for incoming messages
	// handler: function to call when a message is received
	RegisterHandler(handler MessageHandler) error
}

// MessageHandler is called when a message is received from a channel
type MessageHandler func(msg *IncomingMessage) error

// IncomingMessage represents a message received from a channel
type IncomingMessage struct {
	// ChannelID is the channel this message came from (e.g., "whatsapp")
	ChannelID string

	// ChannelUserID is the sender's ID on this channel (e.g., phone number for WhatsApp)
	ChannelUserID string

	// SessionKey is the mapped session key for this user (e.g., "main:agent1:session1")
	SessionKey string

	// Text is the message content
	Text string

	// Media contains any attached media (images, files, etc.)
	Media []*MediaAttachment

	// Timestamp is when the message was received
	Timestamp int64

	// Context carries additional metadata
	Context map[string]interface{}
}

// MediaAttachment represents a file or image attachment
type MediaAttachment struct {
	Type     string // "image", "audio", "video", "document"
	MimeType string
	Data     []byte
	Filename string
	URL      string // Optional URL if hosted
}

// ChannelConfig holds configuration for a channel adapter
type ChannelConfig struct {
	ID      string                 // Channel ID (e.g., "whatsapp")
	Type    string                 // Channel type (e.g., "messaging")
	Enabled bool                   // Whether this channel is enabled
	Options map[string]interface{} // Channel-specific options
}

// ChannelCapabilities describes what a channel can do
type ChannelCapabilities struct {
	SupportsMedia      bool // Can send/receive images, files
	SupportsFormatting bool // Can send formatted text (markdown, bold, etc.)
	SupportsReactions  bool // Can react to messages with emojis
	SupportsThreads    bool // Can organize messages in threads
	MaxMessageLength   int  // Maximum message length (0 = unlimited)
}

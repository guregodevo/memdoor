package memory

import (
	"fmt"

	"memdoor/gateway/config"
)

// FlushSettings holds resolved memory flush configuration
// Pattern: OpenClaw src/auto-reply/reply/memory-flush.ts
type FlushSettings struct {
	Enabled      bool
	Prompt       string
	SystemPrompt string
}

// Default values from OpenClaw
const SilentReplyToken = "NO_REPLY"

var (
	// The reply IS the memory: the runtime appends it to the conversation's
	// notes (pkg/notes, the same notes the notes tool reads). It used to ask the
	// model to write memory/<date>.md — from a call that runs no tools, so the
	// file was never written and the reply was thrown away.
	DefaultMemoryFlushPrompt = fmt.Sprintf(
		"The conversation is about to be compacted, and what is not written down is lost. "+
			"Reply with ONLY what is worth keeping for later work in this project, as short "+
			"bullet points: how to build and test it, conventions, decisions and why, what was "+
			"tried and failed, and anything the person corrected (as \"RULE: …\"). No preamble. "+
			"If nothing is worth keeping, reply with %s.",
		SilentReplyToken,
	)

	DefaultMemoryFlushSystemPrompt = fmt.Sprintf(
		"Pre-compaction memory flush turn. "+
			"The session is near auto-compaction; capture durable memories to disk. "+
			"You may reply, but usually %s is correct.",
		SilentReplyToken,
	)
)

// ResolveFlushSettings resolves memory flush settings from config
// Pattern: OpenClaw src/auto-reply/reply/memory-flush.ts:38-59
func ResolveFlushSettings(cfg *config.CompactionConfig) *FlushSettings {
	if cfg == nil || cfg.MemoryFlush == nil {
		// Default: enabled
		return &FlushSettings{
			Enabled:      true,
			Prompt:       DefaultMemoryFlushPrompt,
			SystemPrompt: DefaultMemoryFlushSystemPrompt,
		}
	}

	flushCfg := cfg.MemoryFlush

	// Check if explicitly disabled
	if !flushCfg.Enabled {
		return nil
	}

	settings := &FlushSettings{
		Enabled:      true,
		Prompt:       DefaultMemoryFlushPrompt,
		SystemPrompt: DefaultMemoryFlushSystemPrompt,
	}

	// Override with config values
	if flushCfg.Prompt != "" {
		settings.Prompt = ensureNoReplyHint(flushCfg.Prompt)
	}
	if flushCfg.SystemPrompt != "" {
		settings.SystemPrompt = ensureNoReplyHint(flushCfg.SystemPrompt)
	}

	return settings
}

// ensureNoReplyHint ensures the prompt includes NO_REPLY hint
func ensureNoReplyHint(text string) string {
	// Check if already contains NO_REPLY
	if len(text) > 0 && (text[len(text)-1:] == SilentReplyToken ||
		findSubstring(text, SilentReplyToken)) {
		return text
	}

	return fmt.Sprintf("%s\n\nIf no user-visible reply is needed, start with %s.", text, SilentReplyToken)
}

// findSubstring checks if substring exists in string
func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Session metadata keys for memory flush tracking
const (
	MetadataKeyCompactionCount = "compactionCount"
	MetadataKeyTotalTokens     = "totalTokens"
)

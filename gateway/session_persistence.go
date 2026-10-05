package gateway

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"memdoor/gateway/config"
	"memdoor/gateway/logs"
	"memdoor/gateway/routing"
	"memdoor/gateway/workspace"

	"memdoor/pkg/llm"
	"memdoor/pkg/shared"
)

// SessionPersistence handles saving and loading conversation history
// Pattern: File-based persistence with per-agent storage (OpenClaw multi-agent)
type SessionPersistence struct {
	cfg *config.Config // Configuration for resolving per-agent paths
	mu  sync.RWMutex
	log *logs.EventLogger
}

// MessageRecord wraps a MessageParam with a timestamp for storage
// Pattern: JSONL format - one JSON object per line (OpenClaw style)
//
// The file is append-only history. When a turn reshapes the conversation to
// fit the window (old tool output stubbed, old messages replaced by a
// summary), the conversation AS SENT is appended as a block of Carried
// records, the first of them a Boundary; the next turn loads from the last
// boundary, so it sends what the last one sent plus what came after — the
// same prefix, which the provider has cached. Recomputed every turn instead,
// the summary changed each time and no prefix survived (2026-09-29). The
// originals stay above the boundary for export and rewind.
type MessageRecord struct {
	Message   llm.MessageParam `json:"message"`
	Timestamp int64            `json:"timestamp"`
	// Boundary: the conversation as sent starts again at this record.
	Boundary bool `json:"boundary,omitempty"`
	// Carried: a copy (or a summary) inside a reshaped conversation, not a
	// message of the history.
	Carried bool `json:"carried,omitempty"`
}

// lastBoundary is the index of the last Boundary record, -1 when none.
func lastBoundary(records []MessageRecord) int {
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Boundary {
			return i
		}
	}
	return -1
}

// asSent is the records a turn loads: from the last boundary on, else all.
func asSent(records []MessageRecord) []MessageRecord {
	if b := lastBoundary(records); b >= 0 {
		return records[b:]
	}
	return records
}

// startsATurn reports whether rec is a request the person typed: an original
// user message carrying text, not a tool result and not a carried copy.
func startsATurn(rec MessageRecord) bool {
	if rec.Carried || rec.Message.Role != llm.MessageParamRoleUser || len(rec.Message.Content) == 0 {
		return false
	}
	return rec.Message.Content[0].OfText != nil
}

// NewSessionPersistence creates a new session persistence manager
// Pattern: Configuration-based per-agent storage (OpenClaw pattern)
func NewSessionPersistence(cfg *config.Config, verbose bool) (*SessionPersistence, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	log := logs.New("Agent")

	// Set default truncation limits if not configured
	if cfg.Session == nil {
		cfg.Session = &config.SessionConfig{}
	}
	if cfg.Session.MaxToolResultBytes == 0 {
		cfg.Session.MaxToolResultBytes = config.DefaultMaxToolResultBytes
	}
	if cfg.Session.TruncationMessageTail == "" {
		cfg.Session.TruncationMessageTail = config.DefaultTruncationMessageTail
	}

	log.Debug("Session persistence initialized with config",
		slog.Int("max_tool_result_bytes", cfg.Session.MaxToolResultBytes))

	return &SessionPersistence{
		cfg: cfg,
		log: log,
	}, nil
}

// resolveSessionStorePath resolves the session store path for a session ID
// Pattern: Workspace → Room → Sessions hierarchy
//
// Resolution logic:
//  1. Check if it's a channel session (format: "workspace:{workspaceID}:channel:{channelID}")
//     Note: Session key uses "channel" for backward compatibility, but represents a channel
//  2. If channel session, store in: ~/.greg/workspaces/{workspaceID}/channels/{channelID}/sessions.json
//  3. Otherwise, parse session ID to extract agent ID and use agent-specific path
func (sp *SessionPersistence) resolveSessionStorePath(sessionID string) (string, error) {
	var storePath string

	// Check if this is a channel session (shared multi-agent conversation in a channel like #general)
	// Format: "workspace:{workspaceID}:channel:{channelID}" (uses "channel" for backward compatibility)
	if strings.HasPrefix(sessionID, "workspace:") {
		parts := strings.Split(sessionID, ":")
		// Per-agent channel transcript (session isolation):
		// workspace:{ws}:channel:{ch}:agent:{agent} → its OWN store, so each agent's
		// reasoning transcript is isolated from the shared channel log.
		if len(parts) == 6 && parts[2] == "channel" && parts[4] == "agent" {
			storePath = shared.MemdoorHome("workspaces", parts[1], "channels", parts[3], "agents", parts[5], "sessions.json")
			return sp.ensureDirectoryExists(storePath)
		}
		// Expect: ["workspace", "{workspaceID}", "channel", "{channelID}"]
		if len(parts) == 4 && parts[2] == "channel" {
			workspaceID := parts[1]
			channelID := parts[3]

			// Workspace → Channel hierarchy in filesystem
			storePath = shared.MemdoorHome("workspaces", workspaceID, "channels", channelID, "sessions.json")

			sp.log.Debug("Resolved workspace/channel session path",
				slog.String("session_id", sessionID),
				slog.String("workspace_id", workspaceID),
				slog.String("channel_id", channelID),
				slog.String("path", storePath))

			return sp.ensureDirectoryExists(storePath)
		}
	}

	// Subagent sessions are ephemeral, single-task, and MUST be isolated — each
	// spawn is its own run. Key the store by the FULL unique session id
	// (…:subagent:run-XXX), NOT by the agent id. Otherwise every coder subagent
	// resolves to the SAME agents/coder/sessions.json and (since readAllRecords
	// doesn't filter by session) loads the entire history of every prior coder
	// subagent — which makes the coder copy its own past ```go narration instead of
	// writing. This is the pollution that made a correctly-built coder "not act".
	// A workflow task is the same kind of run (Greg's no-babysitting
	// workflows, 2026-10-03): "workflow:<name>:<partition>:<task>" fell
	// through to the agent's file, so every task of every run loaded all the
	// ones before it — 105k input tokens for a one-line task, growing each
	// run. Each task's turn gets its own file.
	if strings.HasPrefix(sessionID, "workflow:") {
		safe := strings.NewReplacer(":", "_", "/", "_", "\\", "_").Replace(sessionID)
		storePath = shared.MemdoorHome("workflow-sessions", safe+".json")
		return sp.ensureDirectoryExists(storePath)
	}
	if strings.Contains(sessionID, ":subagent:") {
		safe := strings.NewReplacer(":", "_", "/", "_", "\\", "_").Replace(sessionID)
		storePath = shared.MemdoorHome("subagents", safe+".json")
		return sp.ensureDirectoryExists(storePath)
	}

	// Default: agent-specific session (old format or agent sessions)
	agentID := routing.ResolveSessionAgentID(sessionID)

	storeTemplate := sp.cfg.Session.Store
	if storeTemplate == "" {
		storeTemplate = shared.MemdoorHome("agents", "{agentId}", "sessions.json")
	}

	storePath = workspace.ExpandTemplate(storeTemplate, agentID)
	return sp.ensureDirectoryExists(storePath)
}

// ensureDirectoryExists ensures the parent directory of filePath exists
func (sp *SessionPersistence) ensureDirectoryExists(filePath string) (string, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	return filePath, nil
}

// SaveMessage appends a message to the session's JSONL file
// Pattern: Append-only log with per-agent storage (OpenClaw multi-agent)
func (sp *SessionPersistence) SaveMessage(sessionID string, msg llm.MessageParam) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	msg = sp.truncateLargeToolResults(msg)

	storePath, err := sp.resolveSessionStorePath(sessionID)
	if err != nil {
		return fmt.Errorf("failed to resolve session store path: %w", err)
	}

	timestamp := time.Now().Unix()
	bytesWritten, err := sp.writeRecord(storePath, msg, timestamp)
	if err != nil {
		return err
	}

	sp.log.Debug("Saved message",
		slog.String("session", sessionID),
		slog.String("role", string(msg.Role)),
		slog.Int("bytes", bytesWritten))

	return nil
}

// SaveMessages atomically saves multiple messages to prevent interleaving
//
// Thread-safe: Acquires exclusive lock for entire batch write
//
// CRITICAL: This method MUST be used instead of calling SaveMessage() in a loop
// when saving multiple messages from the same conversation. Calling SaveMessage()
// repeatedly releases the lock between iterations, allowing concurrent executions
// to interleave their messages, which creates orphaned tool_use blocks.
//
// Example of INCORRECT usage (causes race condition):
//
//	for _, msg := range messages {
//	    sp.SaveMessage(sessionID, msg)  // ❌ Lock released between iterations!
//	}
//
// Example of CORRECT usage (atomic batch):
//
//	sp.SaveMessages(sessionID, messages)  // ✅ All messages saved under single lock
//
// Race condition prevented: When multiple agent executions write to the same session
// concurrently, this ensures each execution's messages are written as an atomic block,
// maintaining conversation history integrity (no orphaned tool_use blocks).
func (sp *SessionPersistence) SaveMessages(sessionID string, messages []llm.MessageParam) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	storePath, err := sp.resolveSessionStorePath(sessionID)
	if err != nil {
		return fmt.Errorf("failed to resolve session store path: %w", err)
	}

	file, err := os.OpenFile(storePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	timestamp := time.Now().Unix()
	totalBytes := 0

	for _, msg := range messages {
		msg = sp.truncateLargeToolResults(msg)
		n, err := marshalAndWriteRecord(file, MessageRecord{Message: msg, Timestamp: timestamp})
		if err != nil {
			return err
		}
		totalBytes += n
	}

	sp.log.Debug("Saved message batch atomically",
		slog.String("session", sessionID),
		slog.Int("count", len(messages)),
		slog.Int("bytes", totalBytes))

	// Sessions are SHARED across every agent in a channel, so an active
	// channel's JSONL grows without bound and each load pays for it. Trim it
	// here — in the session layer, not the agent adapter — when it gets large.
	if err := sp.compactSessionFile(storePath, sessionID); err != nil {
		sp.log.Warn("session file compaction failed (file still usable)",
			slog.String("session", sessionID), slog.String("error", err.Error()))
	}

	return nil
}

// SaveConversation appends a reshaped conversation as sent (see
// MessageRecord): the next turn loads from it. Nothing is cut here — it is
// what the model was sent.
func (sp *SessionPersistence) SaveConversation(sessionID string, sent []llm.MessageParam) error {
	if len(sent) == 0 {
		return nil
	}
	sp.mu.Lock()
	defer sp.mu.Unlock()

	storePath, err := sp.resolveSessionStorePath(sessionID)
	if err != nil {
		return fmt.Errorf("failed to resolve session store path: %w", err)
	}
	file, err := os.OpenFile(storePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	timestamp := time.Now().Unix()
	for i, msg := range sent {
		if _, err := marshalAndWriteRecord(file, MessageRecord{Message: msg, Timestamp: timestamp, Boundary: i == 0, Carried: true}); err != nil {
			return err
		}
	}
	sp.log.Debug("Saved the conversation as sent", slog.String("session", sessionID), slog.Int("count", len(sent)))
	return nil
}

// Session-file bounds. Past sessionCompactTriggerBytes the file keeps its
// last sessionKeepMessages records of history.
const (
	sessionCompactTriggerBytes = 1 << 20 // 1 MiB
	sessionKeepMessages        = 200
)

// trimPoint is the first record an oversized file keeps, 0 to keep all. It
// never changes what a turn loads, and the file never starts mid-turn:
//   - with a boundary, only history above it is cut (the conversation as
//     sent starts at the boundary);
//   - without one, the cut moves forward to the next request, so the file
//     does not open on a tool result whose call was cut, which a provider
//     refuses. That was a plain "last 200 records" (2026-09-29).
func trimPoint(records []MessageRecord) int {
	cut := len(records) - sessionKeepMessages
	if cut <= 0 {
		return 0
	}
	if b := lastBoundary(records); b >= 0 && cut > b {
		return b
	}
	for ; cut < len(records); cut++ {
		if records[cut].Boundary || startsATurn(records[cut]) {
			return cut
		}
	}
	return 0
}

// compactSessionFile rewrites an oversized session JSONL from its trimPoint,
// atomically (temp file + rename). Best-effort: the caller logs and continues
// on error (the session stays usable, just large). MUST be called with sp.mu
// held — it reuses the lock-free readAllRecords.
func (sp *SessionPersistence) compactSessionFile(storePath, sessionID string) error {
	fi, err := os.Stat(storePath)
	if err != nil || fi.Size() < sessionCompactTriggerBytes {
		return nil // small (or vanished) — nothing to do
	}
	records, err := sp.readAllRecords(storePath, sessionID)
	if err != nil {
		return err
	}
	cut := trimPoint(records)
	if cut == 0 {
		return nil
	}
	keep := records[cut:]

	tmp := storePath + ".compact.tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	for _, rec := range keep {
		if _, err := marshalAndWriteRecord(f, rec); err != nil {
			f.Close()
			_ = os.Remove(tmp)
			return err
		}
	}
	if cErr := f.Close(); cErr != nil {
		_ = os.Remove(tmp)
		return cErr
	}
	sp.log.Info("compacted session file",
		slog.String("session", sessionID),
		slog.Int("kept", len(keep)), slog.Int("dropped", cut))
	return os.Rename(tmp, storePath)
}

// LoadMessages reads the history from a session's JSONL file
// Pattern: Read and parse JSONL with per-agent storage (OpenClaw multi-agent)
func (sp *SessionPersistence) LoadMessages(sessionID string) ([]llm.MessageParam, error) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	storePath, err := sp.resolveSessionStorePath(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve session store path: %w", err)
	}

	allRecords, err := sp.readAllRecords(storePath, sessionID)
	if err != nil {
		return nil, err
	}

	if len(allRecords) == 0 {
		sp.log.Debug("No saved messages for session",
			slog.String("session", sessionID))
		return []llm.MessageParam{}, nil
	}

	// The history: what was said, without the copies a reshaped
	// conversation carries.
	messages := make([]llm.MessageParam, 0, len(allRecords))
	for _, record := range allRecords {
		if !record.Carried {
			messages = append(messages, record.Message)
		}
	}

	sp.log.Debug("Loaded messages",
		slog.Int("count", len(messages)),
		slog.String("session", sessionID))

	return messages, nil
}

// LoadRecentMessages reads the last N messages of the conversation as last
// sent: from the last boundary when a turn reshaped it (see MessageRecord),
// else of the whole file.
func (sp *SessionPersistence) LoadRecentMessages(sessionID string, limit int) ([]llm.MessageParam, error) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	storePath, err := sp.resolveSessionStorePath(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve session store path: %w", err)
	}

	allRecords, err := sp.readAllRecords(storePath, sessionID)
	if err != nil {
		return nil, err
	}
	// The conversation as last sent, when a turn reshaped it.
	allRecords = asSent(allRecords)

	if len(allRecords) == 0 {
		sp.log.Debug("No saved messages for session",
			slog.String("session", sessionID))
		return []llm.MessageParam{}, nil
	}

	// Return only the last N messages
	startIdx := 0
	if len(allRecords) > limit {
		startIdx = len(allRecords) - limit
	}

	// Ensure we don't break tool_use/tool_result pairs
	startIdx = sp.adjustStartIndexForToolPairs(allRecords, startIdx)

	messages := make([]llm.MessageParam, 0, limit)
	for i := startIdx; i < len(allRecords); i++ {
		messages = append(messages, allRecords[i].Message)
	}

	sp.log.Debug("Loaded recent messages",
		slog.Int("count", len(messages)),
		slog.Int("total", len(allRecords)),
		slog.String("session", sessionID))

	return messages, nil
}

// DeleteSession removes a session's conversation history file
// Pattern: Per-agent deletion (OpenClaw multi-agent)
func (sp *SessionPersistence) DeleteSession(sessionID string) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	storePath, err := sp.resolveSessionStorePath(sessionID)
	if err != nil {
		return fmt.Errorf("failed to resolve session store path: %w", err)
	}

	if err := os.Remove(storePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	sp.log.Debug("Deleted session file",
		slog.String("session", sessionID))

	return nil
}

// ListSessions returns a list of all session IDs that have saved conversations
// Pattern: Per-agent session listing (OpenClaw multi-agent)
//
// This scans all configured agents and returns session files found for each.
// Note: Returns session file paths, not full session keys (e.g., "main", "cron:job1")
func (sp *SessionPersistence) ListSessions() ([]string, error) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	var sessions []string

	// Get list of all agents
	agentIDs := sp.cfg.ListAgentIDs()
	if len(agentIDs) == 0 {
		// No agents configured, return empty list
		return sessions, nil
	}

	// Scan each agent's session store
	for _, agentID := range agentIDs {
		// Build agent session key to resolve path
		sessionKey := routing.BuildAgentMainSessionKey(agentID)
		storePath, err := sp.resolveSessionStorePath(sessionKey)
		if err != nil {
			continue // Skip agents with resolution errors
		}

		// Check if session file exists
		if _, err := os.Stat(storePath); err == nil {
			// Session file exists, add to list
			sessions = append(sessions, sessionKey)
		}
	}

	return sessions, nil
}

// ListSessionsForAgent returns sessions for a specific agent
// Pattern: Agent-scoped session listing (OpenClaw pattern)
func (sp *SessionPersistence) ListSessionsForAgent(agentID string) ([]string, error) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	var sessions []string

	// Normalize agent ID
	normalizedAgentID := routing.NormalizeAgentID(agentID)

	// Build main session key
	mainSessionKey := routing.BuildAgentMainSessionKey(normalizedAgentID)
	storePath, err := sp.resolveSessionStorePath(mainSessionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve session store path: %w", err)
	}

	// Check if session file exists
	if _, err := os.Stat(storePath); err == nil {
		sessions = append(sessions, mainSessionKey)
	}

	return sessions, nil
}

// adjustStartIndexForToolPairs ensures tool_use/tool_result pairs are not broken by the sliding window
// It scans messages in the window for tool_result blocks and backtracks to include their tool_use messages
func (sp *SessionPersistence) adjustStartIndexForToolPairs(allRecords []MessageRecord, startIdx int) int {
	if startIdx == 0 || len(allRecords) == 0 {
		return startIdx // No adjustment needed if starting from beginning
	}

	// Step 1: Collect all tool_use_ids referenced in tool_result blocks within the window
	referencedToolUseIDs := make(map[string]bool)

	for i := startIdx; i < len(allRecords); i++ {
		msg := allRecords[i].Message

		// Check each content block for tool_result
		for _, content := range msg.Content {
			if content.OfToolResult != nil {
				// Found a tool_result block - record its tool_use_id
				referencedToolUseIDs[content.OfToolResult.ToolUseID] = true
			}
		}
	}

	if len(referencedToolUseIDs) == 0 {
		return startIdx // No tool_result blocks in window, no adjustment needed
	}

	// Step 2: Scan backwards from startIdx to find messages with the referenced tool_use blocks
	foundToolUseIDs := make(map[string]bool)
	adjustedStartIdx := startIdx

	for i := startIdx - 1; i >= 0; i-- {
		msg := allRecords[i].Message

		// Check each content block for tool_use
		for _, content := range msg.Content {
			if content.OfToolUse != nil {
				toolUseID := content.OfToolUse.ID

				// Is this tool_use referenced by a tool_result in our window?
				if referencedToolUseIDs[toolUseID] {
					foundToolUseIDs[toolUseID] = true
					adjustedStartIdx = i // Move start index back to include this message
				}
			}
		}

		// Stop if we've found all referenced tool_use blocks
		if len(foundToolUseIDs) == len(referencedToolUseIDs) {
			break
		}
	}

	if adjustedStartIdx < startIdx {
		sp.log.Debug("Adjusted sliding window to preserve tool_use/tool_result pairs",
			slog.Int("original_start", startIdx),
			slog.Int("adjusted_start", adjustedStartIdx),
			slog.Int("backtracked_messages", startIdx-adjustedStartIdx),
			slog.Int("tool_pairs", len(referencedToolUseIDs)))
	}

	return adjustedStartIdx
}

// getTruncationTail returns the configured truncation tail message with fallback to default
func (sp *SessionPersistence) getTruncationTail() string {
	if sp.cfg.Session.TruncationMessageTail != "" {
		return sp.cfg.Session.TruncationMessageTail
	}
	return config.DefaultTruncationMessageTail
}

// marshalAndWriteRecord writes one record as a single JSONL line to w and
// returns the marshaled byte count (excluding the newline). The shared core of
// every session-file writer — batch append (SaveMessages), single append
// (writeRecord), and the compaction rewrite — so they can't drift on format.
func marshalAndWriteRecord(w io.Writer, rec MessageRecord) (int, error) {
	data, err := json.Marshal(rec)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal message: %w", err)
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		return 0, fmt.Errorf("failed to write message: %w", err)
	}
	return len(data), nil
}

// writeRecord writes a single message record to the specified file
// Returns the number of bytes written
func (sp *SessionPersistence) writeRecord(filePath string, msg llm.MessageParam, timestamp int64) (int, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return 0, fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	return marshalAndWriteRecord(file, MessageRecord{Message: msg, Timestamp: timestamp})
}

// readAllRecords reads and parses all message records from a JSONL file
// Returns empty slice if file doesn't exist
func (sp *SessionPersistence) readAllRecords(filePath, sessionID string) ([]MessageRecord, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []MessageRecord{}, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	var allRecords []MessageRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}

		var record MessageRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			sp.log.Warn("Failed to parse line",
				slog.Int("line", lineNum),
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
			continue
		}

		allRecords = append(allRecords, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	return allRecords, nil
}

// Close gracefully shuts down the session persistence layer
// Pattern: Graceful shutdown - ensures all resources are properly released
func (sp *SessionPersistence) Close() error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.log.Info("Session persistence shutting down gracefully")

	// Note: We don't keep file handles open - each Save/Load operation
	// opens, writes/reads, and closes files atomically with defer.
	// Therefore, no file handles need to be closed here.
	//
	// However, this method provides a clean shutdown point for:
	// 1. Logging shutdown completion
	// 2. Future resource cleanup if needed (e.g., WAL flush, cache writes)
	// 3. Consistent shutdown pattern across all components

	sp.log.Info("Session persistence closed successfully")
	return nil
}

// truncateLargeToolResults truncates tool_result content blocks that exceed the configured size limit
// This prevents context window overflow from large tool outputs (Glob, Grep, Read, Browser, etc.)
//
// Pattern: Content window management - truncate large tool results to prevent session bloat
// Only tool_result content blocks are truncated; tool_use and text blocks are preserved as-is
func (sp *SessionPersistence) truncateLargeToolResults(msg llm.MessageParam) llm.MessageParam {
	maxBytes := sp.cfg.Session.MaxToolResultBytes
	if maxBytes == 0 {
		return msg // Truncation disabled
	}

	truncationTail := sp.getTruncationTail()

	// Only process user messages (which contain tool_result blocks)
	if msg.Role != llm.MessageParamRoleUser {
		return msg
	}

	// A tool result is cut when it ENTERS the conversation (capToolResult,
	// to the same limit), so what is saved is what was sent; this is the
	// net under it. It writes a copy: the blocks are shared with the
	// conversation in memory.
	out, cut := cutToolResults(msg, maxBytes, truncationTail)
	if cut > 0 {
		sp.log.Info("Truncated large tool results in message", slog.Int("truncated_count", cut))
	}
	return out
}

// cutToolResults returns msg with every tool-result text over maxBytes cut
// to it (tail included), and how many were cut. msg itself is not changed.
func cutToolResults(msg llm.MessageParam, maxBytes int, tail string) (llm.MessageParam, int) {
	cut := 0
	var blocks []llm.ContentBlockParamUnion
	for i, b := range msg.Content {
		if b.OfToolResult == nil {
			continue
		}
		var texts []llm.ToolResultBlockParamContentUnion
		for j, c := range b.OfToolResult.Content {
			if c.OfText == nil || len(c.OfText.Text) <= maxBytes {
				continue
			}
			if texts == nil {
				texts = append([]llm.ToolResultBlockParamContentUnion(nil), b.OfToolResult.Content...)
			}
			t := *c.OfText
			t.Text = cutText(t.Text, maxBytes, tail)
			texts[j].OfText = &t
			cut++
		}
		if texts == nil {
			continue
		}
		if blocks == nil {
			blocks = append([]llm.ContentBlockParamUnion(nil), msg.Content...)
		}
		r := *b.OfToolResult
		r.Content = texts
		blocks[i].OfToolResult = &r
	}
	if blocks != nil {
		msg.Content = blocks
	}
	return msg, cut
}

// cutText is text cut to maxBytes, tail included, never inside a character.
func cutText(text string, maxBytes int, tail string) string {
	if len(text) <= maxBytes {
		return text
	}
	n := maxBytes - len(tail)
	if n < 0 {
		n = 0
	}
	for n > 0 && !utf8.RuneStart(text[n]) {
		n--
	}
	return text[:n] + tail
}

// RewindSession truncates the persisted transcript to undo the last N turns —
// a turn starts at a USER message carrying real text (a prompt), not a
// tool_result echo, and runs to the end of its assistant/tool activity.
// Recovery lever for a derailed turn (live: four refused patches poisoning
// the context): drop the poison, keep everything before it. The in-memory
// session must be cleared by the caller so the next turn reloads this file.
// Returns the number of entries dropped.
func (sp *SessionPersistence) RewindSession(sessionID string, turns int) (int, error) {
	if turns < 1 {
		turns = 1
	}
	sp.mu.Lock()
	defer sp.mu.Unlock()

	storePath, err := sp.resolveSessionStorePath(sessionID)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve session store path: %w", err)
	}
	data, err := os.ReadFile(storePath)
	if err != nil {
		return 0, fmt.Errorf("no persisted session to rewind: %w", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	type entryPeek struct {
		Carried bool `json:"carried"`
		Message struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
			} `json:"content"`
		} `json:"message"`
	}
	// Walk backwards counting real user-prompt boundaries.
	cut := -1
	remaining := turns
	for i := len(lines) - 1; i >= 0; i-- {
		var e entryPeek
		if json.Unmarshal([]byte(lines[i]), &e) != nil {
			continue
		}
		// A carried copy of a request is not a turn: the request itself is
		// above the block, and cutting there drops the block with it, so
		// the turn before loads as it was.
		if !e.Carried && e.Message.Role == "user" && len(e.Message.Content) > 0 && e.Message.Content[0].Type == "text" {
			remaining--
			if remaining == 0 {
				cut = i
				break
			}
		}
	}
	if cut < 0 {
		return 0, fmt.Errorf("session has fewer than %d turns — use clear to wipe it entirely", turns)
	}
	dropped := len(lines) - cut

	kept := strings.Join(lines[:cut], "\n")
	if kept != "" {
		kept += "\n"
	}
	tmp := storePath + ".rewind.tmp"
	if err := os.WriteFile(tmp, []byte(kept), 0o644); err != nil {
		return 0, err
	}
	if err := os.Rename(tmp, storePath); err != nil {
		return 0, err
	}
	sp.log.Info(fmt.Sprintf("Session rewound %d turn(s): %d entries dropped", turns, dropped),
		slog.String("session", sessionID), slog.Int("dropped", dropped))
	return dropped, nil
}

package queue

// QueueMode determines how incoming messages are queued
// Pattern: OpenClaw src/auto-reply/reply/queue/types.ts
type QueueMode string

const (
	// QueueModeSteer injects message into current run (cancels pending tool calls after next tool boundary)
	QueueModeSteer QueueMode = "steer"

	// QueueModeFollowup enqueues for next agent turn after current run ends
	QueueModeFollowup QueueMode = "followup"

	// QueueModeCollect coalesces all queued messages into single followup turn (default)
	QueueModeCollect QueueMode = "collect"

	// QueueModeSteerBacklog steers now + preserves message for followup turn
	QueueModeSteerBacklog QueueMode = "steer-backlog"

	// QueueModeInterrupt aborts active run, then runs newest message (legacy)
	QueueModeInterrupt QueueMode = "interrupt"

	// QueueModeQueue legacy alias for steer
	QueueModeQueue QueueMode = "queue"
)

// QueueDropPolicy determines what to do when queue is full
// Pattern: OpenClaw src/utils/queue-helpers.ts
type QueueDropPolicy string

const (
	// DropPolicyOld drops oldest messages when queue is full
	DropPolicyOld QueueDropPolicy = "old"

	// DropPolicyNew drops newest messages when queue is full (rejects new messages)
	DropPolicyNew QueueDropPolicy = "new"

	// DropPolicySummarize keeps short bullet list of dropped messages, injects as synthetic followup
	DropPolicySummarize QueueDropPolicy = "summarize"
)

// QueueDedupeMode determines how to deduplicate queue items
// Pattern: OpenClaw src/auto-reply/reply/queue/types.ts
type QueueDedupeMode string

const (
	// DedupeModeMessageID deduplicates by message ID
	DedupeModeMessageID QueueDedupeMode = "message-id"

	// DedupeModePrompt deduplicates by prompt text
	DedupeModePrompt QueueDedupeMode = "prompt"

	// DedupeModeNone no deduplication
	DedupeModeNone QueueDedupeMode = "none"
)

// QueueSettings contains queue configuration for a session/channel
// Pattern: OpenClaw src/auto-reply/reply/queue/types.ts
type QueueSettings struct {
	Mode       QueueMode       // Queue mode (default: collect)
	DebounceMs int             // Debounce timeout in milliseconds (default: 1000)
	Cap        int             // Max queue size (default: 20)
	DropPolicy QueueDropPolicy // Drop policy when queue is full (default: summarize)
}

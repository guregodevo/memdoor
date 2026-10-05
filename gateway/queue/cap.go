package queue

// QueueSummaryState tracks dropped messages when using summarize policy
// Pattern: OpenClaw src/utils/queue-helpers.ts QueueSummaryState
type QueueSummaryState struct {
	DropPolicy   QueueDropPolicy
	DroppedCount int
	SummaryLines []string
}

// QueueCapManager manages queue capacity and drop policy
type QueueCapManager struct {
}

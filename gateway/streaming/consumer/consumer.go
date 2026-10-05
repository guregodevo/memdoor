package consumer

import (
	"sync"
	"time"

	"memdoor/gateway/infra"
)

// Consumer orchestrates event processing using discriminator, assembler, and deduplicator
// This is the main entry point for UI-agnostic event consumption
type Consumer struct {
	discriminator *Discriminator
	assembler     *StreamAssembler
	deduplicator  *Deduplicator
	handler       EventHandler
	options       ConsumerOptions
	pruneTimer    *time.Ticker
	stopChan      chan struct{}
	mu            sync.RWMutex
	verbose       bool
}

// NewConsumer creates a new event consumer with the given handler
func NewConsumer(handler EventHandler, options ConsumerOptions) *Consumer {
	c := &Consumer{
		discriminator: NewDiscriminator(options.EnableLogging),
		assembler:     NewStreamAssembler(options.MaxRunStates, options.EnableLogging),
		deduplicator:  NewDeduplicator(options.DeduplicationTTL, options.EnableLogging),
		handler:       handler,
		options:       options,
		stopChan:      make(chan struct{}),
		verbose:       options.EnableLogging,
	}

	// Start background pruning
	c.startPruning()

	return c
}

// ProcessEvent processes an incoming agent event and calls the handler with processed results
func (c *Consumer) ProcessEvent(event infra.AgentEvent) error {
	// Check if run is already finalized (deduplication)
	if c.deduplicator.IsFinalized(event.RunID) {
		if c.verbose {
			// Log skipped event
		}
		return nil
	}

	// Track session-run association
	if event.SessionID != "" {
		c.deduplicator.AddSessionRun(event.SessionID, event.RunID)
	}

	// Parse event data based on stream type
	parsedData, err := c.discriminator.ParseEventData(event)
	if err != nil {
		// Log error but don't fail - return error to caller
		return err
	}

	// Process event through assembler
	result, err := c.assembler.ProcessEvent(event, parsedData)
	if err != nil {
		return err
	}

	// Mark as finalized if result indicates completion
	if result.Finalized {
		c.deduplicator.MarkFinalized(event.RunID)
	}

	// Convert to ProcessedEvent for handler
	processedEvent := c.convertToProcessedEvent(event, result, parsedData)

	// Call the handler SYNCHRONOUSLY, in event order.
	//
	// This fired a goroutine PER EVENT. Events carry monotonic sequence numbers,
	// but a goroutine each let seq N+1 reach the handler before seq N — and the
	// handler appends the chunk to the transcript, so the text itself came out
	// scrambled: "program" rendered as "ograprm", "</anthropic>" as
	// "</anthrpioc>", adjacent chunks swapping across the whole reply
	// (2026-08-30).
	//
	// It was invisible until streamed deltas started arriving: with only a
	// handful of coarse events per turn the race almost never lost, and at 45
	// tokens a second it always does.
	//
	// The EMITTER side already learned this and says so in
	// infra.EventEmitter.EmitEvent — "a goroutine per event let seq N+1 be
	// delivered before seq N ... the client reassembled an out-of-order,
	// corrupted stream". The consumer needs the same discipline: ordered,
	// serial delivery. It cannot stall anything, because the handler only
	// enqueues a Bubble Tea message.
	if c.handler != nil {
		func() {
			// One bad handler must not take down the read loop.
			defer func() { _ = recover() }()
			c.handler(processedEvent)
		}()
	}

	return nil
}

// convertToProcessedEvent converts UpdateResult to ProcessedEvent for UI consumption
func (c *Consumer) convertToProcessedEvent(event infra.AgentEvent, result *UpdateResult, parsedData ParsedEventData) *ProcessedEvent {
	processed := &ProcessedEvent{
		RunID:         event.RunID,
		SessionID:     event.SessionID,
		EventType:     result.EventType,
		TextDelta:     result.TextDelta,
		FullText:      result.FullText,
		TextAppend:    result.TextAppend,
		ToolUpdate:    result.ToolUpdate,
		ThinkingState: result.ThinkingState,
		Finalized:     result.Finalized,
		Timestamp:     time.UnixMilli(event.Timestamp),
		Metadata:      make(map[string]interface{}),
	}

	// Add stream type to metadata
	processed.Metadata["stream"] = string(event.Stream)
	processed.Metadata["seq"] = event.Seq

	// Add parsed data to metadata so handlers can access it
	processed.Metadata["parsed_data"] = parsedData

	// Extract error messages
	switch data := parsedData.(type) {
	case ErrorEventData:
		// The runtime emits {"error": …}; a message-shaped frame is
		// honoured too. Reading only "message" left the TUI's spinner
		// on a turn the gateway had already failed (2026-09-13).
		processed.Error = data.Message
		if processed.Error == "" {
			processed.Error = data.Error
		}
	case LifecycleEventData:
		if data.Error != "" {
			processed.Error = data.Error
		}
	case ToolEventData:
		if data.Error != "" {
			processed.Error = data.Error
		}
	}

	return processed
}

// GetRunState returns the current state for a run
func (c *Consumer) GetRunState(runID string) *RunState {
	return c.assembler.GetRunState(runID)
}

// GetSessionRuns returns all run IDs for a session
func (c *Consumer) GetSessionRuns(sessionID string) []string {
	return c.deduplicator.GetSessionRuns(sessionID)
}

// ClearRun removes a run from the consumer state
func (c *Consumer) ClearRun(runID string) {
	c.assembler.ClearRun(runID)
}

// ClearSession removes all runs for a session
func (c *Consumer) ClearSession(sessionID string) {
	c.deduplicator.ClearSessionRuns(sessionID)
}

// GetStats returns statistics about the consumer
func (c *Consumer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"assembler":    c.assembler.GetStats(),
		"deduplicator": c.deduplicator.GetStats(),
	}
}

// startPruning starts background pruning of old state
func (c *Consumer) startPruning() {
	if c.options.PruneInterval <= 0 {
		return // Pruning disabled
	}

	c.pruneTimer = time.NewTicker(c.options.PruneInterval)

	go func() {
		for {
			select {
			case <-c.pruneTimer.C:
				c.prune()
			case <-c.stopChan:
				return
			}
		}
	}()
}

// prune removes old state from assembler and deduplicator
func (c *Consumer) prune() {
	// Prune assembler state (runs older than TTL)
	c.assembler.PruneOldRuns(c.options.DeduplicationTTL)

	// Prune deduplicator state
	c.deduplicator.Prune()
}

// Stop stops background pruning
func (c *Consumer) Stop() {
	close(c.stopChan)
	if c.pruneTimer != nil {
		c.pruneTimer.Stop()
	}
}

// SetHandler updates the event handler (thread-safe)
func (c *Consumer) SetHandler(handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// GetActiveRuns returns all active (non-finalized) run IDs
func (c *Consumer) GetActiveRuns() []string {
	return c.assembler.GetActiveRuns()
}

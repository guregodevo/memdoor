package queue

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"memdoor/gateway/logs"
)

// JobEnqueuer defines the interface for enqueueing jobs
// This allows components to depend on the interface instead of concrete implementation
type JobEnqueuer interface {
	EnqueueJob(job *AgentJob) error
}

// Lane represents a global lane type
// Pattern: OpenClaw src/process/lanes.ts
type Lane string

const (
	LaneMain     Lane = "main"     // Main user interactions (maxConcurrent: 4)
	LaneCron     Lane = "cron"     // Cron job executions (maxConcurrent: 4)
	LaneSubagent Lane = "subagent" // Sub-agent spawns (maxConcurrent: 8)
	LaneNested   Lane = "nested"   // Nested agent runs (maxConcurrent: 8)
)

// ResponseWriter is a callback to send response back to client
type ResponseWriter func(response interface{}) error

// AgentJob represents a job to be executed by an agent
type AgentJob struct {
	SessionKey        string
	Message           interface{} // The message to process (type depends on consumer)
	GlobalLane        Lane
	EnqueueTime       time.Time
	StartTime         time.Time
	Context           context.Context
	ResponseWriter    ResponseWriter // Optional callback to send response
	ExtraSystemPrompt string         // Optional extra system prompt to inject (Week 28: ping-pong)
	AgentID           string         // Target agent whose config (palette + prompt) this job runs as; set for spawned subagents
	BuddyTools        []string       // The agent's tool palette, resolved at spawn time and carried on the job (so the executor doesn't re-fetch it under DB contention)
	Workdir           string         // The requester's working directory, carried to a spawned subagent so it works where its requester works (empty = server default)
	Timeout           time.Duration  // How long a spawned subagent may run before its turn is cancelled (0 = no limit); the requester's runTimeoutSeconds
}

// SessionLane represents a per-session FIFO queue
// Ensures only one job runs per session at a time
type SessionLane struct {
	sessionKey string
	queue      []*AgentJob
	active     bool
	mu         sync.Mutex
}

// Enqueue adds a job to the session lane
func (sl *SessionLane) Enqueue(job *AgentJob) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	sl.queue = append(sl.queue, job)
}

// Dequeue removes and returns the next job from the session lane
// Returns nil if queue is empty
func (sl *SessionLane) Dequeue() *AgentJob {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if len(sl.queue) == 0 {
		return nil
	}

	job := sl.queue[0]
	sl.queue = sl.queue[1:]
	return job
}

// IsEmpty returns true if the session lane has no jobs
func (sl *SessionLane) IsEmpty() bool {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	return len(sl.queue) == 0
}

// Len returns the number of jobs in the session lane
func (sl *SessionLane) Len() int {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	return len(sl.queue)
}

// GlobalLane represents a global lane with concurrency limits
type GlobalLane struct {
	name          Lane
	maxConcurrent int
	active        int
	queue         []*SessionLane // Queue of session lanes waiting to execute
	mu            sync.Mutex
}

// Enqueue adds a session lane to the global lane
func (gl *GlobalLane) Enqueue(sessionLane *SessionLane) {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	gl.queue = append(gl.queue, sessionLane)
}

// Dequeue removes and returns the next session lane from the global lane
// Returns nil if queue is empty
func (gl *GlobalLane) Dequeue() *SessionLane {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	if len(gl.queue) == 0 {
		return nil
	}

	sessionLane := gl.queue[0]
	gl.queue = gl.queue[1:]
	return sessionLane
}

// CanAccept returns true if the global lane can accept more concurrent jobs
func (gl *GlobalLane) CanAccept() bool {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	return gl.active < gl.maxConcurrent
}

// IncrementActive increments the active job count
func (gl *GlobalLane) IncrementActive() {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	gl.active++
}

// DecrementActive decrements the active job count
func (gl *GlobalLane) DecrementActive() {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	gl.active--
}

// IsEmpty returns true if the global lane has no queued session lanes
func (gl *GlobalLane) IsEmpty() bool {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	return len(gl.queue) == 0
}

// Len returns the number of session lanes in the global lane queue
func (gl *GlobalLane) Len() int {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	return len(gl.queue)
}

// JobExecutor is a function that executes a job
type JobExecutor func(ctx context.Context, job *AgentJob) error

// QueueManager manages session lanes and global lanes
type QueueManager struct {
	sessionLanes map[string]*SessionLane // session:<key> → lane
	globalLanes  map[Lane]*GlobalLane    // main, cron, subagent
	executor     JobExecutor
	log          *logs.EventLogger
	mu           sync.RWMutex
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

// NewQueueManager creates a new queue manager
func NewQueueManager(executor JobExecutor, verbose bool) *QueueManager {
	qm := &QueueManager{
		sessionLanes: make(map[string]*SessionLane),
		globalLanes:  make(map[Lane]*GlobalLane),
		executor:     executor,
		log:          logs.New("Queue"),
		stopChan:     make(chan struct{}),
	}

	// Initialize global lanes with default concurrency
	qm.globalLanes[LaneMain] = &GlobalLane{
		name:          LaneMain,
		maxConcurrent: 4, // Default: 4 concurrent main jobs
	}

	qm.globalLanes[LaneCron] = &GlobalLane{
		name:          LaneCron,
		maxConcurrent: 4, // Default: 4 concurrent cron jobs
	}

	qm.globalLanes[LaneSubagent] = &GlobalLane{
		name:          LaneSubagent,
		maxConcurrent: 8, // Default: 8 concurrent subagent jobs
	}

	qm.globalLanes[LaneNested] = &GlobalLane{
		name:          LaneNested,
		maxConcurrent: 8, // Default: 8 concurrent nested jobs
	}

	return qm
}

// Start starts the queue manager's background workers
func (qm *QueueManager) Start() {
	// Start global lane processors
	for _, lane := range qm.globalLanes {
		qm.wg.Add(1)
		go qm.processGlobalLane(lane)
	}
}

// Stop stops the queue manager gracefully
func (qm *QueueManager) Stop() {
	close(qm.stopChan)
	qm.wg.Wait()
}

// EnqueueJob enqueues a job for execution
func (qm *QueueManager) EnqueueJob(job *AgentJob) error {
	// 1. Get or create session lane
	sessionLane := qm.getOrCreateSessionLane(job.SessionKey)

	// 2. Enqueue job into session lane
	sessionLane.Enqueue(job)

	// 3. If session lane is not active, enqueue it into global lane
	sessionLane.mu.Lock()
	if !sessionLane.active {
		sessionLane.active = true
		sessionLane.mu.Unlock()

		globalLane := qm.getGlobalLane(job.GlobalLane)
		globalLane.Enqueue(sessionLane)

		qm.log.Debug("Job enqueued",
			slog.String("session", job.SessionKey),
			slog.String("lane", string(job.GlobalLane)))
	} else {
		sessionLane.mu.Unlock()
		qm.log.Debug("Job queued in session lane",
			slog.String("session", job.SessionKey),
			slog.String("status", "session lane active"))
	}

	return nil
}

// getOrCreateSessionLane gets or creates a session lane
func (qm *QueueManager) getOrCreateSessionLane(sessionKey string) *SessionLane {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if lane, ok := qm.sessionLanes[sessionKey]; ok {
		return lane
	}

	lane := &SessionLane{
		sessionKey: sessionKey,
		queue:      make([]*AgentJob, 0),
		active:     false,
	}

	qm.sessionLanes[sessionKey] = lane
	return lane
}

// getGlobalLane gets a global lane by name
func (qm *QueueManager) getGlobalLane(name Lane) *GlobalLane {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	if lane, ok := qm.globalLanes[name]; ok {
		return lane
	}

	// Return main lane as fallback
	return qm.globalLanes[LaneMain]
}

// processGlobalLane processes jobs from a global lane
func (qm *QueueManager) processGlobalLane(lane *GlobalLane) {
	defer qm.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-qm.stopChan:
			return
		case <-ticker.C:
			// Check if we can accept more concurrent jobs
			if !lane.CanAccept() {
				continue
			}

			// Dequeue next session lane
			sessionLane := lane.Dequeue()
			if sessionLane == nil {
				continue
			}

			// Increment active count
			lane.IncrementActive()

			// Process session lane in background
			qm.wg.Add(1)
			go func(sl *SessionLane, gl *GlobalLane) {
				defer qm.wg.Done()
				defer gl.DecrementActive()

				qm.processSessionLane(sl, gl)
			}(sessionLane, lane)
		}
	}
}

// processSessionLane processes all jobs in a session lane
func (qm *QueueManager) processSessionLane(sessionLane *SessionLane, globalLane *GlobalLane) {
	for {
		// Dequeue next job
		job := sessionLane.Dequeue()
		if job == nil {
			// No more jobs in session lane
			sessionLane.mu.Lock()
			sessionLane.active = false
			sessionLane.mu.Unlock()

			qm.log.Debug("Session lane complete",
				slog.String("session", sessionLane.sessionKey))
			return
		}

		// Log if queued for more than 2 seconds
		queueTime := time.Since(job.EnqueueTime)
		if queueTime > 2*time.Second {
			qm.log.Warn("Job queued longer than threshold",
				slog.String("session", sessionLane.sessionKey),
				slog.Duration("queue_time", queueTime),
				slog.Duration("threshold", 2*time.Second))
		}

		// Execute job
		job.StartTime = time.Now()
		if err := qm.executor(job.Context, job); err != nil {
			qm.log.WithError(err).Error("Job execution failed",
				slog.String("session", sessionLane.sessionKey))
		}

		executionTime := time.Since(job.StartTime)
		qm.log.Debug("Job completed",
			slog.String("session", sessionLane.sessionKey),
			slog.Duration("execution_time", executionTime))
	}
}

// GetQueueDepth returns the queue depth for debugging
func (qm *QueueManager) GetQueueDepth() map[string]interface{} {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	sessionDepths := make(map[string]int)
	for key, lane := range qm.sessionLanes {
		if len := lane.Len(); len > 0 {
			sessionDepths[key] = len
		}
	}

	globalDepths := make(map[Lane]map[string]interface{})
	for name, lane := range qm.globalLanes {
		lane.mu.Lock()
		globalDepths[name] = map[string]interface{}{
			"queued": len(lane.queue), // Direct access while holding lock
			"active": lane.active,
			"max":    lane.maxConcurrent,
		}
		lane.mu.Unlock()
	}

	return map[string]interface{}{
		"sessions": sessionDepths,
		"global":   globalDepths,
	}
}

// SetGlobalLaneMaxConcurrent sets the max concurrent jobs for a global lane
func (qm *QueueManager) SetGlobalLaneMaxConcurrent(lane Lane, max int) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if globalLane, ok := qm.globalLanes[lane]; ok {
		globalLane.mu.Lock()
		globalLane.maxConcurrent = max
		globalLane.mu.Unlock()
		return nil
	}

	return fmt.Errorf("global lane %s not found", lane)
}

// EnqueueSubagentJob enqueues a subagent job for execution
// Pattern: Convenience method for tools.SessionsSpawnTool
// This simplifies subagent job creation and ensures correct lane assignment
func (qm *QueueManager) EnqueueSubagentJob(runID, sessionKey, agentID, message string, tools []string, systemPrompt, workdir string, timeout time.Duration) error {
	// Create job with subagent lane. The agent's config (palette + prompt) was
	// resolved at spawn time and is carried HERE, so the executor runs this job AS
	// that agent without re-fetching from the DB.
	job := &AgentJob{
		SessionKey:        sessionKey,
		Message:           message, // Task message for the subagent to process
		GlobalLane:        LaneSubagent,
		EnqueueTime:       time.Now(),
		Context:           context.Background(),
		AgentID:           agentID,
		BuddyTools:        tools,
		ExtraSystemPrompt: systemPrompt,
		Workdir:           workdir,
		Timeout:           timeout,
	}

	// Enqueue in subagent lane
	return qm.EnqueueJob(job)
}

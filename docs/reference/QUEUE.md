# SessionLane Queue System

**Developer Reference for Memdoor's Queue-Based Execution Model**

**Version**: 1.0
**Date**: 2026-04-18

## Table of Contents
- [Overview](#overview)
- [Why We Built It](#why-we-built-it)
- [How It Works](#how-it-works)
- [Architecture](#architecture)
- [Race Condition Prevention](#race-condition-prevention)
- [Usage Patterns](#usage-patterns)
- [API Reference](#api-reference)
- [Testing](#testing)

---

## Overview

The SessionLane Queue is Memdoor's **per-session FIFO execution queue** that ensures only one agent job executes per session at a time. This prevents race conditions, conversation corruption, and the dreaded "orphaned tool_use blocks" error from LLM API.

**Key Concept**: Every session gets its own dedicated lane (queue). Jobs for that session are serialized through the lane, preventing concurrent modifications to conversation state.

```
Session A: [Job1] → [Job2] → [Job3]  (sequential)
Session B: [Job4] → [Job5]            (sequential, parallel to Session A)
Session C: [Job6]                     (sequential, parallel to A & B)
```

## Why We Built It

### The Problem: Orphaned Tool_Use Blocks

**Symptom**: LLM API error:
```
"messages.8: tool_use ids were found without tool_result blocks immediately after:
toolu_vrtx_0164adqYXd2FtD5Jjae1Xpxx"
```

**Root Cause**: Race condition when multiple threads concurrently execute agent runs on the same session:

```
Timeline of Failure (WITHOUT Queue):
─────────────────────────────────────────────────────────────
t0: Thread A loads messages [1..10]
t1: Thread B loads messages [1..10]  ← Same snapshot!
t2: Thread A generates tool_use block #11
t3: Thread B generates tool_use block #12
t4: Thread A crashes before saving
t5: Thread B saves successfully → file has [1..10, 12]  ← Missing #11!
t6: Next load: orphaned tool_use #11 has no tool_result
t7: LLM API rejects conversation 
```

**Impact**:
- Conversation corruption
- Agent execution failures
- User frustration
- Data loss

### The Solution: SessionLane Queue

**With Queue**:
```
Timeline of Success (WITH Queue):
─────────────────────────────────────────────────────────────
t0: Thread A enqueues Job A for session:123
t1: Thread B enqueues Job B for session:123
t2: SessionLane processes Job A (Thread B waits)
t3: Job A completes, saves messages [1..11]
t4: SessionLane processes Job B (loads fresh state [1..11])
t5: Job B completes, saves messages [1..12]
t6: No orphaned blocks 
```

**Benefits**:
-  No race conditions
-  Per-session serialization
-  Cross-session parallelism
-  Conversation integrity
-  Predictable execution order

---

## How It Works

### 1. Per-Session FIFO Queues

Each session gets a dedicated `SessionLane` - a FIFO queue that serializes jobs:

```go
type SessionLane struct {
    sessionKey string        // e.g., "workspace:1:channel:test"
    queue      []*AgentJob   // FIFO queue of pending jobs
    active     bool          // Is a job currently executing?
    mu         sync.Mutex    // Thread-safe queue operations
}
```

**Key Property**: Only one job runs per lane at a time.

### 2. Job Enqueuing

Jobs are enqueued with session context:

```go
job := &queue.AgentJob{
    SessionKey:     "workspace:1:channel:test",
    Message:        "Hello, agent!",
    GlobalLane:     queue.LaneMain,
    Context:        ctx,
    ResponseWriter: responseCallback,
}

queueManager.EnqueueJob(job)
```

### 3. Execution Flow

```
┌─────────────────────────────────────────────────────────┐
│ 1. Job arrives → EnqueueJob(job)                       │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ 2. Get or create SessionLane for job.SessionKey        │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ 3. Append job to lane's FIFO queue                     │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ 4. If lane not active, trigger processQueue()          │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ 5. processQueue() dequeues and executes FIFO           │
│    - Mark lane as active                               │
│    - Execute job (calls executeAgentJob)               │
│    - Wait for completion                               │
│    - Process next job in queue (if any)                │
│    - Mark lane as inactive when empty                  │
└─────────────────────────────────────────────────────────┘
```

### 4. Worker Pool for Cross-Session Parallelism

While jobs are serialized **per session**, different sessions can execute in parallel:

```go
// Global lanes with worker pools
const (
    LaneMain     Lane = "main"     // maxConcurrent: 4
    LaneCron     Lane = "cron"     // maxConcurrent: 4
    LaneSubagent Lane = "subagent" // maxConcurrent: 8
    LaneNested   Lane = "nested"   // maxConcurrent: 8
)
```

**Example**:
```
Global Lane: Main (4 workers)
├─ Worker 1: Session A → Job 1
├─ Worker 2: Session B → Job 4
├─ Worker 3: Session C → Job 6
└─ Worker 4: Session D → Job 7

Session A lane: [Job1] → [Job2] → [Job3]  (sequential)
Session B lane: [Job4] → [Job5]            (sequential)
Session C lane: [Job6]                     (sequential)
Session D lane: [Job7] → [Job8]            (sequential)
```

---

## Architecture

### Components

#### 1. `QueueManager` (gateway/queue/lanes.go)
Central orchestrator for all queue operations.

```go
type QueueManager struct {
    executor        JobExecutor                    // Executes jobs
    sessionLanes    map[string]*SessionLane        // Per-session queues
    globalLanes     map[Lane]*sync.WaitGroup       // Worker pools
    workerSemaphores map[Lane]chan struct{}        // Worker limits
    mu              sync.RWMutex
}
```

**Responsibilities**:
- Create/manage SessionLanes
- Dispatch jobs to appropriate lanes
- Enforce worker pool limits
- Provide cleanup

#### 2. `SessionLane` (gateway/queue/lanes.go)
Per-session FIFO queue.

```go
type SessionLane struct {
    sessionKey string
    queue      []*AgentJob
    active     bool
    mu         sync.Mutex
}
```

**Responsibilities**:
- Maintain FIFO queue for session
- Serialize job execution
- Signal when queue is empty

#### 3. `AgentJob` (gateway/queue/lanes.go)
Job specification.

```go
type AgentJob struct {
    SessionKey        string              // Session identifier
    Message           interface{}         // Message payload
    GlobalLane        Lane                // Which worker pool?
    Context           context.Context     // Execution context
    ResponseWriter    ResponseWriter      // Callback for response
    ExtraSystemPrompt string              // Optional system prompt
}
```

#### 4. `JobEnqueuer` Interface (gateway/queue/lanes.go)
Dependency injection interface.

```go
type JobEnqueuer interface {
    EnqueueJob(job *AgentJob) error
}
```

**Purpose**: Allows components to depend on interface instead of concrete `QueueManager`.

---

## Race Condition Prevention

### How Locks Are Prevented

The SessionLane queue prevents race conditions through **serialization**, not locks on conversation state.

**Traditional Locking Approach** :
```go
// ANTI-PATTERN: Lock conversation file
conversationLock.Lock()
defer conversationLock.Unlock()
messages := loadConversation()
response := agent.Execute(messages)
saveConversation(append(messages, response))
```

**Problems**:
- Deadlock risk
- Lock contention
- Hard to debug
- Doesn't scale across processes

**SessionLane Approach** :
```go
// PATTERN: Enqueue job, let queue serialize
queueManager.EnqueueJob(&AgentJob{
    SessionKey: "session:123",
    Message:    "Hello",
    // No locks needed!
})
```

**Why It Works**:
1. **FIFO Guarantee**: Jobs execute in order enqueued
2. **One-at-a-time**: Lane's `active` flag prevents concurrent execution
3. **Fresh State**: Each job loads latest conversation state
4. **Atomic Save**: Each job saves after completion
5. **No Contention**: Different sessions execute in parallel

### Conversation Integrity

**Without Queue**:
```
Thread A: Load [1..10] → Generate 11 → Crash
Thread B: Load [1..10] → Generate 12 → Save [1..10, 12]
Result: Missing message 11 
```

**With Queue**:
```
Job A: Load [1..10] → Generate 11 → Save [1..11]
Job B: Load [1..11] → Generate 12 → Save [1..12]
Result: Complete conversation 
```

---

## Usage Patterns

### Pattern 1: Chat Message Handling

**Location**: `gateway/chat_server.go:1598-1645`

```go
// Route user message through queue for serialization
responseChan := make(chan *AgentResponse, 1)
errorChan := make(chan error, 1)

responseWriter := func(resp interface{}) error {
    if agentResp, ok := resp.(*AgentResponse); ok {
        responseChan <- agentResp
    } else if err, ok := resp.(error); ok {
        errorChan <- err
    }
    return nil
}

job := &queue.AgentJob{
    SessionKey:     sessionKey,
    Message:        userMessage,
    GlobalLane:     queue.LaneMain,
    Context:        ctx,
    ResponseWriter: responseWriter,
}

s.queueManager.EnqueueJob(job)

// Wait synchronously for response
select {
case response := <-responseChan:
    // Success - send to client
case err := <-errorChan:
    // Error - return to client
case <-time.After(180 * time.Second):
    // Timeout
}
```

**Key Points**:
- Synchronous from caller's perspective
- Async execution via queue
- Channel-based response pattern

### Pattern 2: Cron Jobs

**Location**: `gateway/server_jobs.go:135-195`

```go
func (s *Server) executeCronJob(ctx context.Context, job *config.CronJob, agentID string, sessionKey string) error {
    responseChan := make(chan *AgentResponse, 1)
    errorChan := make(chan error, 1)

    queueJob := &queue.AgentJob{
        SessionKey:     sessionKey,
        Message:        job.Message,
        GlobalLane:     queue.LaneMain,
        EnqueueTime:    time.Now(),
        Context:        ctx,
        ResponseWriter: responseWriter,
    }

    s.queueManager.EnqueueJob(queueJob)

    // Wait for completion
    select {
    case response = <-responseChan:
        return nil
    case err := <-errorChan:
        return err
    case <-time.After(180 * time.Second):
        return fmt.Errorf("timeout")
    }
}
```

### Pattern 3: A2A (Agent-to-Agent) Execution

**Location**: `gateway/run_tracker.go:182-259`

```go
func (rt *RunTracker) ExecuteAsync(...) {
    go func() {
        // Create queue job
        queueJob := &queue.AgentJob{
            SessionKey:     run.SessionKey,
            Message:        run.Message,
            GlobalLane:     queue.LaneMain,
            Context:        ctx,
            ResponseWriter: responseWriter,
        }

        // Enqueue and wait
        rt.enqueue.EnqueueJob(queueJob)

        select {
        case response := <-responseChan:
            rt.SetRunResponse(run.RunID, response)
        case err := <-errorChan:
            rt.SetRunError(run.RunID, err)
        case <-time.After(180 * time.Second):
            rt.SetRunError(run.RunID, fmt.Errorf("timeout"))
        }
    }()
}
```

---

## API Reference

### `JobEnqueuer` Interface

```go
type JobEnqueuer interface {
    EnqueueJob(job *AgentJob) error
}
```

**Method**: `EnqueueJob(job *AgentJob) error`
- **Purpose**: Enqueue a job for execution
- **Parameters**:
  - `job`: Job specification with session, message, lane
- **Returns**: `error` if enqueue fails
- **Thread-safe**: Yes

### `AgentJob` Structure

```go
type AgentJob struct {
    SessionKey        string              // Required: Session identifier
    Message           interface{}         // Required: Message payload
    GlobalLane        Lane                // Required: Worker pool
    EnqueueTime       time.Time           // Set by queue
    StartTime         time.Time           // Set by queue
    Context           context.Context     // Optional: Execution context
    ResponseWriter    ResponseWriter      // Optional: Response callback
    ExtraSystemPrompt string              // Optional: System prompt injection
}
```

### `Lane` Constants

```go
const (
    LaneMain     Lane = "main"     // User interactions (4 workers)
    LaneCron     Lane = "cron"     // Cron jobs (4 workers)
    LaneSubagent Lane = "subagent" // Subagent spawns (8 workers)
    LaneNested   Lane = "nested"   // Nested runs (8 workers)
)
```

### `ResponseWriter` Callback

```go
type ResponseWriter func(response interface{}) error
```

**Purpose**: Callback to receive job execution result.

**Parameters**:
- `response`: Either `*AgentResponse` (success) or `error` (failure)

**Returns**: `error` if callback fails

**Pattern**:
```go
responseWriter := func(resp interface{}) error {
    if agentResp, ok := resp.(*AgentResponse); ok {
        responseChan <- agentResp
    } else if err, ok := resp.(error); ok {
        errorChan <- err
    }
    return nil
}
```

---

## Testing

### Unit Tests

**Location**: `gateway/queue/queue_test.go`

Key tests:
- `TestQueueManager_EnqueueJob`: Basic enqueue
- `TestQueueManager_SessionSerialization`: Per-session FIFO
- `TestQueueManager_MultiSessionParallelism`: Cross-session parallelism
- `TestSessionLane_FIFO`: FIFO ordering
- `TestSessionLane_Concurrency`: Thread safety

### Integration Tests

**Scenario 1**: Basic A2A
```bash
./memdoor agent --message "Hey @coder! Test message" --channel test --agent-id writer
sleep 20
./memdoor messages --channel test --limit 5
./memdoor logs query --regex "tool_use.*without.*tool_result" --limit 10
```

**Expected**: No orphaned tool_use errors.

**Scenario 2**: Concurrent Execution
```bash
./memdoor agent --message "Test 1" --channel test --agent-id writer &
./memdoor agent --message "Test 2" --channel test --agent-id writer &
wait
./memdoor logs errors --limit 10
```

**Expected**: Both messages processed, no errors.

**Scenario 3**: Load Test
```bash
./scripts/load_test/progressive-load-test.sh
# or the short version: ./scripts/load_test/quick-progressive-test.sh
```

**Expected**:
- All messages processed
- No orphaned tool_use blocks
- No race condition errors

### Debugging Queue Issues

**Check queue state**:
```bash
# Check for queue-related errors
./memdoor logs query --regex "Enqueue|SessionLane|queue" --limit 50

# Check for orphaned tool_use
./memdoor logs query --regex "tool_use.*without.*tool_result" --limit 10

# Check execution order
./memdoor logs query --regex "Executing job|Job completed" --limit 20
```

**Common Issues**:

1. **Job not executing**: Check worker pool limits
2. **Out-of-order execution**: Check sessionKey consistency
3. **Timeouts**: Check 180s timeout, increase if needed
4. **Orphaned tool_use**: Ensure ALL code paths use queue

---

## Migration Guide

### Before: Direct ProcessMessage Calls 

```go
// ANTI-PATTERN: Direct execution bypasses queue
session, _ := s.sessions.GetOrCreateSession(sessionKey, "main")
response, err := s.agent.ProcessMessage(ctx, message, session, runID, "")
// Risk: Race conditions, orphaned tool_use blocks
```

### After: Queue Routing 

```go
// PATTERN: Route through queue for serialization
responseChan := make(chan *AgentResponse, 1)
errorChan := make(chan error, 1)

responseWriter := func(resp interface{}) error {
    if agentResp, ok := resp.(*AgentResponse); ok {
        responseChan <- agentResp
    } else if err, ok := resp.(error); ok {
        errorChan <- err
    }
    return nil
}

job := &queue.AgentJob{
    SessionKey:     sessionKey,
    Message:        message,
    GlobalLane:     queue.LaneMain,
    Context:        ctx,
    ResponseWriter: responseWriter,
}

s.queueManager.EnqueueJob(job)

// Wait synchronously
select {
case response := <-responseChan:
    // Handle success
case err := <-errorChan:
    // Handle error
case <-time.After(180 * time.Second):
    // Handle timeout
}
```

---

## Performance Characteristics

### Throughput

- **Per-session**: Sequential (FIFO)
- **Cross-session**: Parallel (up to worker pool limit)
- **Typical latency**: <10ms enqueue overhead
- **Worker pools**: 4-8 concurrent sessions

### Scalability

**Current limits**:
- `LaneMain`: 4 concurrent sessions
- `LaneSubagent`: 8 concurrent sessions
- No limit on number of sessions (queued)

**To increase throughput**:
1. Increase worker pool sizes in `queue/lanes.go`
2. Add more global lanes
3. Optimize job execution time

### Memory

- SessionLanes are cleaned up after last job completes
- Jobs are garbage collected after execution
- No persistent state (stateless queue)

---

## Best Practices

### 1. Always Use Queue for Agent Execution

**DO**:
```go
queueManager.EnqueueJob(&queue.AgentJob{...})
```

**DON'T**:
```go
agent.ProcessMessage(ctx, message, session, runID, "")  // Bypasses queue!
```

### 2. Use Synchronous Response Pattern

```go
// Create response channels
responseChan := make(chan *AgentResponse, 1)
errorChan := make(chan error, 1)

// Enqueue job
queueManager.EnqueueJob(job)

// Wait synchronously
select {
case response := <-responseChan:
    // Success
case err := <-errorChan:
    // Error
case <-time.After(180 * time.Second):
    // Timeout
}
```

### 3. Depend on Interface, Not Implementation

```go
// DO: Depend on interface
type MyComponent struct {
    enqueue queue.JobEnqueuer
}

// DON'T: Depend on concrete type
type MyComponent struct {
    queueManager *queue.QueueManager  // Tight coupling
}
```

### 4. Use Consistent Session Keys

```go
// Consistent format
sessionKey := fmt.Sprintf("workspace:%s:channel:%s", workspaceID, channelID)

// Avoid: Inconsistent keys split session lanes
// "channel:test" vs "workspace:1:channel:test"  ← Different lanes!
```

### 5. Set Appropriate Timeouts

```go
// Most jobs: 180 seconds (3 minutes)
case <-time.After(180 * time.Second):

// Long-running: Increase timeout
case <-time.After(300 * time.Second):

// Quick jobs: Decrease timeout
case <-time.After(30 * time.Second):
```

---

## Related Documentation

- [Architecture Overview](./ARCHITECTURE.md)
- [Cron Jobs](./CRON.md)
- [Logging](./LOGS.md)
- [CLI Reference](./CLI.md)

---

## Changelog

**2026-03-13**: Initial version - SessionLane queue system documentation created

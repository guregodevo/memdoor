package queue

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestSessionLane_EnqueueDequeue tests basic session lane operations
func TestSessionLane_EnqueueDequeue(t *testing.T) {
	sl := &SessionLane{sessionKey: "test_session"}

	// Test empty queue
	if !sl.IsEmpty() {
		t.Error("New session lane should be empty")
	}

	if sl.Len() != 0 {
		t.Errorf("Empty lane length = %d, want 0", sl.Len())
	}

	// Enqueue jobs
	job1 := &AgentJob{SessionKey: "test_session", Message: "job1"}
	job2 := &AgentJob{SessionKey: "test_session", Message: "job2"}

	sl.Enqueue(job1)
	sl.Enqueue(job2)

	if sl.Len() != 2 {
		t.Errorf("Lane length after enqueue = %d, want 2", sl.Len())
	}

	if sl.IsEmpty() {
		t.Error("Lane should not be empty after enqueue")
	}

	// Dequeue in FIFO order
	dequeuedJob := sl.Dequeue()
	if dequeuedJob != job1 {
		t.Error("First dequeued job should be job1")
	}

	if sl.Len() != 1 {
		t.Errorf("Lane length after first dequeue = %d, want 1", sl.Len())
	}

	dequeuedJob = sl.Dequeue()
	if dequeuedJob != job2 {
		t.Error("Second dequeued job should be job2")
	}

	if !sl.IsEmpty() {
		t.Error("Lane should be empty after dequeuing all jobs")
	}

	// Dequeue from empty queue should return nil
	dequeuedJob = sl.Dequeue()
	if dequeuedJob != nil {
		t.Error("Dequeue from empty queue should return nil")
	}
}

// TestSessionLane_Concurrent tests concurrent access to session lane
func TestSessionLane_Concurrent(t *testing.T) {
	sl := &SessionLane{sessionKey: "test_session"}
	var wg sync.WaitGroup

	// Enqueue 100 jobs concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			job := &AgentJob{
				SessionKey: "test_session",
				Message:    id,
			}
			sl.Enqueue(job)
		}(i)
	}

	wg.Wait()

	if sl.Len() != 100 {
		t.Errorf("Lane length after concurrent enqueue = %d, want 100", sl.Len())
	}

	// Dequeue all jobs
	count := 0
	for !sl.IsEmpty() {
		job := sl.Dequeue()
		if job == nil {
			break
		}
		count++
	}

	if count != 100 {
		t.Errorf("Dequeued %d jobs, want 100", count)
	}
}

// TestGlobalLane_ConcurrencyLimit tests global lane concurrency limiting
func TestGlobalLane_ConcurrencyLimit(t *testing.T) {
	gl := &GlobalLane{
		name:          LaneMain,
		maxConcurrent: 2,
		active:        0,
	}

	// Should be able to start 2 concurrent jobs
	if !gl.CanAccept() {
		t.Error("Should be able to start first job")
	}
	gl.IncrementActive()

	if !gl.CanAccept() {
		t.Error("Should be able to start second job")
	}
	gl.IncrementActive()

	// Third job should exceed limit
	if gl.CanAccept() {
		t.Error("Should not be able to start third job (exceeds limit)")
	}

	// Mark one job as done
	gl.DecrementActive()

	// Now should be able to start again
	if !gl.CanAccept() {
		t.Error("Should be able to start after marking one done")
	}
}

// TestGlobalLane_EnqueueDequeue tests global lane queue operations
func TestGlobalLane_EnqueueDequeue(t *testing.T) {
	gl := &GlobalLane{
		name:          LaneMain,
		maxConcurrent: 4,
	}

	sl1 := &SessionLane{sessionKey: "session1"}
	sl2 := &SessionLane{sessionKey: "session2"}

	// Enqueue session lanes
	gl.Enqueue(sl1)
	gl.Enqueue(sl2)

	if gl.Len() != 2 {
		t.Errorf("Global lane queue length = %d, want 2", gl.Len())
	}

	// Dequeue in FIFO order
	dequeued := gl.Dequeue()
	if dequeued != sl1 {
		t.Error("First dequeued lane should be sl1")
	}

	dequeued = gl.Dequeue()
	if dequeued != sl2 {
		t.Error("Second dequeued lane should be sl2")
	}

	if gl.Len() != 0 {
		t.Errorf("Global lane queue should be empty, got length %d", gl.Len())
	}
}

// TestQueueManager_EnqueueJob tests basic job enqueue
func TestQueueManager_EnqueueJob(t *testing.T) {
	// A channel, not a bool plus a sleep: the executor runs on the lane's own
	// goroutine, so an unsynchronised flag is a data race the detector fails
	// on — and the sleep made it look like a timing question rather than a
	// correctness one.
	called := make(chan struct{}, 1)
	executor := func(ctx context.Context, job *AgentJob) error {
		select {
		case called <- struct{}{}:
		default:
		}
		return nil
	}

	qm := NewQueueManager(executor, false)
	qm.Start() // Start the queue manager workers

	// Enqueue a job
	job := &AgentJob{
		SessionKey:  "test_session",
		Message:     "test message",
		GlobalLane:  LaneMain,
		EnqueueTime: time.Now(),
		Context:     context.Background(),
	}

	err := qm.EnqueueJob(job)
	if err != nil {
		t.Fatalf("EnqueueJob failed: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Error("Executor should have been called")
	}
}

// TestQueueManager_MultipleJobs tests processing multiple jobs
func TestQueueManager_MultipleJobs(t *testing.T) {
	var mu sync.Mutex
	executedCount := 0
	executor := func(ctx context.Context, job *AgentJob) error {
		mu.Lock()
		executedCount++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // Simulate work
		return nil
	}

	qm := NewQueueManager(executor, false)
	qm.Start() // Start the queue manager workers

	// Enqueue 10 jobs
	for i := 0; i < 10; i++ {
		job := &AgentJob{
			SessionKey:  "test_session",
			Message:     i,
			GlobalLane:  LaneMain,
			EnqueueTime: time.Now(),
			Context:     context.Background(),
		}
		err := qm.EnqueueJob(job)
		if err != nil {
			t.Fatalf("EnqueueJob %d failed: %v", i, err)
		}
	}

	// Wait for all jobs to execute
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if executedCount != 10 {
		t.Errorf("Executed %d jobs, want 10", executedCount)
	}
}

// TestQueueManager_SessionIsolation tests that sessions are processed independently
func TestQueueManager_SessionIsolation(t *testing.T) {
	var mu sync.Mutex
	executedSessions := make(map[string]int)

	executor := func(ctx context.Context, job *AgentJob) error {
		mu.Lock()
		executedSessions[job.SessionKey]++
		mu.Unlock()
		time.Sleep(20 * time.Millisecond) // Simulate work
		return nil
	}

	qm := NewQueueManager(executor, false)
	qm.Start() // Start the queue manager workers

	// Enqueue jobs for 3 different sessions
	for i := 0; i < 5; i++ {
		for _, session := range []string{"session1", "session2", "session3"} {
			job := &AgentJob{
				SessionKey:  session,
				Message:     i,
				GlobalLane:  LaneMain,
				EnqueueTime: time.Now(),
				Context:     context.Background(),
			}
			err := qm.EnqueueJob(job)
			if err != nil {
				t.Fatalf("EnqueueJob failed: %v", err)
			}
		}
	}

	// Wait for all jobs to execute
	time.Sleep(1 * time.Second)

	mu.Lock()
	defer mu.Unlock()

	// Each session should have executed 5 jobs
	for _, session := range []string{"session1", "session2", "session3"} {
		count := executedSessions[session]
		if count != 5 {
			t.Errorf("Session %s executed %d jobs, want 5", session, count)
		}
	}
}

// TestLaneConstants tests that lane constants are defined correctly
func TestLaneConstants(t *testing.T) {
	lanes := []Lane{LaneMain, LaneCron, LaneSubagent, LaneNested}

	// Verify lane values
	if LaneMain != "main" {
		t.Errorf("LaneMain = %q, want 'main'", LaneMain)
	}

	if LaneCron != "cron" {
		t.Errorf("LaneCron = %q, want 'cron'", LaneCron)
	}

	if LaneSubagent != "subagent" {
		t.Errorf("LaneSubagent = %q, want 'subagent'", LaneSubagent)
	}

	if LaneNested != "nested" {
		t.Errorf("LaneNested = %q, want 'nested'", LaneNested)
	}

	// Verify all lanes are unique
	seen := make(map[Lane]bool)
	for _, lane := range lanes {
		if seen[lane] {
			t.Errorf("Duplicate lane: %q", lane)
		}
		seen[lane] = true
	}
}

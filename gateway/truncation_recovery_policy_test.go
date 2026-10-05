package gateway

import "testing"

// The whole recovery sequence, testable without a single inference — which is
// the point of putting the policy behind an interface rather than leaving it as
// flags and counters inside the turn loop.
func TestRecoverySequenceEscalatesThenResumesThenGivesUp(t *testing.T) {
	r, err := NewTruncationRecovery(32768)
	if err != nil {
		t.Fatal(err)
	}

	// Stage 1: escalate ONCE, carrying the larger cap.
	first := r.Next()
	if first.Kind != RecoveryEscalate {
		t.Fatalf("the first response to truncation must be to retry with more room, got %v", first.Kind)
	}
	if first.MaxTokens != 32768 {
		t.Errorf("the escalated cap must reach the retry, got %d", first.MaxTokens)
	}

	// Stage 2: bounded resumes, numbered so the log can show progress.
	for want := 1; want <= maxResumeAttempts; want++ {
		step := r.Next()
		if step.Kind != RecoveryResume {
			t.Fatalf("attempt %d should resume, got %v", want, step.Kind)
		}
		if step.Attempt != want {
			t.Errorf("resume attempt = %d, want %d", step.Attempt, want)
		}
		if step.Prompt == "" {
			t.Error("a resume with no instruction just repeats the generation that was cut")
		}
	}

	// Then stop — a model that cannot finish in three resumes will not on the
	// fourth, and each one is a full generation, paid for.
	if step := r.Next(); step.Kind != RecoveryExhausted {
		t.Errorf("after %d resumes recovery must give up, got %v", maxResumeAttempts, step.Kind)
	}
	if step := r.Next(); step.Kind != RecoveryExhausted {
		t.Error("exhausted must stay exhausted")
	}
}

// Escalation happens once. A policy that escalated on every truncation would
// loop at the largest cap forever, which is the failure it exists to prevent.
func TestEscalationHappensOnlyOnce(t *testing.T) {
	r, _ := NewTruncationRecovery(32768)
	if r.Next().Kind != RecoveryEscalate {
		t.Fatal("precondition")
	}
	if step := r.Next(); step.Kind == RecoveryEscalate {
		t.Error("escalating twice would retry the largest cap forever")
	}
}

// Fail fast on a cap that cannot help: retrying at the same or a smaller limit
// burns a full generation to hit the same wall.
func TestNewTruncationRecoveryRejectsAUselessCap(t *testing.T) {
	for _, bad := range []int64{0, -1} {
		if _, err := NewTruncationRecovery(bad); err == nil {
			t.Errorf("cap %d must be rejected at construction", bad)
		}
	}
}

// The factory returns the INTERFACE, so the turn loop cannot reach past the
// policy into its counters.
func TestFactoryReturnsTheInterface(t *testing.T) {
	var _ TruncationRecovery = mustRecovery(t, 1024)
}

func mustRecovery(t *testing.T, cap int64) TruncationRecovery {
	t.Helper()
	r, err := NewTruncationRecovery(cap)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

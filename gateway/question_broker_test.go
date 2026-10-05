package gateway

import "testing"

func TestQuestionBroker(t *testing.T) {
	b := newQuestionBroker()

	ch, cancel := b.register("q1")
	defer cancel()

	// answer to an unknown id returns false, doesn't block.
	if b.answer("nope", "x") {
		t.Fatal("answer to unknown id should return false")
	}

	// answer to the pending id delivers to the channel and reports true.
	if !b.answer("q1", "Option A") {
		t.Fatal("answer to a pending question should return true")
	}
	select {
	case got := <-ch:
		if got != "Option A" {
			t.Fatalf("want %q, got %q", "Option A", got)
		}
	default:
		t.Fatal("expected the answer on the channel")
	}

	// A second answer to the same id is a no-op (already consumed).
	if b.answer("q1", "Option B") {
		t.Fatal("second answer to a consumed question should return false")
	}
}

func TestQuestionBrokerCancelCleansUp(t *testing.T) {
	b := newQuestionBroker()
	_, cancel := b.register("q2")
	cancel()
	if b.answer("q2", "x") {
		t.Fatal("answer after cancel should return false (cleaned up)")
	}
}

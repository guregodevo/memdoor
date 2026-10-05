package gateway

import (
	"strings"
	"testing"
)

// A full send buffer used to discard the event with a Debug line nobody reads.
// For a streaming turn each dropped event is a missing token in the rendered
// text — invisible until someone stares at a transcript with holes. The drop
// stays (blocking the broadcaster on one slow client would stall every other
// client), but it must be COUNTED and VISIBLE.
func TestAFullSendBufferCountsItsDrops(t *testing.T) {
	c := &Client{ID: "c1", send: make(chan []byte, 2)}
	if err := c.Send([]byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := c.Send([]byte("b")); err != nil {
		t.Fatal(err)
	}
	// Buffer full: these drop.
	for i := 0; i < 3; i++ {
		if err := c.Send([]byte("x")); err == nil {
			t.Fatal("a send into a full buffer reported success")
		}
	}
	if got := c.DroppedEvents(); got != 3 {
		t.Errorf("DroppedEvents() = %d, want 3", got)
	}
	if err := c.Send([]byte("x")); err == nil || !strings.Contains(err.Error(), "dropped") {
		t.Errorf("the error does not say events were dropped: %v", err)
	}
}

func TestAHealthySendCountsNothing(t *testing.T) {
	c := &Client{ID: "c2", send: make(chan []byte, 8)}
	for i := 0; i < 5; i++ {
		if err := c.Send([]byte("ok")); err != nil {
			t.Fatal(err)
		}
	}
	if got := c.DroppedEvents(); got != 0 {
		t.Errorf("DroppedEvents() = %d after healthy sends, want 0", got)
	}
}

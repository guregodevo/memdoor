package gateway

import (
	"context"
	"encoding/json"
	"testing"
)

// A streamed command's output comes back in the order the command wrote it,
// stdout and stderr interleaved as on a terminal. Two pipes with a copying
// goroutine each made the order a race: on the public CI runner bash's
// "command not found" arrived before the "start" echoed ahead of it, and
// the hint that reads that message missed it (2026-10-07).
func TestStreamedBashKeepsStdoutAndStderrInOrder(t *testing.T) {
	in, _ := json.Marshal(map[string]string{"command": "echo a; echo b >&2; echo c; echo d >&2"})
	for i := 0; i < 200; i++ {
		out, err := bashStreamer{}.Stream(context.Background(), in, nil)
		if err != nil {
			t.Fatal(err)
		}
		if out != "a\nb\nc\nd\n" {
			t.Fatalf("run %d: output out of order: %q", i, out)
		}
	}
}

package cmd

import (
	"os"
	"testing"
	"time"
)

func TestPipedStdinOpenButSilentDoesNotBlock(t *testing.T) {
	r, w, _ := os.Pipe()
	defer w.Close() // held open for the whole test, like an inherited pipe
	start := time.Now()
	if got := pipedStdin(r, 200*time.Millisecond); got != "" {
		t.Fatalf("got %q", got)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("blocked %s", d)
	}
}

func TestPipedStdinReadsContent(t *testing.T) {
	r, w, _ := os.Pipe()
	go func() {
		w.Write([]byte("line one\n"))
		time.Sleep(300 * time.Millisecond) // slower than the grace, after the start
		w.Write([]byte("line two\n"))
		w.Close()
	}()
	if got := pipedStdin(r, 100*time.Millisecond); got != "line one\nline two" {
		t.Fatalf("got %q", got)
	}
}

func TestPipedStdinEmpty(t *testing.T) {
	r, w, _ := os.Pipe()
	w.Close()
	if got := pipedStdin(r, time.Second); got != "" {
		t.Fatalf("got %q", got)
	}
}

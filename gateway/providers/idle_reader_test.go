package providers

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// A dead TCP stream blocks Read forever — no FIN arrives when a remote vLLM
// is killed mid-stream — and the only bound was the whole-request 20-minute
// client timeout. Measured live 2026-08-31 12:24: the machine restarted and
// answered fresh requests within seconds, while the wedged turn sat silent
// for its remaining quarter hour. An idle deadline is the right grain: bytes
// reset it, silence past the limit fails the stream so the retry machinery
// can act while the machine is back.
func TestASilentStreamFailsAtTheIdleDeadline(t *testing.T) {
	pr, _ := io.Pipe() // a reader that never delivers and never closes
	r := newIdleTimeoutReader(pr, 80*time.Millisecond)
	defer r.Close()

	start := time.Now()
	_, err := r.Read(make([]byte, 64))
	if err == nil {
		t.Fatal("a permanently silent stream returned no error")
	}
	if !errors.Is(err, errStreamIdle) {
		t.Errorf("error is %v, want errStreamIdle so the caller can name the cause", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v to give up — the deadline did not fire", elapsed)
	}
}

// Bytes reset the deadline: a slow-but-alive stream is a long generation, not
// a hang, and must never be cut.
func TestAliveBytesKeepResettingTheDeadline(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		for i := 0; i < 6; i++ {
			time.Sleep(50 * time.Millisecond) // under the 80ms idle limit, repeatedly
			_, _ = pw.Write([]byte("tok "))
		}
		pw.Close()
	}()
	r := newIdleTimeoutReader(pr, 80*time.Millisecond)
	defer r.Close()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("an alive stream was cut: %v (read %q)", err, got)
	}
	if want := strings.Repeat("tok ", 6); string(got) != want {
		t.Errorf("read %q, want %q", got, want)
	}
}

// then silence forever — no close, no FIN

// Before the first byte the longer limit rules; after it, the idle one.
func TestIdleReaderWaitsLongerForTheFirstByte(t *testing.T) {
	pr, pw := io.Pipe()
	r := newIdleTimeoutReaderFirst(pr, 300*time.Millisecond, 80*time.Millisecond)
	go func() {
		time.Sleep(150 * time.Millisecond) // past the idle limit, within the first-byte one
		pw.Write([]byte("a"))
		time.Sleep(200 * time.Millisecond) // past the idle limit, with a byte already seen
		pw.Write([]byte("b"))
		pw.Close()
	}()
	buf := make([]byte, 8)
	if n, err := r.Read(buf); err != nil || n != 1 {
		t.Fatalf("the first byte must arrive within the first-byte limit: n=%d err=%v", n, err)
	}
	if _, err := r.Read(buf); !errors.Is(err, errStreamIdle) {
		t.Fatalf("after the first byte the idle limit rules, got %v", err)
	}
}

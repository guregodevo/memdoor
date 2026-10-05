package providers

import (
	"errors"
	"fmt"
	"io"
	"time"
)

// errStreamIdle names the cause when a stream is cut for silence, so the
// caller's error report says "the stream went quiet" and not "read failed".
var errStreamIdle = errors.New("stream idle past deadline")

// idleTimeoutReader fails a Read that sits with NO bytes for longer than the
// idle limit. Bytes reset the clock.
//
// A dead TCP stream blocks Read forever — when an upstream dies
// mid-stream no FIN arrives — and the only bound was the whole-request
// 20-minute client timeout. Measured live 2026-08-31 12:24: the upstream
// restarted and answered fresh requests within seconds, while the wedged
// turn sat silent for its remaining quarter hour. Idle is the right
// grain for a token stream: a slow generation still delivers SOMETHING every
// few seconds, so minutes of true silence mean the connection, not the model.
type idleTimeoutReader struct {
	r     io.Reader
	c     io.Closer // closes the underlying body to unblock the pending Read
	idle  time.Duration
	timer *time.Timer
	fired chan struct{}
}

func newIdleTimeoutReader(r io.Reader, idle time.Duration) *idleTimeoutReader {
	return newIdleTimeoutReaderFirst(r, idle, idle)
}

// newIdleTimeoutReaderFirst allows `first` before the first byte and
// `idle` between bytes after that: a queued request is not a dead one.
func newIdleTimeoutReaderFirst(r io.Reader, first, idle time.Duration) *idleTimeoutReader {
	it := &idleTimeoutReader{r: r, idle: idle, fired: make(chan struct{})}
	if c, ok := r.(io.Closer); ok {
		it.c = c
	}
	it.timer = time.AfterFunc(first, func() {
		close(it.fired)
		if it.c != nil {
			_ = it.c.Close() // unblocks the Read stuck on the dead connection
		}
	})
	return it
}

func (it *idleTimeoutReader) Read(p []byte) (int, error) {
	n, err := it.r.Read(p)
	select {
	case <-it.fired:
		return n, fmt.Errorf("%w (no bytes for %s)", errStreamIdle, it.idle)
	default:
	}
	if n > 0 {
		it.timer.Reset(it.idle)
	}
	return n, err
}

func (it *idleTimeoutReader) Close() error {
	it.timer.Stop()
	if it.c != nil {
		return it.c.Close()
	}
	return nil
}

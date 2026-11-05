package providers

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// errStreamIdle names the cause when a stream is cut for silence, so the
// caller's error report says "the stream went quiet" and not "read failed".
var errStreamIdle = errors.New("stream idle past deadline")

// errKeepAliveOnly is the same cut when the stream sent only keep-alive
// comments for keepAliveOnlyAfter: errors.Is(err, errStreamIdle) holds too.
var errKeepAliveOnly = fmt.Errorf("%w: only keep-alive comments", errStreamIdle)

// keepAliveOnlyAfter bounds a stream that sends only SSE comments. OpenRouter
// keeps a waiting request open with ": OPENROUTER PROCESSING" lines, while a
// model thinks and also when the request is stalled upstream: an ideator's
// call sat over ten minutes on them while its twin answered in 15 s
// (2026-11-05, #99), and a CI review was cut at the action's 10 minutes. So
// comments keep a stream alive only this long after its last content.
var keepAliveOnlyAfter = 5 * time.Minute

// idleTimeoutReader fails a Read that sits with no content for longer than
// the idle limit. Content (any byte outside an SSE comment line) resets the
// clock to the idle limit; a comment line (": keep-alive") resets it too, but
// never past keepAliveOnlyAfter since the last content.
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
	quiet time.Duration // how long comments alone keep it open (keepAliveOnlyAfter)
	timer *time.Timer
	fired chan struct{}
	cut   sync.Once // the timer may fire again after a Reset: the stream is cut once

	lastContent time.Time // the start, then the last byte of content
	lineStart   bool      // the next byte starts a line
	inComment   bool      // the current line started with ':'
}

func newIdleTimeoutReader(r io.Reader, idle time.Duration) *idleTimeoutReader {
	return newIdleTimeoutReaderFirst(r, idle, idle)
}

// newIdleTimeoutReaderFirst allows `first` before the first byte and
// `idle` between bytes after that: a queued request is not a dead one.
func newIdleTimeoutReaderFirst(r io.Reader, first, idle time.Duration) *idleTimeoutReader {
	it := &idleTimeoutReader{r: r, idle: idle, quiet: keepAliveOnlyAfter, fired: make(chan struct{}), lastContent: time.Now(), lineStart: true}
	if c, ok := r.(io.Closer); ok {
		it.c = c
	}
	it.timer = time.AfterFunc(first, func() {
		it.cut.Do(func() {
			close(it.fired)
			if it.c != nil {
				_ = it.c.Close() // unblocks the Read stuck on the dead connection
			}
		})
	})
	return it
}

func (it *idleTimeoutReader) Read(p []byte) (int, error) {
	n, err := it.r.Read(p)
	select {
	case <-it.fired:
		if quiet := time.Since(it.lastContent); quiet >= it.quiet {
			return n, fmt.Errorf("%w for %s", errKeepAliveOnly, quiet.Round(time.Second))
		}
		return n, fmt.Errorf("%w (no bytes for %s)", errStreamIdle, it.idle)
	default:
	}
	if n > 0 {
		if it.content(p[:n]) {
			it.lastContent = time.Now()
			it.timer.Reset(it.idle)
		} else if left := it.quiet - time.Since(it.lastContent); left > 0 {
			it.timer.Reset(min(it.idle, left))
		} else {
			it.timer.Reset(0) // comments only, for too long: cut now
		}
	}
	return n, err
}

// content reports whether b holds a byte outside an SSE comment line and
// outside the line breaks. The line state carries across Reads, so a comment
// split over two reads stays a comment.
func (it *idleTimeoutReader) content(b []byte) bool {
	found := false
	for _, c := range b {
		switch {
		case c == '\n':
			it.lineStart, it.inComment = true, false
		case c == '\r':
		case it.lineStart && c == ':':
			it.lineStart, it.inComment = false, true
		default:
			it.lineStart = false
			if !it.inComment {
				found = true
			}
		}
	}
	return found
}

func (it *idleTimeoutReader) Close() error {
	it.timer.Stop()
	if it.c != nil {
		return it.c.Close()
	}
	return nil
}

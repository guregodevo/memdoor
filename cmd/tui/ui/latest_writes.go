package ui

import (
	"math"
	"sync"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

// latestWrites keeps a stream of state writes in order. Bubble Tea runs each
// command on its own goroutine, so an earlier "working" can reach the
// terminal, or cmux, after a later "done". Each write gets a number when it is
// made; one older than the last written is dropped. After close nothing lands.
type latestWrites struct {
	made    atomic.Uint64
	mu      sync.Mutex
	written uint64
}

// cmd makes a write; the number is taken now, on the UI loop.
func (l *latestWrites) cmd(write func()) tea.Cmd {
	n := l.made.Add(1)
	return func() tea.Msg {
		l.mu.Lock()
		defer l.mu.Unlock()
		if n < l.written {
			return nil
		}
		l.written = n
		write()
		return nil
	}
}

// close runs the last write, told whether anything was written before, and
// drops every write still queued.
func (l *latestWrites) close(last func(sent bool)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	sent := l.written > 0 && l.written < math.MaxUint64
	l.written = math.MaxUint64
	last(sent)
}

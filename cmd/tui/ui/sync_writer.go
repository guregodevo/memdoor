package ui

import (
	"io"
	"os"
)

// syncWriter brackets every frame in DEC mode 2026 ("synchronized output"), so
// the terminal buffers the whole repaint and applies it atomically.
//
// Bubble Tea's standard renderer emits exactly one Write per frame, which
// makes this the whole fix for streaming tear: without it a terminal may paint
// a frame while its bytes are still arriving, and at ~45 repaints/s that is
// visible tearing — a drag-copy taken mid-repaint captures rows from two
// different frames ("goog le_news.py", "</t hi nk>", measured live
// 2026-08-30). Terminals without mode 2026 ignore both sequences.
//
// It wraps the terminal FILE, not just its write stream. Bubble Tea decides
// whether it can query the window size with one type assertion on its output —
// io.ReadWriteCloser plus Fd() (charmbracelet/x/term.File, tty_unix.go:25) —
// and the first version of this implemented only Write. The assertion failed,
// ttyOutput stayed nil, no WindowSizeMsg was ever delivered, ready never
// flipped, and the TUI sat on "Initializing..." forever (live 2026-08-31).
// Everything except Write forwards untouched; only frames get the bracket.
type syncWriter struct{ tty *os.File }

// NewSyncWriter wraps the terminal the TUI program renders to.
func NewSyncWriter(tty *os.File) io.Writer { return syncWriter{tty: tty} }

const (
	beginSync = "\x1b[?2026h"
	endSync   = "\x1b[?2026l"
)

func (w syncWriter) Write(p []byte) (int, error) { return bracketedWrite(w.tty, p) }
func (w syncWriter) Read(p []byte) (int, error)  { return w.tty.Read(p) }
func (w syncWriter) Close() error                { return w.tty.Close() }
func (w syncWriter) Fd() uintptr                 { return w.tty.Fd() }

// bracketedWrite sends begin-mark + frame + end-mark as ONE underlying write:
// three writes would let the terminal read a frame with its bracket split
// across reads, which is the tear this exists to prevent.
func bracketedWrite(out io.Writer, p []byte) (int, error) {
	buf := make([]byte, 0, len(p)+len(beginSync)+len(endSync))
	buf = append(buf, beginSync...)
	buf = append(buf, p...)
	buf = append(buf, endSync...)
	if _, err := out.Write(buf); err != nil {
		return 0, err
	}
	// The renderer checks n against len(p); the bracket bytes are ours, not its.
	return len(p), nil
}

package cmd

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"sync"
	"time"

	"memdoor/cmd/tui/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// REMOTE CONTROL's terminal view: the phone gets the TUI itself, not a
// re-drawing of it. A second bubbletea program runs in this process on the
// same conversation (its own /ws, like a second TUI window), reads the keys
// the page's xterm.js sends and writes its screen back, sealed like every
// other frame. The page asks for it with {"tty":{cols,rows,fresh}}, types
// with {"tty_in":…}; the screen comes back on the "tty" stream.
//
// ONE SCREEN FOR EVERY PAGE (2026-09-29, view-only links and several
// guests). The relay hands the screen to every page on the session, so there
// is one program: a driving page that opens starts it if none runs and
// otherwise resizes it — it never restarts one another page is using. A page
// that opens late asks with {"watch":<its id>} and gets what the program has
// drawn so far in one frame addressed to it, so its screen is the others'.

const (
	remoteTTYStream = "tty"
	// remoteTTYFlush batches the renderer's writes: the relay drops a frame
	// the phone is too slow for, and fewer, larger frames rarely hit that.
	remoteTTYFlush    = 40 * time.Millisecond
	remoteTTYFrameMax = 64 << 10
	remoteTTYKeysMax  = 1024
	remoteTTYMinCols  = 20
	remoteTTYMaxCols  = 300
	remoteTTYMinRows  = 5
	remoteTTYMaxRows  = 200
	// remoteTTYHistoryMax is how much of the screen's output a late page is
	// replayed. Past it the oldest bytes go, and the replay is followed by a
	// repaint so the page still ends on the right screen.
	remoteTTYHistoryMax = 1 << 20
)

// remoteTTYSize is a driving page's terminal: its size, and Fresh when the
// page asks for a program after the last one ended ("open it again"). A
// running program is resized, never replaced: other pages may be using it.
type remoteTTYSize struct {
	Cols  int  `json:"cols"`
	Rows  int  `json:"rows"`
	Fresh bool `json:"fresh,omitempty"`
}

func (s remoteTTYSize) clamped() tea.WindowSizeMsg {
	return tea.WindowSizeMsg{
		Width:  min(max(s.Cols, remoteTTYMinCols), remoteTTYMaxCols),
		Height: min(max(s.Rows, remoteTTYMinRows), remoteTTYMaxRows),
	}
}

// remoteTTY is one running phone program.
type remoteTTY struct {
	p  *tea.Program
	in *io.PipeWriter
	// keys keeps what was typed in the order it was typed: one writer
	// drains it into the program's input.
	keys chan string
	stop chan struct{}
	once sync.Once

	// hmu orders the screen: a frame is added to history and queued under
	// it, and a replay is taken and queued under it, so a late page's replay
	// and the live frames after it never overlap or leave a gap.
	hmu       sync.Mutex
	history   []byte
	truncated bool
	size      tea.WindowSizeMsg
}

// feed hands the program its input, in order, until the program ends.
func (t *remoteTTY) feed() {
	for {
		select {
		case k := <-t.keys:
			if _, err := io.WriteString(t.in, k); err != nil {
				return
			}
		case <-t.stop:
			return
		}
	}
}

func (t *remoteTTY) end() {
	t.once.Do(func() {
		close(t.stop)
		_ = t.in.Close()
		t.p.Kill()
	})
}

// remoteTTYEvent is one "tty" frame: screen bytes, or the program's end.
func remoteTTYEvent(out []byte, ended bool) ([]byte, error) {
	data := map[string]any{}
	if len(out) > 0 {
		data["out"] = base64.StdEncoding.EncodeToString(out)
	}
	if ended {
		data["ended"] = true
	}
	return json.Marshal(map[string]any{"stream": remoteTTYStream, "data": data})
}

// remoteTTYSizeEvent tells every page the program's size: a page that did
// not set it (a view-only page, a second phone) draws at the program's size.
func remoteTTYSizeEvent(size tea.WindowSizeMsg) ([]byte, error) {
	return json.Marshal(map[string]any{"stream": remoteTTYStream, "data": map[string]any{
		"cols": size.Width, "rows": size.Height,
	}})
}

// remoteTTYReplayEvent is the screen so far, for the page whose id is for.
// none: there is no program yet.
func remoteTTYReplayEvent(forID string, history []byte, size tea.WindowSizeMsg, none bool) ([]byte, error) {
	data := map[string]any{"for": forID}
	if none {
		data["none"] = true
	} else {
		data["replay"] = base64.StdEncoding.EncodeToString(history)
		data["cols"], data["rows"] = size.Width, size.Height
	}
	return json.Marshal(map[string]any{"stream": remoteTTYStream, "data": data})
}

// record adds a flushed frame to the history and queues it, under hmu.
func (t *remoteTTY) record(r *remoteRelay, out []byte) {
	b, err := remoteTTYEvent(out, false)
	if err != nil {
		return
	}
	t.hmu.Lock()
	defer t.hmu.Unlock()
	t.history = append(t.history, out...)
	if over := len(t.history) - remoteTTYHistoryMax; over > 0 {
		t.history = append(t.history[:0:0], t.history[over:]...)
		t.truncated = true
	}
	r.queueTTY(b, t.stop)
}

// resize applies a driving page's size and tells every page, under hmu.
func (t *remoteTTY) resize(r *remoteRelay, size tea.WindowSizeMsg) {
	t.hmu.Lock()
	defer t.hmu.Unlock()
	if size == t.size {
		return
	}
	t.size = size
	go t.p.Send(size)
	if b, err := remoteTTYSizeEvent(size); err == nil {
		r.queueTTY(b, t.stop)
	}
}

// ttyReplay sends the page that asked (forID) the screen so far.
func (r *remoteRelay) ttyReplay(forID string) {
	r.ttyMu.Lock()
	t := r.tty
	r.ttyMu.Unlock()
	if t == nil {
		if b, err := remoteTTYReplayEvent(forID, nil, tea.WindowSizeMsg{}, true); err == nil {
			r.queueTTY(b, r.closed)
		}
		return
	}
	t.hmu.Lock()
	b, err := remoteTTYReplayEvent(forID, t.history, t.size, false)
	if err == nil {
		r.queueTTY(b, t.stop)
	}
	repaint := t.truncated
	size := t.size
	t.hmu.Unlock()
	if repaint {
		// The replay lost its beginning: a repaint puts the live screen right.
		go t.p.Send(size)
	}
}

// ttyScreen resizes the phone program, or starts one when none runs.
func (r *remoteRelay) ttyScreen(size remoteTTYSize) {
	if r.s.TTY == nil {
		return
	}
	r.ttyMu.Lock()
	defer r.ttyMu.Unlock()
	if r.tty != nil {
		r.tty.resize(r, size.clamped())
		return
	}
	pr, pw := io.Pipe()
	t := &remoteTTY{in: pw, keys: make(chan string, remoteTTYKeysMax), stop: make(chan struct{}), size: size.clamped()}
	go t.feed()
	out := newRemoteTTYWriter(r, t)
	t.p = tea.NewProgram(r.s.TTY(),
		tea.WithInput(pr),
		// No terminal behind it: the size comes from the page (WindowSizeMsg
		// below), and the writer already sends whole frames.
		tea.WithOutput(out),
		// The process's signals belong to the TUI on the Mac's terminal.
		tea.WithoutSignalHandler(),
	)
	r.tty = t
	go t.p.Send(ui.SetProgramMsg{Program: t.p})
	go t.p.Send(size.clamped())
	if b, err := remoteTTYSizeEvent(size.clamped()); err == nil {
		go r.queueTTY(b, t.stop)
	}
	go func() {
		_, _ = t.p.Run()
		out.flush()
		select {
		case <-t.stop: // closed: /remote off, or the TUI quit
		default: // quit on the phone: say so, the page offers a restart
			if b, err := remoteTTYEvent(nil, true); err == nil {
				r.queueTTY(b, t.stop)
			}
		}
		r.ttyMu.Lock()
		if r.tty == t {
			r.tty = nil
		}
		r.ttyMu.Unlock()
		t.end()
	}()
}

// ttyType hands the page's keys to the phone program.
func (r *remoteRelay) ttyType(keys string) {
	r.ttyMu.Lock()
	t := r.tty
	r.ttyMu.Unlock()
	if t == nil {
		return
	}
	select {
	case t.keys <- keys:
	case <-t.stop:
	default: // the program is not reading: drop the key rather than stall the relay's reader
	}
}

func (r *remoteRelay) stopTTY() {
	r.ttyMu.Lock()
	defer r.ttyMu.Unlock()
	if r.tty != nil {
		r.tty.end()
		r.tty = nil
	}
}

// queueTTY waits for room: a dropped screen frame corrupts the phone's
// display, unlike a dropped chat delta.
func (r *remoteRelay) queueTTY(b []byte, stop <-chan struct{}) {
	select {
	case r.ttyOut <- b:
	case <-stop:
	case <-r.closed:
	}
}

// remoteTTYWriter collects the renderer's writes and sends them as one
// frame per flush interval.
type remoteTTYWriter struct {
	r     *remoteRelay
	t     *remoteTTY
	stop  <-chan struct{}
	mu    sync.Mutex
	buf   []byte
	timer *time.Timer
}

func newRemoteTTYWriter(r *remoteRelay, t *remoteTTY) *remoteTTYWriter {
	return &remoteTTYWriter{r: r, t: t, stop: t.stop}
}

func (w *remoteTTYWriter) Write(b []byte) (int, error) {
	select {
	case <-w.stop:
		return 0, io.ErrClosedPipe
	default:
	}
	w.mu.Lock()
	w.buf = append(w.buf, b...)
	full := len(w.buf) >= remoteTTYFrameMax
	if !full && w.timer == nil {
		w.timer = time.AfterFunc(remoteTTYFlush, w.flush)
	}
	w.mu.Unlock()
	if full {
		w.flush()
	}
	return len(b), nil
}

func (w *remoteTTYWriter) flush() {
	w.mu.Lock()
	out := w.buf
	w.buf = nil
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	w.mu.Unlock()
	if len(out) == 0 {
		return
	}
	w.t.record(w.r, out)
}

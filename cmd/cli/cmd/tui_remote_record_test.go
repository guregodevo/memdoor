package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// The landing page's recording (web/public/remote.cast, played by
// web/src/components/RemoteDemo.tsx) is made here: this joins a live
// conversation as its browser, asks for a phone-sized terminal, types a
// question a key at a time and writes every screen frame the page would draw
// to an asciinema v2 cast. Skipped unless asked for:
//
//	cd /tmp/memdoor && MEMDOOR_REMOTE_URL=http://localhost:18789 \
//	    MEMDOOR_BILLING_TOKEN=<local gateway token> memdoor tui   # then /remote
//	MEMDOOR_REMOTE_URL=http://localhost:18789 REC_ID=<the link's id> \
//	    REC_OUT=web/public/remote.cast REC_Q="<the question>" \
//	    go test ./cmd/cli/cmd/ -run TestRecordRemoteCast -v
func TestRecordRemoteCast(t *testing.T) {
	id, out, question := os.Getenv("REC_ID"), os.Getenv("REC_OUT"), os.Getenv("REC_Q")
	if id == "" {
		t.Skip("set REC_ID, REC_OUT and REC_Q to record")
	}
	// A phone's size by default (the /remote recording); REC_COLS/REC_ROWS
	// for a laptop's, REC_UNTIL for what ends the take (the turn's end by
	// default; "■ " for a workflow run's end) and REC_WAIT for how long.
	cols, rows := envInt("REC_COLS", 50), envInt("REC_ROWS", 34)
	until := os.Getenv("REC_UNTIL")
	if until == "" {
		until = "answered by"
	}
	wait := time.Duration(envInt("REC_WAIT", 170)) * time.Second
	var l remoteLink
	for _, x := range loadRemoteLinks() {
		if x.ID == id {
			l = x
		}
	}
	if l.Key == "" {
		t.Fatalf("no link with id %q on this machine: run /remote in the conversation first", id)
	}
	proof, _ := remoteBrowserProof(l.Key)
	aead, _ := remoteAEAD(l.Key)
	relay := "ws" + strings.TrimPrefix(remoteURL(), "http") + "/api/relay/browser?"
	c, _, err := websocket.DefaultDialer.Dial(relay+url.Values{"id": {l.ID}, "auth": {proof}}.Encode(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	send := func(turn remoteRelayTurn) {
		b, _ := json.Marshal(turn)
		p, _ := remoteSeal(aead, remoteToTerminal, b)
		_ = c.WriteJSON(remoteRelayWire{Kind: "turn", Payload: p})
	}
	type frame struct {
		at time.Duration
		b  []byte
	}
	got := make(chan []byte, 1024)
	go func() {
		for {
			var w remoteRelayWire
			if c.ReadJSON(&w) != nil {
				close(got)
				return
			}
			p, err := remoteOpen(aead, remoteToBrowser, w.Payload)
			if err != nil {
				continue
			}
			var ev struct {
				Stream string
				Data   struct{ Out string }
			}
			_ = json.Unmarshal(p, &ev)
			if ev.Stream != "tty" || ev.Data.Out == "" {
				continue
			}
			b, _ := base64.StdEncoding.DecodeString(ev.Data.Out)
			got <- b
		}
	}()
	start := time.Now()
	var frames []frame
	var screen strings.Builder
	collect := func(d time.Duration, until string) bool {
		end := time.After(d)
		for {
			select {
			case b, ok := <-got:
				if !ok {
					return false
				}
				frames = append(frames, frame{time.Since(start), b})
				screen.Write(b)
				if until != "" && strings.Contains(screen.String(), until) {
					return true
				}
			case <-end:
				return false
			}
		}
	}
	send(remoteRelayTurn{TTY: &remoteTTYSize{Cols: cols, Rows: rows, Fresh: true}})
	collect(3*time.Second, "")
	for _, r := range question {
		send(remoteRelayTurn{TTYIn: string(r)})
		collect(55*time.Millisecond, "")
	}
	collect(600*time.Millisecond, "")
	entered := time.Since(start)
	send(remoteRelayTurn{TTYIn: "\r"})
	done := collect(wait, until)
	collect(2*time.Second, "")

	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	recorded := 0.0
	if len(frames) > 0 {
		recorded = frames[len(frames)-1].at.Seconds()
	}
	// The title keeps the real duration: the caption on the page quotes it.
	head, _ := json.Marshal(map[string]any{
		"version": 2, "width": cols, "height": rows, "timestamp": start.Unix(),
		"title": fmt.Sprintf("memdoor /remote, %.0f s recorded", recorded),
		"env":   map[string]string{"TERM": "xterm-256color"},
	})
	fmt.Fprintf(f, "%s\n", head)
	// Waits are fast-forwarded, never cut: every frame is kept, in order.
	// After 1.2 s of nothing but small repaints (the spinner), the rest of
	// that wait plays at 12x.
	played, prev, quiet := 0.0, 0.0, 0.0
	for _, fr := range frames {
		gap := fr.at.Seconds() - prev
		prev = fr.at.Seconds()
		if gap > 0.5 {
			gap = 0.5
		}
		if fr.at > entered && len(fr.b) < 400 {
			if quiet > 1.2 {
				gap /= 12
			}
			quiet += gap
		} else {
			quiet = 0
		}
		played += gap
		line, _ := json.Marshal([]any{played, "o", string(fr.b)})
		fmt.Fprintf(f, "%s\n", line)
	}
	fmt.Printf("finished=%v frames=%d recorded=%.0fs played=%.0fs\n", done, len(frames), prev, played)
}

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

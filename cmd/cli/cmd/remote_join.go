package cmd

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// `memdoor join <link>`: a /remote link opened in a terminal instead of a
// browser (roadmap MUST item 4, parity with omp's `omp join`). It is the page
// at /r/<id> without the page: the same relay, the same keys, the same shared
// screen and replay (tui_remote_tty.go). The guest's terminal is the screen
// and the keyboard; the HOST's machine runs the agent, in the host's folder —
// which is what `memdoor tui --gateway <url> --channel <id>` could not
// promise, since each TUI sends its own launch directory with a turn.
//
// A full link (#k=) drives; a watch-only link (#v=) watches, and its keys are
// not sent. Ctrl+] leaves, as in telnet: every other key belongs to the TUI.

// remoteJoinDetach leaves the session: Ctrl+], which no TUI key uses.
const remoteJoinDetach = 0x1d

// remoteJoinKeys are what a link gives a client of the relay.
type remoteJoinKeys struct {
	read  cipher.AEAD // opens the screen; seals what a watcher may send
	write cipher.AEAD // seals what a driver sends; nil on a watch-only link
	proof string      // joins the relay
}

// remoteJoinTarget is a parsed /remote link.
type remoteJoinTarget struct {
	base string // scheme://host of the relay
	id   string
	keys remoteJoinKeys
}

func (t remoteJoinTarget) canDrive() bool { return t.keys.write != nil }

// parseRemoteLink reads https://<host>/r/<id>#k=<key> or #v=<token>.
func parseRemoteLink(link string) (remoteJoinTarget, error) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || u.Host == "" {
		return remoteJoinTarget{}, errors.New("not a link: paste the whole https://…/r/… link /remote printed")
	}
	id, ok := strings.CutPrefix(u.Path, "/r/")
	if !ok || id == "" || strings.Contains(id, "/") {
		return remoteJoinTarget{}, errors.New("not a remote-control link: it has no /r/<id>")
	}
	frag, err := url.ParseQuery(u.Fragment)
	if err != nil {
		return remoteJoinTarget{}, fmt.Errorf("the part after # is damaged: %w", err)
	}
	t := remoteJoinTarget{base: u.Scheme + "://" + u.Host, id: id}
	switch {
	case frag.Get("k") != "":
		key := frag.Get("k")
		if t.keys.read, err = remoteAEAD(key); err != nil {
			return t, err
		}
		if t.keys.write, err = remoteWriteAEAD(key); err != nil {
			return t, err
		}
		t.keys.proof, err = remoteBrowserProof(key)
		return t, err
	case frag.Get("v") != "":
		raw, err := base64.RawURLEncoding.DecodeString(frag.Get("v"))
		if err != nil || len(raw) != 64 {
			return t, errors.New("the watch-only part after #v= is damaged: copy the link again")
		}
		block, err := aes.NewCipher(raw[:32])
		if err != nil {
			return t, err
		}
		if t.keys.read, err = cipher.NewGCM(block); err != nil {
			return t, err
		}
		t.keys.proof = base64.RawURLEncoding.EncodeToString(raw[32:])
		return t, nil
	}
	return t, errors.New("the link has no key after #: copy it exactly as /remote printed it")
}

// remoteJoin is one guest on a session.
type remoteJoin struct {
	target remoteJoinTarget
	conn   *websocket.Conn
	out    io.Writer
	size   func() (cols, rows int)

	wmu      sync.Mutex // one writer on the socket
	awaiting string     // the replay this guest waits for; live frames skip until then
}

// errRemoteRevoked: the host ran /remote off.
var errRemoteRevoked = errors.New("this link was turned off on the computer it belonged to")

// errRemoteEnded: the TUI on the host quit (^C twice).
var errRemoteEnded = errors.New("the session ended")

// joinRemote connects to the link's relay and runs until the guest leaves
// (Ctrl+] on in), the host's TUI ends, or the link is revoked. resized fires
// when the guest's terminal changes size (nil: never).
func joinRemote(link string, in io.Reader, out io.Writer, size func() (int, int), resized <-chan struct{}) error {
	target, err := parseRemoteLink(link)
	if err != nil {
		return err
	}
	ws := "ws" + strings.TrimPrefix(target.base, "http") + "/api/relay/browser?" +
		url.Values{"id": {target.id}, "auth": {target.keys.proof}}.Encode()
	conn, resp, err := websocket.DefaultDialer.Dial(ws, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return errors.New("nothing is serving this link: the host's TUI is closed, or the link was turned off")
		}
		return fmt.Errorf("could not reach %s: %w", target.base, err)
	}
	defer conn.Close()
	conn.SetReadLimit(remoteFrameMax)
	j := &remoteJoin{target: target, conn: conn, out: out, size: size}

	if target.canDrive() {
		cols, rows := size()
		if err := j.send(remoteRelayTurn{TTY: &remoteTTYSize{Cols: cols, Rows: rows, Fresh: true}}); err != nil {
			return err
		}
	}
	j.awaiting = randomHex(8)
	if err := j.send(remoteRelayTurn{Watch: j.awaiting}); err != nil {
		return err
	}

	done := make(chan error, 3)
	go func() { done <- j.readScreen() }()
	go func() { done <- j.readKeys(in) }()
	go func() {
		for range resized {
			if target.canDrive() {
				cols, rows := size()
				_ = j.send(remoteRelayTurn{TTY: &remoteTTYSize{Cols: cols, Rows: rows}})
			}
		}
	}()
	return <-done
}

// send seals a turn for the terminal: with the write key when this link has
// one, else with the session key (the terminal then only lets it watch).
func (j *remoteJoin) send(t remoteRelayTurn) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	key := j.target.keys.write
	if key == nil {
		key = j.target.keys.read
	}
	payload, err := remoteSeal(key, remoteToTerminal, b)
	if err != nil {
		return err
	}
	j.wmu.Lock()
	defer j.wmu.Unlock()
	_ = j.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return j.conn.WriteJSON(remoteRelayWire{Kind: "turn", Payload: payload})
}

// readKeys forwards what the guest types, until Ctrl+] (nil: they left).
func (j *remoteJoin) readKeys(in io.Reader) error {
	buf := make([]byte, 1024)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if i := strings.IndexByte(string(chunk), remoteJoinDetach); i >= 0 {
				if i > 0 && j.target.canDrive() {
					_ = j.send(remoteRelayTurn{TTYIn: string(chunk[:i])})
				}
				return nil
			}
			if j.target.canDrive() {
				if err := j.send(remoteRelayTurn{TTYIn: string(chunk)}); err != nil {
					return err
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				// No more input (a pipe): keep watching until the screen ends.
				select {}
			}
			return err
		}
	}
}

// readScreen writes the host's screen to out until the session ends.
func (j *remoteJoin) readScreen() error {
	for {
		_, raw, err := j.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, 4001) {
				return errRemoteRevoked
			}
			return fmt.Errorf("the connection to the relay closed: %w", err)
		}
		var wire remoteRelayWire
		if json.Unmarshal(raw, &wire) != nil || wire.Kind != "event" {
			continue
		}
		plain, err := remoteOpen(j.target.keys.read, remoteToBrowser, wire.Payload)
		if err != nil {
			continue
		}
		var ev struct {
			Stream string `json:"stream"`
			Data   struct {
				Out    string `json:"out"`
				Ended  bool   `json:"ended"`
				For    string `json:"for"`
				Replay string `json:"replay"`
				None   bool   `json:"none"`
			} `json:"data"`
		}
		if json.Unmarshal(plain, &ev) != nil || ev.Stream != remoteTTYStream {
			continue // the chat stream is for the page's reading view
		}
		d := ev.Data
		switch {
		case d.For != "":
			if d.For != j.awaiting {
				continue // another guest's replay
			}
			j.awaiting = ""
			// A clean screen, then what the others see.
			_, _ = io.WriteString(j.out, "\x1b[2J\x1b[H")
			if d.None {
				_, _ = io.WriteString(j.out, "Waiting for the terminal: it appears when someone opens the full link.\r\n")
				continue
			}
			if b, err := base64.StdEncoding.DecodeString(d.Replay); err == nil {
				_, _ = j.out.Write(b)
			}
		case j.awaiting != "":
			// Live frames before the replay are already in it.
		case d.Out != "":
			if b, err := base64.StdEncoding.DecodeString(d.Out); err == nil {
				_, _ = j.out.Write(b)
			}
		case d.Ended:
			return errRemoteEnded
		}
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var joinCmd = &cobra.Command{
	Use:   "join <link>",
	Short: "Open a /remote link in this terminal: the host's TUI, here (Ctrl+] leaves)",
	Long: `Open a link that /remote printed on another computer, in this terminal
instead of a browser. The host's computer runs the agent, in the host's
folder; this terminal is its screen and keyboard. A watch-only link (#v=)
shows the screen and sends nothing.

Ctrl+] leaves. Every other key goes to the TUI.`,
	Example: `  memdoor join 'https://memdoor.ai/r/0L17wXRvEC5IzW_D#k=…'`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target, err := parseRemoteLink(args[0])
		if err != nil {
			return err
		}
		mode := "you can type; Ctrl+] leaves"
		if !target.canDrive() {
			mode = "watching only; Ctrl+] leaves"
		}
		fmt.Fprintf(os.Stderr, "Joining %s — %s.\n", target.id, mode)

		size := func() (int, int) {
			if c, r, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
				return c, r
			}
			return 80, 24
		}
		// restore puts the guest's terminal back: out of raw mode, cursor
		// shown, attributes reset, on a fresh line below the host's screen.
		restore := func() {}
		if term.IsTerminal(int(os.Stdin.Fd())) {
			old, err := term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				return err
			}
			restore = func() {
				_ = term.Restore(int(os.Stdin.Fd()), old)
				fmt.Fprint(os.Stdout, "\x1b[?25h\x1b[0m\r\n")
			}
		}
		err = joinRemote(args[0], os.Stdin, os.Stdout, size, terminalResized())
		restore()
		switch {
		case err == nil:
			fmt.Fprintln(os.Stderr, "Left the session. The host's TUI keeps running.")
			return nil
		case errors.Is(err, errRemoteEnded):
			fmt.Fprintln(os.Stderr, "The host's TUI for this link ended.")
			return nil
		case errors.Is(err, errRemoteRevoked):
			// The host's choice, not the guest's error.
			fmt.Fprintln(os.Stderr, "This link was turned off on the computer it belonged to.")
			return nil
		}
		return err
	},
}

func init() {
	rootCmd.AddCommand(joinCmd)
}

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"memdoor/cmd/tui/ui"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// memdoor remote <id> — the phone's link for a conversation, from a script
// with no window (Greg, 2026-10-12: "there should be a remote command to
// return in stdout the link", "from a resume id"; 2026-10-24, once the
// gateway held the link: "memdoor remote does not make sense, we should use
// gateway instead", "we need to pass the --gateway <host> so memdoor
// --gateway <host> remote"). It hands the conversation's link to the gateway
// named by --gateway (this machine's by default), which hosts the phone's
// terminal (remote_host.go), prints the link on stdout, one line, and
// returns. Nothing here stays up: the link lives as long as that gateway, so
// a job that waits while a phone drives waits on its own gateway, not on
// this. `memdoor join`, a relay client in a terminal, went the same day:
// --gateway <box> is the terminal's door onto a box.
//
// Stdout is the link and nothing else. The watch-only link and the QR code
// go to stderr: a job's log shows the code, and a phone scans it off the
// screen (Greg: "or the qr code"). `off` revokes the link.
var remoteCmd = &cobra.Command{
	Use:   "remote [id | off <id>]",
	Short: "The link that opens a conversation on your phone, on stdout; the gateway holds it",
	Long: `memdoor remote <id>                    print the link on stdout; the gateway holds the conversation on it
memdoor remote --last                  the most recent conversation here
memdoor remote                         the list, and a question: which one
memdoor remote --view <id>             the watch-only link instead
memdoor remote off <id>                revoke the link
memdoor --gateway <box> remote <id>    the same, for a conversation on that box: the box holds the link

Without --gateway it is the gateway on this machine. The id is the one memdoor
run and memdoor resume print. The link is the one /remote gives in a window,
and it stays valid: the same one opens the
conversation again next time. The gateway holds it with nothing else open and
brings it back when it restarts, so a job that must wait while you drive from
the phone waits on its own gateway (memdoor gateway & … wait), not on this.`,
	Args:              cobra.MaximumNArgs(2),
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		last, _ := cmd.Flags().GetBool("last")
		view, _ := cmd.Flags().GetBool("view")
		off := len(args) >= 1 && args[0] == "off"
		if off {
			args = args[1:]
		}
		r, err := remoteTarget(args, last)
		if err != nil {
			return err
		}
		if err := firstRunReady(); err != nil {
			return err
		}
		c := NewClient()
		wsID := strings.TrimSpace(r.Workspace)
		if wsID == "" {
			wsID = strings.TrimSpace(workspaceSlug)
		}
		if wsID == "" {
			return fmt.Errorf("no workspace for this conversation — `memdoor workspace use <slug>` pins this directory")
		}
		// The phone's turns run in the conversation's own project, wherever
		// this command was typed.
		dir := r.Dir
		if dir == "" {
			dir, _ = os.Getwd()
		}
		curDir := func() string { return dir }
		poster := tuiPoster(c, wsID, curDir)
		w := newTUIWiring(c, wsID, wsURLOf(gatewayAddr), curDir, poster)
		remote := w.remoteFor(ui.FirstAgent(), r.ChannelID, r.ChannelName).resolved()
		arg := ""
		switch {
		case off:
			arg = "off"
		case view:
			arg = "view"
		}
		text, err := tuiRemote(remote)(arg)
		if err != nil {
			return err
		}
		if off {
			fmt.Fprintln(os.Stderr, text)
			return nil
		}
		if remoteHeldHere(r.ChannelID) {
			// The gateway could not take the link (an older binary): a
			// link held by a command that has just returned opens nothing.
			stopRemote(r.ChannelID)
			return errGatewayHoldsNoLink
		}
		link, _, err := remoteLinkFor(r.ChannelID, false)
		if err != nil {
			return err
		}
		base := remote.relayBase()
		out, watch := link.url(base), ""
		if watch, err = link.viewURL(base); err != nil {
			return err
		}
		if view {
			out = watch
		}
		fmt.Fprintln(os.Stdout, out)
		fmt.Fprintln(os.Stderr, remoteLinkText(r, watch, view, base))
		return nil
	},
}

var errGatewayHoldsNoLink = errors.New("this gateway does not hold links (it is older than this command): upgrade it, or type /remote in a window on it")

// remoteAsks reports whether a person is at the keyboard to pick from the
// list (a test answers for them).
var remoteAsks = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// remoteTarget is the conversation: an id (any unique prefix of the one a
// run printed, from any directory), --last for this directory's most recent
// (falling back to the most recent anywhere), or, at a terminal, the one
// picked from the list, as memdoor resume asks. The list and the question
// go to stderr: stdout stays the link.
func remoteTarget(args []string, last bool) (sessionRecord, error) {
	if len(args) == 1 {
		return findSessionByID(loadSessionIndex(), args[0])
	}
	cwd, _ := os.Getwd()
	recs := sessionsFor(cwd)
	if len(recs) == 0 {
		recs = sessionsFor("")
	}
	if len(recs) == 0 {
		return sessionRecord{}, fmt.Errorf("no conversation yet — `memdoor run \"…\"` or `memdoor tui` starts one")
	}
	switch {
	case last:
		return recs[0], nil
	case remoteAsks():
		r, ok, err := askSession(recs, true, "Which one on your phone?", os.Stderr)
		if err != nil {
			return sessionRecord{}, err
		}
		if !ok {
			return sessionRecord{}, fmt.Errorf("no conversation picked")
		}
		return r, nil
	}
	return sessionRecord{}, fmt.Errorf("which conversation? memdoor remote <id>, or --last")
}

// remoteLinkText is what stderr says beside the link: the watch-only link
// (unless that is the one printed), the QR code of the printed link, and
// who holds it.
func remoteLinkText(r sessionRecord, watch string, view bool, base string) string {
	var b strings.Builder
	title := strings.TrimSpace(r.Title)
	if title == "" {
		title = shortID(r.ChannelID)
	}
	if view {
		fmt.Fprintf(&b, "watch only · %q\n", title)
	} else {
		fmt.Fprintf(&b, "on your phone · %q\nwatch only: %s\n", title, watch)
	}
	link := watch
	if !view {
		if l, _, err := remoteLinkFor(r.ChannelID, false); err == nil {
			link = l.url(base)
		}
	}
	b.WriteString(qrText(link))
	b.WriteString("\n")
	b.WriteString("held by your gateway, on " + relayHostOf(base) + ": it works with nothing open here, and comes back when the gateway restarts")
	return b.String()
}

func init() {
	remoteCmd.Flags().Bool("last", false, "The most recent conversation in this directory")
	remoteCmd.Flags().Bool("view", false, "Print the watch-only link instead")
	rootCmd.AddCommand(remoteCmd)
}

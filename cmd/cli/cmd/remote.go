package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"memdoor/cmd/tui/ui"

	"github.com/spf13/cobra"
)

// memdoor remote <id> — the phone door with no window (Greg, 2026-10-12:
// "there should be a remote command to return in stdout the link", "from a
// resume id"). It prints the conversation's link on stdout, one line, and
// then holds the conversation on the relay until ctrl+c: a script or a CI
// job that started a run with `memdoor run` can hand
// it to a phone, and the job waits there while the phone drives. The page
// the phone opens is the TUI on that conversation, as /remote gives it.
//
// The hold ends on ctrl+c (or a term signal), and on that alone: nothing
// cancels the command's context on its own, so a closed pipeline leaves the
// process holding (proved 2026-10-12: `memdoor remote --last | head -1` kept
// running after the reader exited).
//
// Stdout is the link and nothing else. The watch-only link, the QR code and
// the hold line go to stderr: a job's log shows the code, and a phone scans
// it off the screen (Greg: "or the qr code"). `off` revokes the link.
var remoteCmd = &cobra.Command{
	Use:   "remote [id | off <id>]",
	Short: "A link that opens a conversation on your phone; printed, then held until ctrl+c",
	Long: `memdoor remote <id>        print the link on stdout and hold the conversation on the relay
memdoor remote --last      the most recent conversation here
memdoor remote --view <id> the watch-only link instead
memdoor remote off <id>    revoke the link

The id is the one memdoor run and memdoor resume print. The link stays valid
across holds: the same one opens the conversation again next time.`,
	Args: cobra.MaximumNArgs(2),
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
		handShellKeyToGateway(c)
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
		remote := w.remoteFor(ui.FirstAgent(), r.ChannelID, r.ChannelName)
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
		link, _, err := remoteLinkFor(r.ChannelID, false)
		if err != nil {
			return err
		}
		out, watch := link.url(), ""
		if watch, err = link.viewURL(); err != nil {
			return err
		}
		if view {
			out = watch
		}
		fmt.Fprintln(os.Stdout, out)
		fmt.Fprintln(os.Stderr, remoteHoldText(r, watch, view))
		return holdRemote(cmd.Context())
	},
}

// remoteTarget is the conversation: an id (any unique prefix of the one a
// run printed, from any directory), or --last for this directory's most
// recent, falling back to the most recent anywhere.
func remoteTarget(args []string, last bool) (sessionRecord, error) {
	switch {
	case len(args) == 1:
		return findSessionByID(loadSessionIndex(), args[0])
	case last:
		cwd, _ := os.Getwd()
		recs := sessionsFor(cwd)
		if len(recs) == 0 {
			recs = sessionsFor("")
		}
		if len(recs) == 0 {
			return sessionRecord{}, fmt.Errorf("no conversation yet — `memdoor run \"…\"` or `memdoor tui` starts one")
		}
		return recs[0], nil
	}
	return sessionRecord{}, fmt.Errorf("which conversation? memdoor remote <id>, or --last")
}

// remoteHoldText is what stderr says beside the link: the watch-only link
// (unless that is the one printed), the QR code of the printed link, and
// that the conversation is held.
func remoteHoldText(r sessionRecord, watch string, view bool) string {
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
			link = l.url()
		}
	}
	b.WriteString(qrText(link))
	b.WriteString("\n")
	b.WriteString("holding this conversation on the relay · ctrl+c ends it, the link stays")
	return b.String()
}

// holdRemote keeps the process, and so the relay, alive until a signal or
// the command's context ends.
func holdRemote(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	return nil
}

func init() {
	remoteCmd.Flags().Bool("last", false, "The most recent conversation in this directory")
	remoteCmd.Flags().Bool("view", false, "Print the watch-only link instead")
	rootCmd.AddCommand(remoteCmd)
}

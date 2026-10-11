package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"memdoor/pkg/shared"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Resuming a conversation.
//
// Every `memdoor tui` starts a FRESH channel: a busy channel's accumulated
// history poisons a small model, so a clean slate per launch is what keeps the
// coder reliable. The cost is that yesterday's conversation is unreachable
// unless you know its channel id — so the TUI records what it started, and
// `memdoor resume` hands it back.
//
// The index is local (~/.memdoor/sessions.json) and written by the TUI itself:
// the gateway already stores the transcript, but only the client knows which
// directory you launched in and what you first asked, which are the two things
// that let you recognise a session later.

type sessionRecord struct {
	ChannelID   string    `json:"channel_id"`
	ChannelName string    `json:"channel_name"`
	Title       string    `json:"title"`
	Workspace   string    `json:"workspace"`
	Dir         string    `json:"dir"`
	Turns       int       `json:"turns"`
	StartedAt   time.Time `json:"started_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func sessionsIndexPath() string {
	return shared.MemdoorHome("sessions.json")
}

func loadSessionIndex() []sessionRecord {
	path := sessionsIndexPath()
	if path == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var recs []sessionRecord
	_ = json.Unmarshal(b, &recs)
	return recs
}

// recordSession notes (or updates) a conversation in the local index. Called by
// the TUI on every turn: the first one names the session, the rest keep it at
// the top of the list. Failures are silent — losing the index must never break
// a turn.
func recordSession(rec sessionRecord) {
	path := sessionsIndexPath()
	if path == "" || rec.ChannelID == "" {
		return
	}
	recs := loadSessionIndex()
	now := time.Now()
	found := false
	for i := range recs {
		if recs[i].ChannelID == rec.ChannelID {
			recs[i].UpdatedAt = now
			recs[i].Turns++
			if recs[i].Title == "" {
				recs[i].Title = rec.Title
			}
			found = true
			break
		}
	}
	if !found {
		rec.StartedAt, rec.UpdatedAt, rec.Turns = now, now, 1
		recs = append(recs, rec)
	}
	// Keep the file bounded: a hundred conversations is more than anyone
	// scrolls, and this is a convenience index, not a record of truth.
	sort.Slice(recs, func(i, j int) bool { return recs[i].UpdatedAt.After(recs[j].UpdatedAt) })
	if len(recs) > 100 {
		recs = recs[:100]
	}
	if b, err := json.MarshalIndent(recs, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, b, 0o600)
	}
}

// sessionsFor returns the index, most recent first, optionally limited to
// conversations started in dir — the ones about the project you are in.
func sessionsFor(dir string) []sessionRecord {
	recs := loadSessionIndex()
	sort.Slice(recs, func(i, j int) bool { return recs[i].UpdatedAt.After(recs[j].UpdatedAt) })
	if dir == "" {
		return recs
	}
	var here []sessionRecord
	for _, r := range recs {
		if r.Dir == dir {
			here = append(here, r)
		}
	}
	return here
}

func humanSince(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

var resumeCmd = &cobra.Command{
	Use:   "resume [n | id]",
	Short: "Resume a previous conversation (picks up its history where you left off)",
	Long: `Resume a previous conversation.

With no argument it lists recent conversations and asks which one. Pass a
number to take one straight from that list, the id the window printed when
you left it (memdoor resume 8488c86b — any unique prefix of the id works,
from any directory), or --last for the most recent.

By default it lists conversations started in this directory; --all shows
every one.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		last, _ := cmd.Flags().GetBool("last")

		dir := ""
		if !all {
			dir, _ = os.Getwd()
		}
		recs := sessionsFor(dir)
		if len(recs) == 0 {
			if !all {
				return fmt.Errorf("no conversations recorded for this directory yet — `memdoor resume --all` to see every one, or `memdoor tui` to start")
			}
			return fmt.Errorf("no conversations recorded yet — `memdoor tui` starts one")
		}

		pick := -1
		switch {
		case last:
			pick = 0
		case len(args) == 1:
			n, err := strconv.Atoi(args[0])
			if err == nil && n >= 1 && n <= len(recs) {
				pick = n - 1
				break
			}
			// Not a row: the id the window printed at exit. An id names one
			// conversation wherever it was started, so the whole index is
			// searched, not this directory's slice.
			r, ferr := findSessionByID(loadSessionIndex(), args[0])
			if ferr != nil {
				return ferr
			}
			recs, pick = []sessionRecord{r}, 0
		default:
			if headless, _ := cmd.Flags().GetBool("headless"); headless {
				printSessions(recs, all) // a pipe gets the list, never a question
				return nil
			}
			printSessions(recs, all)
			fmt.Print("\nResume which? (number or id, Enter to cancel): ")
			line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			line = strings.TrimSpace(line)
			if line == "" {
				return nil
			}
			n, err := strconv.Atoi(line)
			if err == nil && n >= 1 && n <= len(recs) {
				pick = n - 1
				break
			}
			r, ferr := findSessionByID(recs, line)
			if ferr != nil {
				return ferr
			}
			recs, pick = []sessionRecord{r}, 0
		}

		r := recs[pick]
		if headless, _ := cmd.Flags().GetBool("headless"); headless {
			follow, _ := cmd.Flags().GetBool("follow")
			limit, _ := cmd.Flags().GetInt("limit")
			return resumeHeadless(r, limit, follow)
		}
		fmt.Printf("Resuming %q (%s, %d turns)\n", r.Title, humanSince(r.UpdatedAt), r.Turns)
		return runTUI(tuiOptions{
			resumeChannel:   r.ChannelName,
			resumeChannelID: r.ChannelID,
			resumeTitle:     r.Title,
			workspace:       r.Workspace,
		})
	},
}

// shortIDLen is how much of a conversation's id the window prints at exit
// and the list shows: enough to be unique among a hundred, short enough to
// type. `memdoor resume <prefix>` takes any unique prefix.
const shortIDLen = 8

func shortID(channelID string) string {
	if len(channelID) > shortIDLen {
		return channelID[:shortIDLen]
	}
	return channelID
}

// findSessionByID resolves the id a person typed after `memdoor resume`:
// the full channel id, or a prefix of it (at least 4 characters) that
// matches exactly one recorded conversation.
func findSessionByID(recs []sessionRecord, id string) (sessionRecord, error) {
	id = strings.TrimSpace(id)
	if len(id) < 4 {
		return sessionRecord{}, fmt.Errorf("%q is not a row number or a conversation id — `memdoor conversations` lists both", id)
	}
	var hits []sessionRecord
	for _, r := range recs {
		if r.ChannelID == id {
			return r, nil
		}
		if strings.HasPrefix(r.ChannelID, id) {
			hits = append(hits, r)
		}
	}
	switch len(hits) {
	case 0:
		return sessionRecord{}, fmt.Errorf("no conversation with id %s — `memdoor conversations --all` lists every one", id)
	case 1:
		return hits[0], nil
	}
	return sessionRecord{}, fmt.Errorf("%d conversations start with %s — give more of the id", len(hits), id)
}

// resumeLine is what the window prints as it closes: the command that
// brings this conversation back, or a plain goodbye when nothing was said
// in it (a record exists only once a turn was sent, so there is nothing
// to resume).
func resumeLine(channelID string) string {
	for _, r := range loadSessionIndex() {
		if r.ChannelID == channelID && r.Turns > 0 {
			return "Resume this conversation with: memdoor resume " + shortID(channelID)
		}
	}
	return "Goodbye!"
}

func printSessions(recs []sessionRecord, showDir bool) {
	fmt.Println("Recent conversations:")
	fmt.Println()
	fmt.Printf("      %-*s  %-56s  %-13s  %s\n", shortIDLen, "ID", "TITLE", "STARTED", "LAST USED")
	for i, r := range recs {
		title := r.Title
		if title == "" {
			title = "(no prompt yet)"
		}
		if len([]rune(title)) > 56 {
			title = string([]rune(title)[:55]) + "…"
		}
		started := r.StartedAt.Local().Format("Jan 2 15:04")
		line := fmt.Sprintf("  %2d. %-*s  %-56s  %-13s  %s", i+1, shortIDLen, shortID(r.ChannelID), title, started, humanSince(r.UpdatedAt))
		if showDir && r.Dir != "" {
			line += "  " + shortDir(r.Dir)
		}
		fmt.Println(line)
	}
}

func shortDir(dir string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(dir, home) {
		return "~" + strings.TrimPrefix(dir, home)
	}
	return dir
}

var conversationsCmd = &cobra.Command{
	Use:   "conversations",
	Short: "List recent conversations (what `memdoor resume` picks from)",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		dir := ""
		if !all {
			dir, _ = os.Getwd()
		}
		recs := sessionsFor(dir)
		if len(recs) == 0 {
			fmt.Println("No conversations recorded yet — `memdoor tui` starts one.")
			return nil
		}
		printSessions(recs, all)
		return nil
	},
}

func init() {
	resumeCmd.Flags().Bool("all", false, "Every conversation, not just this directory's")
	resumeCmd.Flags().Bool("last", false, "Resume the most recent without asking")
	resumeCmd.Flags().Bool("headless", false, "Print the conversation as text instead of opening a window (for a pipe)")
	resumeCmd.Flags().Bool("follow", false, "With --headless: keep printing what a running turn does until it ends")
	resumeCmd.Flags().Int("limit", 200, "With --headless: how many messages to print")
	conversationsCmd.Flags().Bool("all", false, "Every conversation, not just this directory's")
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(conversationsCmd)
}

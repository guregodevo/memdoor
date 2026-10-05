package cmd

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View and query gateway logs",
}

var (
	logsLimit         int
	logsSince         string
	logsRegex         string
	logsOrder         string
	logsWorkspaceFlag string
	logsShowData      bool
	logsSession       string
	logsRun           string
)

var (
	logsPruneBefore string
	logsPruneDays   int
	logsPruneAll    bool
)

var logsPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Delete old log events and reclaim disk space",
	Long: `Prune the gateway's event log to reclaim space. Specify exactly one of:
  --before <dur>   delete events older than a Go duration (e.g. 720h)
  --days <n>       delete events older than n days
  --all            delete every event

Runs a VACUUM afterward so freed pages return to the OS. Automatic
retention also runs in the gateway (30 days);
this command is the manual escape hatch.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		q := url.Values{}
		switch {
		case logsPruneAll:
			q.Set("all", "true")
		case logsPruneBefore != "":
			q.Set("before", logsPruneBefore)
		case logsPruneDays > 0:
			q.Set("days", strconv.Itoa(logsPruneDays))
		default:
			return fmt.Errorf("specify one of --before <dur>, --days <n>, or --all")
		}
		var resp struct {
			Deleted int64  `json:"deleted"`
			Cutoff  string `json:"cutoff"`
		}
		if err := NewClient().PostJSON("/api/logs/prune?"+q.Encode(), nil, &resp); err != nil {
			return err
		}
		fmt.Printf("✓ pruned %d log event(s) older than %s\n", resp.Deleted, resp.Cutoff)
		return nil
	},
}

var logsQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query log events",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		params := url.Values{}
		params.Set("limit", strconv.Itoa(logsLimit))
		if logsSince != "" {
			params.Set("since", logsSince)
		}
		if logsRegex != "" {
			params.Set("regex", logsRegex)
		}
		if logsOrder != "" {
			params.Set("order", logsOrder)
		}
		if logsWorkspaceFlag != "" {
			params.Set("workspace", logsWorkspaceFlag)
		}
		if logsSession != "" {
			params.Set("session", logsSession)
		}
		if logsRun != "" {
			params.Set("run", logsRun)
		}

		var result struct {
			Events []map[string]interface{} `json:"events"`
		}
		if err := c.GetJSON("/api/logs/query?"+params.Encode(), &result); err != nil {
			return err
		}
		events := result.Events

		for _, e := range events {
			ts := ""
			if t, ok := e["timestamp"].(string); ok {
				if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
					ts = parsed.Format("15:04:05.000")
				} else {
					ts = t
				}
			}
			level := ""
			if l, ok := e["level"]; ok {
				level = fmt.Sprintf("%v", l)
			}
			component := ""
			if c, ok := e["component"]; ok && c != nil {
				component = fmt.Sprintf("[%v]", c)
			}
			msg := ""
			if m, ok := e["message"]; ok {
				msg = fmt.Sprintf("%v", m)
			}
			fmt.Printf("%s %-5s %-12s %s\n", ts, level, component, msg)
			if logsShowData {
				if id, ok := e["id"].(string); ok && id != "" {
					fmt.Printf("    id=%s\n", id)
				}
				// Which conversation and run the line belongs to: a parent's
				// tool calls and a sub-session's read alike without them.
				for _, k := range []string{"session", "run_id"} {
					if v, ok := e[k].(string); ok && v != "" {
						fmt.Printf("    %s=%s\n", k, v)
					}
				}
				if data, ok := e["data"].(map[string]interface{}); ok {
					keys := make([]string, 0, len(data))
					for k := range data {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						fmt.Printf("    %s=%v\n", k, data[k])
					}
				}
			}
		}

		fmt.Printf("\n(%d events)\n", len(events))
		return nil
	},
}

var logsErrorsCmd = &cobra.Command{
	Use:   "errors",
	Short: "Show recent errors",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		params := url.Values{}
		if logsSince != "" {
			params.Set("since", logsSince)
		}
		params.Set("limit", strconv.Itoa(logsLimit))

		var result struct {
			Errors []map[string]interface{} `json:"events"`
		}
		if err := c.GetJSON("/api/logs/errors?"+params.Encode(), &result); err != nil {
			return err
		}
		errors := result.Errors

		if len(errors) == 0 {
			fmt.Println("No errors found")
			return nil
		}

		for _, e := range errors {
			ts := ""
			if t, ok := e["timestamp"].(string); ok {
				if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
					ts = parsed.Format("2006-01-02 15:04:05")
				}
			}
			msg := fmt.Sprintf("%v", e["message"])
			component := ""
			if c, ok := e["component"]; ok && c != nil {
				component = fmt.Sprintf("[%v] ", c)
			}
			fmt.Printf("%s ERROR %s%s\n", ts, component, msg)
		}
		fmt.Printf("\n(%d errors)\n", len(errors))
		return nil
	},
}

var logsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show log statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		var stats map[string]interface{}
		if err := c.GetJSON("/api/logs/stats", &stats); err != nil {
			return err
		}

		for k, v := range stats {
			fmt.Printf("%-20s %v\n", k+":", v)
		}
		return nil
	},
}

var logsTraceCmd = &cobra.Command{
	Use:   "trace",
	Short: "Trace a log event",
	RunE: func(cmd *cobra.Command, args []string) error {
		eventID, _ := cmd.Flags().GetString("event-id")
		if eventID == "" && len(args) > 0 {
			eventID = args[0]
		}
		if eventID == "" {
			return fmt.Errorf("--event-id is required")
		}

		c := NewClient()
		body, err := c.GetText("/api/logs/trace?event_id=" + eventID)
		if err != nil {
			return err
		}
		fmt.Println(body)
		return nil
	},
}

var logsSessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Show session timeline",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session-id")
		if sessionID == "" && len(args) > 0 {
			sessionID = args[0]
		}
		if sessionID == "" {
			return fmt.Errorf("--session-id is required")
		}

		c := NewClient()
		body, err := c.GetText("/api/logs/session?session_id=" + sessionID)
		if err != nil {
			return err
		}
		fmt.Println(body)
		return nil
	},
}

func init() {
	logsQueryCmd.Flags().IntVar(&logsLimit, "limit", 50, "Number of events")
	logsQueryCmd.Flags().StringVar(&logsSince, "since", "", "Time range (e.g., 5m, 1h)")
	logsQueryCmd.Flags().StringVar(&logsRegex, "regex", "", "Filter by regex pattern")
	logsQueryCmd.Flags().StringVar(&logsOrder, "order", "", "Sort order: asc|desc")
	logsQueryCmd.Flags().StringVar(&logsSession, "session", "", "Only this session's events (the session= that --data shows)")
	logsQueryCmd.Flags().StringVar(&logsRun, "run", "", "Only this run's events (the run_id= that --data shows)")
	logsQueryCmd.Flags().StringVar(&logsWorkspaceFlag, "workspace-filter", "", "Filter to events whose data.workspace equals this slug (what the workspace-scoped handlers attach via slog)")
	logsQueryCmd.Flags().BoolVar(&logsShowData, "data", false, "Show each event's id and data attributes")

	logsErrorsCmd.Flags().IntVar(&logsLimit, "limit", 20, "Number of errors")
	logsErrorsCmd.Flags().StringVar(&logsSince, "since", "", "Time range (e.g., 5m, 1h)")

	logsTraceCmd.Flags().String("event-id", "", "Event ID to trace")
	logsSessionCmd.Flags().String("session-id", "", "Session ID")

	logsPruneCmd.Flags().StringVar(&logsPruneBefore, "before", "", "Delete events older than this Go duration (e.g. 720h)")
	logsPruneCmd.Flags().IntVar(&logsPruneDays, "days", 0, "Delete events older than n days")
	logsPruneCmd.Flags().BoolVar(&logsPruneAll, "all", false, "Delete all events")
	// The help promises "exactly one of"; enforce it so e.g. `--days 30 --all`
	// errors instead of silently letting --all win and wiping everything.
	logsPruneCmd.MarkFlagsMutuallyExclusive("before", "days", "all")

	logsCmd.AddCommand(logsPruneCmd)
	logsCmd.AddCommand(logsQueryCmd)
	logsCmd.AddCommand(logsErrorsCmd)
	logsCmd.AddCommand(logsStatsCmd)
	logsCmd.AddCommand(logsTraceCmd)
	logsCmd.AddCommand(logsSessionCmd)
	rootCmd.AddCommand(logsCmd)
}

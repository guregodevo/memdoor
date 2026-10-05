package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// cronRunRecord mirrors gateway domain.CronRunRecord's JSON. Decoding into a
// typed struct (not map[string]interface{}) is deliberate: it was the map that
// let the field names drift silently — the CLI read "timestamp"/"status" while
// the API sent "start_time"/"success", so every row printed <nil>. A rename now
// leaves a blank column that a contract test catches, not a mystery at runtime.
type cronRunRecord struct {
	JobID      string    `json:"job_id"`
	StartTime  time.Time `json:"start_time"`
	DurationMs int64     `json:"duration_ms"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
}

// formatCronDuration renders a run's duration_ms the way a person reads it.
func formatCronDuration(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return time.Duration(ms * int64(time.Millisecond)).Round(time.Millisecond).String()
}

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage scheduled cron jobs",
}

var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cron jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		var result struct {
			Jobs []map[string]interface{} `json:"jobs"`
		}
		if err := c.GetJSON("/api/cron", &result); err != nil {
			return err
		}
		jobs := result.Jobs

		fmt.Printf("%-15s %-15s %-10s %-8s %s\n", "JOB ID", "SCHEDULE", "AGENT", "ENABLED", "MESSAGE")
		fmt.Printf("%-15s %-15s %-10s %-8s %s\n", "------", "--------", "-----", "-------", "-------")
		for _, j := range jobs {
			id := fmt.Sprintf("%v", j["id"])
			schedule := fmt.Sprintf("%v", j["schedule"])
			agent := ""
			if a, ok := j["agent_id"]; ok && a != nil {
				agent = fmt.Sprintf("%v", a)
			}
			enabled := "no"
			if e, ok := j["enabled"].(bool); ok && e {
				enabled = "yes"
			}
			message := ""
			if m, ok := j["message"]; ok {
				message = fmt.Sprintf("%v", m)
			}
			fmt.Printf("%-15s %-15s %-10s %-8s %s\n", id, schedule, agent, enabled, message)
		}
		return nil
	},
}

var (
	cronAddWorkflow, cronAddPartition, cronAddDir string
	cronAddID                                     string
	cronAddSchedule                               string
	cronAddMessage                                string
	cronAddAgent                                  string
	cronAddEnabled                                bool
)

var cronAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new cron job",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		message := cronAddMessage
		workdir := ""
		if cronAddWorkflow != "" {
			// A scheduled workflow: the same words as in the window, run in
			// this project (or --dir).
			message = "/workflow run " + cronAddWorkflow
			if cronAddPartition != "" {
				message += " " + cronAddPartition
			}
			dir := cronAddDir
			if dir == "" {
				dir, _ = os.Getwd()
			}
			abs, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			workdir = abs
		}
		if message == "" {
			return fmt.Errorf("give --message for an agent turn, or --workflow for a workflow run")
		}
		body := map[string]interface{}{
			"id":       cronAddID,
			"schedule": cronAddSchedule,
			"message":  message,
			"enabled":  cronAddEnabled,
		}
		if workdir != "" {
			body["workdir"] = workdir
		}
		if cronAddAgent != "" {
			body["agent_id"] = cronAddAgent
		}

		if err := c.PostExpectOK("/api/cron", body); err != nil {
			return err
		}

		fmt.Printf("Cron job %q added successfully\n", cronAddID)
		fmt.Printf("  Schedule: %s\n", cronAddSchedule)
		fmt.Printf("  Message:  %s\n", cronAddMessage)
		if cronAddEnabled {
			fmt.Println("  Status:   Enabled (will run on schedule)")
		} else {
			fmt.Println("  Status:   Disabled")
		}
		return nil
	},
}

var cronRemoveCmd = &cobra.Command{
	Use:   "remove [job-id]",
	Short: "Remove a cron job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		if err := c.DeleteExpectOK("/api/cron/" + args[0]); err != nil {
			return err
		}
		fmt.Printf("Cron job %q removed successfully\n", args[0])
		return nil
	},
}

var cronHistoryLimit int

var cronHistoryCmd = &cobra.Command{
	Use:   "history [job-id]",
	Short: "Show execution history",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		params := url.Values{}
		params.Set("limit", strconv.Itoa(cronHistoryLimit))
		if len(args) > 0 {
			params.Set("job_id", args[0])
		}

		var result struct {
			Records []cronRunRecord `json:"records"`
		}
		if err := c.GetJSON("/api/cron-history?"+params.Encode(), &result); err != nil {
			return err
		}
		history := result.Records

		if len(history) == 0 {
			fmt.Println("No cron runs yet.")
			return nil
		}

		fmt.Printf("%-20s %-10s %-8s %s\n", "TIME", "DURATION", "STATUS", "ERROR")
		fmt.Printf("%-20s %-10s %-8s %s\n", "----", "--------", "------", "-----")
		for _, h := range history {
			ts := "-"
			if !h.StartTime.IsZero() {
				ts = h.StartTime.Local().Format("2006-01-02 15:04:05")
			}
			status := "ok"
			if !h.Success {
				status = "failed"
			}
			fmt.Printf("%-20s %-10s %-8s %s\n", ts, formatCronDuration(h.DurationMs), status, h.Error)
		}
		return nil
	},
}

var cronStatsCmd = &cobra.Command{
	Use:   "stats [job-id]",
	Short: "Show job statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		path := "/api/cron-stats"
		if len(args) > 0 {
			path += "?job_id=" + args[0]
		}

		var result struct {
			Stats []map[string]interface{} `json:"stats"`
		}
		if err := c.GetJSON(path, &result); err != nil {
			return err
		}
		stats := result.Stats

		fmt.Printf("%-15s %-6s %-8s %-7s %-12s %-12s %s\n", "JOB ID", "TOTAL", "SUCCESS", "FAILED", "SUCCESS RATE", "AVG DURATION", "LAST RUN")
		fmt.Printf("%-15s %-6s %-8s %-7s %-12s %-12s %s\n", "------", "-----", "-------", "------", "------------", "------------", "--------")
		for _, s := range stats {
			fmt.Printf("%-15v %-6v %-8v %-7v %-12v %-12v %v\n",
				s["job_id"], s["total"], s["success"], s["failed"],
				s["success_rate"], s["avg_duration"], s["last_run"])
		}
		return nil
	},
}

func init() {
	cronAddCmd.Flags().StringVar(&cronAddID, "id", "", "Job identifier (required)")
	cronAddCmd.Flags().StringVar(&cronAddSchedule, "schedule", "", "Cron schedule (required)")
	cronAddCmd.Flags().StringVar(&cronAddMessage, "message", "", "Job message: what the agent is asked (or use --workflow)")
	cronAddCmd.Flags().StringVar(&cronAddWorkflow, "workflow", "", "Run this workflow of the project on the schedule instead of an agent turn")
	cronAddCmd.Flags().StringVar(&cronAddPartition, "partition", "", "With --workflow: the run's partition, e.g. today (once a day); default a fresh run each time")
	cronAddCmd.Flags().StringVar(&cronAddDir, "dir", "", "With --workflow: the project directory (default: here)")
	cronAddCmd.Flags().StringVar(&cronAddAgent, "agent", "", "Agent ID")
	cronAddCmd.Flags().BoolVar(&cronAddEnabled, "enabled", true, "Enable immediately")
	_ = cronAddCmd.MarkFlagRequired("id")
	_ = cronAddCmd.MarkFlagRequired("schedule")

	cronHistoryCmd.Flags().IntVar(&cronHistoryLimit, "limit", 10, "Number of records")

	cronCmd.AddCommand(cronListCmd)
	cronCmd.AddCommand(cronAddCmd)
	cronCmd.AddCommand(cronRemoveCmd)
	cronCmd.AddCommand(cronHistoryCmd)
	cronCmd.AddCommand(cronStatsCmd)
	rootCmd.AddCommand(cronCmd)
}

package cmd

import (
	"fmt"
	"time"

	"memdoor/gateway/providers"
	"memdoor/pkg/metering"

	"github.com/spf13/cobra"
)

func cacheCell(e providers.MeterEntry) string {
	if e.CachedTokens <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", e.CacheHitRate()*100)
}

// meterCmd lists every model request from the ledger /usage sums
// (~/.memdoor/meter.jsonl): per-engine totals, the last N requests, or the
// last 30 days per user.
var meterCmd = &cobra.Command{
	Use:   "meter",
	Short: "Every model request: the model that answered, its tokens and cost (the ledger /usage sums)",
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := providers.ReadMeter()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("Meter is empty — no model requests recorded yet.")
			return nil
		}
		if users, _ := cmd.Flags().GetBool("users"); users {
			since := time.Now().AddDate(0, 0, -30)
			per := metering.UsageByUser(entries, since)
			fmt.Printf("Last 30 days, per user\n")
			fmt.Printf("%-40s %-6s %-9s %-10s %-11s %s\n", "USER", "REQUESTS", "SESSIONS", "BRAIN MIN", "OUT TOKENS", "COST $")
			for _, u := range per {
				fmt.Printf("%-40s %-6d %-9d %-10.1f %-11d %.2f\n", u.User, u.Turns, u.Sessions, float64(u.BrainMS)/60_000, u.OutputTokens, u.EstCost)
			}
			return nil
		}
		if tail, _ := cmd.Flags().GetInt("tail"); tail > 0 {
			if tail > len(entries) {
				tail = len(entries)
			}
			for _, e := range entries[len(entries)-tail:] {
				fmt.Printf("%s  %-14s %6d in %6d out  %5s cached  %6.1fs\n",
					e.TS.Local().Format("01-02 15:04:05"), e.Engine, e.InputTokens, e.OutputTokens,
					cacheCell(e), float64(e.DurationMS)/1000)
			}
			return nil
		}
		type agg struct {
			requests      int
			in, out       int64
			cached        int64
			cachedOf      int64 // input tokens on requests that REPORTED a cache figure
			busyMS        int64
			todayRequests int
		}
		today := time.Now().Local().Format("2006-01-02")
		per := map[string]*agg{}
		for _, e := range entries {
			a := per[e.Engine]
			if a == nil {
				a = &agg{}
				per[e.Engine] = a
			}
			a.requests++
			a.in += e.InputTokens
			a.out += e.OutputTokens
			// Only average over requests the engine actually reported on, or a
			// server that publishes nothing reads as a 0% hit rate.
			if e.CachedTokens > 0 {
				a.cached += e.CachedTokens
				a.cachedOf += e.InputTokens
			}
			a.busyMS += e.DurationMS
			if e.TS.Local().Format("2006-01-02") == today {
				a.todayRequests++
			}
		}
		fmt.Printf("%-16s %-7s %-7s %10s %10s %8s %9s\n", "ENGINE", "REQUESTS", "TODAY", "TOKENS IN", "TOKENS OUT", "CACHED", "TIME")
		for name, a := range per {
			hit := "n/a"
			if a.cachedOf > 0 {
				hit = fmt.Sprintf("%.0f%%", float64(a.cached)/float64(a.cachedOf)*100)
			}
			fmt.Printf("%-16s %-7d %-7d %10d %10d %8s %8.1fs\n",
				name, a.requests, a.todayRequests, a.in, a.out, hit, float64(a.busyMS)/1000)
		}
		fmt.Println("\nCACHED is the share of prompt tokens served from the provider's prefix cache,")
		fmt.Println("averaged only over requests that reported it. \"n/a\" means the provider")
		fmt.Println("published nothing, which is NOT the same as a miss.")
		fmt.Println("Ledger: ~/.memdoor/meter.jsonl   Recent requests: memdoor meter --tail 10")
		return nil
	},
}

func init() {
	meterCmd.Flags().Bool("users", false, "Per-user usage over the last 30 days: model requests, sessions, brain minutes, cost")
	meterCmd.Flags().Int("tail", 0, "Show the last N metered model requests instead of the summary")
	rootCmd.AddCommand(meterCmd)
}

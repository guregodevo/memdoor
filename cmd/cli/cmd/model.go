package cmd

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// memdoor model: the same catalogue the window's /model-search reads, from
// the shell (Greg, 2026-09-26: "update also command line"). Pinning is per
// conversation, so it lives in the window (/model <id>); here you find the
// id and see each agent's ladder.
var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Models: each agent's ladder, and the catalogue to pin from",
	Long: `Which models answer, and which you can pick.

  memdoor model                  each agent's ladder, first rung first
  memdoor model search <name>    find a model in the catalogue (id, context, cost band)
  memdoor model providers <id>   a model's hosts, to prefer or reorder

In the window, /model shows this conversation's model and pins one:
/model 2 (a rung), /model vendor/name [price|throughput|latency|default]
[order host1,host2] (any model from the search), /model auto.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var st struct {
			AgentLadders map[string][]string `json:"agent_ladders"`
			AgentModels  map[string]string   `json:"agent_models"`
		}
		if err := NewClient().GetJSON("/api/llm/burst", &st); err != nil {
			return err
		}
		if len(st.AgentLadders) == 0 && len(st.AgentModels) == 0 {
			fmt.Println("No brain is serving yet.")
			return nil
		}
		// Every agent either side knows about: a BYOK gateway resolves the
		// ladders in the binary and may name no single model per agent, and
		// printing only the models listed nothing at all (2026-09-27).
		agents := make([]string, 0, len(st.AgentModels)+len(st.AgentLadders))
		seen := map[string]bool{}
		for a := range st.AgentModels {
			agents, seen[a] = append(agents, a), true
		}
		for a := range st.AgentLadders {
			if !seen[a] {
				agents = append(agents, a)
			}
		}
		sort.Strings(agents)
		for _, agent := range agents {
			if ladder := st.AgentLadders[agent]; len(ladder) > 1 {
				fmt.Printf("%-10s %s\n", agent, strings.Join(ladder, " → "))
			} else {
				fmt.Printf("%-10s %s\n", agent, st.AgentModels[agent])
			}
		}
		return nil
	},
}

// `memdoor model check` — can this model actually run the loop? (Greg,
// 2026-09-27: "the coding agent should work with any model!? … Maybe we should
// detect it".) The catalogue says a model advertises tools; this asks it to emit
// a call, pick the right one, carry on after a result, and write a patch our
// parser accepts, on your own key, for a few hundred tokens.
var modelCheckCmd = &cobra.Command{
	Use:   "check [vendor/name ...]",
	Short: "Probe a model for what a coder turn needs (no argument: the ladder)",
	Example: `  memdoor model check z-ai/glm-5.3   one model
  memdoor model check                every rung of the coder's ladder
  memdoor model check --saved        what was probed already, spending nothing
  memdoor model check --stale        only the reports that have aged out (a weekly cron line)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if saved, _ := cmd.Flags().GetBool("saved"); saved {
			var res struct {
				Checks map[string]modelCheckView `json:"checks"`
			}
			if err := NewClient().GetJSON("/api/models/check", &res); err != nil {
				return err
			}
			if len(res.Checks) == 0 {
				fmt.Println("Nothing probed yet — `memdoor model check` tries the ladder.")
				return nil
			}
			for _, c := range res.Checks {
				printModelCheck(c)
			}
			return nil
		}
		// A SCHEDULE IS THE PERSON'S CRON, NOT A TIMER OF OURS. A probe is
		// three or four real calls on their key, so nothing re-probes in the
		// background; this is the one command a weekly line can call, and it
		// spends nothing when every report is fresh.
		if stale, _ := cmd.Flags().GetBool("stale"); stale {
			var res struct {
				Checks []modelCheckView `json:"checks"`
			}
			if err := NewClient().PostJSON("/api/models/check?stale=1", map[string]any{}, &res); err != nil {
				return err
			}
			if len(res.Checks) == 0 {
				fmt.Println("Every report is fresh — nothing probed, nothing spent.")
				return nil
			}
			for _, c := range res.Checks {
				printModelCheck(c)
			}
			return nil
		}
		targets := args
		if len(targets) == 0 {
			targets = []string{""} // the ladder
		}
		fmt.Println("Probing (four or five calls per model, on your key)…")
		for _, id := range targets {
			path := "/api/models/check?ladder=1"
			if id != "" {
				path = "/api/models/check?id=" + url.QueryEscape(id)
			}
			var res struct {
				Checks []modelCheckView `json:"checks"`
			}
			if err := NewClient().PostJSON(path, map[string]any{}, &res); err != nil {
				fmt.Printf("  %-34s could not be probed: %v\n", id, err)
				continue
			}
			for _, c := range res.Checks {
				printModelCheck(c)
			}
		}
		fmt.Println("")
		fmt.Println("  call = emitted a tool call · right = chose the tool the task needed")
		fmt.Println("  args = arguments parsed · answer = carried on after a result")
		fmt.Println("  patch = apply_patch our parser accepts (the text-tag fallback covers a miss)")
		fmt.Println("  follows = told to load the workflow skill first, did so (? = probed before this check existed)")
		return nil
	},
}

// modelCheckView mirrors gateway.modelCheck over the wire.
type modelCheckView struct {
	ID        string    `json:"id"`
	At        time.Time `json:"at"`
	ToolCall  bool      `json:"tool_call"`
	RightTool bool      `json:"right_tool"`
	ValidArgs bool      `json:"valid_args"`
	RoundTrip bool      `json:"round_trip"`
	Patch     bool      `json:"patch"`
	Followed  string    `json:"followed"` // "yes" | "no" | "" (not probed)
	Ms        int64     `json:"ms"`
	InTokens  int64     `json:"in_tokens"`
	OutTokens int64     `json:"out_tokens"`
	Note      string    `json:"note"`
}

func printModelCheck(c modelCheckView) {
	mark := func(ok bool) string {
		if ok {
			return "✓"
		}
		return "·"
	}
	// The age belongs in the table for the same reason the pin warning dates
	// itself: a model is a moving target, and a report with no date is a claim
	// with no expiry.
	follows := "?"
	switch c.Followed {
	case "yes":
		follows = "✓"
	case "no":
		follows = "·"
	}
	fmt.Printf("  %-34s %s call %s right %s args %s answer %s patch %s follows  %5dms  %s\n",
		c.ID, mark(c.ToolCall), mark(c.RightTool), mark(c.ValidArgs), mark(c.RoundTrip), mark(c.Patch), follows, c.Ms, probedWhen(c.At))
	if c.Note != "" {
		fmt.Printf("  %-34s %s\n", "", c.Note)
	}
}

var modelProvidersCmd = &cobra.Command{
	Use:   "providers <vendor/name>",
	Short: "A model's hosts: precision, context, uptime, cost band",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		hosts, err := modelProviders(args[0])
		if err != nil {
			return err
		}
		fmt.Println(strings.ReplaceAll(renderModelProviders(args[0], hosts), "**", ""))
		return nil
	},
}

var modelSearchCmd = &cobra.Command{
	Use:   "search <name>",
	Short: "Find a model in the catalogue",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q := strings.Join(args, " ")
		models, err := searchModelCatalog(q)
		if err != nil {
			return err
		}
		fmt.Println(strings.ReplaceAll(renderModelCatalog(q, models), "**", ""))
		return nil
	},
}

func init() {
	modelCheckCmd.Flags().Bool("saved", false, "Print what was probed already; probe nothing")
	modelCheckCmd.Flags().Bool("stale", false, "Re-probe only the reports older than a week (what a cron line calls)")
	modelCmd.AddCommand(modelSearchCmd, modelProvidersCmd, modelCheckCmd)
	rootCmd.AddCommand(modelCmd)
}

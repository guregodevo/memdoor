package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"memdoor/pkg/savings"
)

// `memdoor savings` — the receipt for the product's one claim, read from the
// person's own turns (pkg/savings). Greg, 2026-09-27: "let's focus on cheaper
// by efficiency". This is the number, on their machine, not in our benchmark.
//
// Everything here is measured, and the arithmetic is stated rather than
// implied: bytes come from the ledger, tokens are bytes over four, and dollars
// are priced at the CHEAPEST model the month actually ran on, so the figure is
// a floor and not a flattering one.
var savingsCmd = &cobra.Command{
	Use:   "savings",
	Short: "What the decision model kept out of your model bill this month",
	Long: `What the decision model kept out of your model bill this month.

Judged reads: the bytes a tool would have returned against the bytes it did.
The toolbox: tool schemas the narrowed palette did not carry, per call.
Early stops: turns ended for want of progress, counted and never priced.

Priced at the cheapest model the month ran on, so it is a floor. Your provider's
invoice is the authority on what you paid; this is what you did not.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		when := time.Now()
		if month, _ := cmd.Flags().GetString("month"); month != "" {
			t, err := time.Parse("2006-01", month)
			if err != nil {
				return fmt.Errorf("a month looks like 2026-09, not %q", month)
			}
			when = t
		}
		s, err := savings.ReadMonth(when)
		if err != nil {
			return err
		}
		if s.JudgedCalls == 0 && s.ToolboxCalls == 0 && s.EarlyStops == 0 {
			fmt.Printf("Nothing recorded for %s yet. The decision model writes this ledger as it works:\n", s.Month)
			fmt.Println("  is it on?   with an OpenRouter or decision key it is; without one nothing is judged")
			fmt.Println("              `memdoor connect typesafe` turns it on next to any chat provider")
			return nil
		}

		fmt.Printf("Saved in %s\n\n", s.Month)
		if s.JudgedCalls > 0 {
			fmt.Printf("  Judged reads   %s of %s read by the judge, %s passed on   (%d calls)\n",
				humanBytes(s.JudgedRaw-s.JudgedKept), humanBytes(s.JudgedRaw), humanBytes(s.JudgedKept), s.JudgedCalls)
			for _, t := range topTools(s.ByTool, 4) {
				fmt.Printf("                 %-12s %s\n", t.name, humanBytes(t.saved))
			}
		}
		if s.ToolboxCalls > 0 {
			fmt.Printf("  Toolbox        %s of tool schemas not sent            (%d calls)\n",
				humanBytes(s.ToolboxSaved), s.ToolboxCalls)
		}
		if s.EarlyStops > 0 {
			fmt.Printf("  Early stops    %d turn%s ended for want of progress    (not priced)\n", s.EarlyStops, plural(s.EarlyStops))
		}

		tokens := s.TokensSaved()
		fmt.Printf("\n  ≈ %s input tokens your models never read (%v bytes to a token)\n",
			humanCount(tokens), int(s.BytesPerToken))

		name, price := cheapestModelPrice(s.ModelsSeen)
		if price > 0 {
			fmt.Printf("  ≈ %s at %s ($%.2f per million in) — a floor: that is the cheapest\n",
				money(s.DollarsSaved(price)), name, price)
			fmt.Printf("    model you ran this month\n")
		}
		// The same tokens, priced at what a premium model charges. Labelled as
		// arithmetic, not as a claim about anyone's product: it is the reason
		// the saving is worth more to someone coding on an expensive model.
		fmt.Printf("  ≈ %s of the same tokens at $3.00 per million, the order a premium\n", money(s.DollarsSaved(3)))
		fmt.Printf("    model charges\n")
		fmt.Printf("\n  The ledger is ~/.memdoor/savings.jsonl, append-only. Your provider's invoice\n")
		fmt.Printf("  says what you paid; this says what you did not.\n")
		return nil
	},
}

func init() {
	savingsCmd.Flags().String("month", "", "A month to sum instead of this one (2026-09)")
	rootCmd.AddCommand(savingsCmd)
}

// cheapestModelPrice finds the cheapest input price among the models the month
// ran on, from the catalogue the gateway can reach. A saving priced at the
// cheapest model is the least it can be worth.
func cheapestModelPrice(seen map[string]int) (string, float64) {
	if len(seen) == 0 {
		return "", 0
	}
	name, price := "", 0.0
	for id := range seen {
		models, err := searchModelCatalog(id)
		if err != nil {
			continue
		}
		for _, m := range models {
			if m.ID != id || m.InPerM <= 0 {
				continue
			}
			if price == 0 || m.InPerM < price {
				name, price = m.ID, m.InPerM
			}
		}
	}
	return name, price
}

type toolSaving struct {
	name  string
	saved int64
}

func topTools(by map[string]int64, n int) []toolSaving {
	out := make([]toolSaving, 0, len(by))
	for k, v := range by {
		out = append(out, toolSaving{k, v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].saved > out[j].saved })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// money keeps small figures legible: a receipt that reads "$0.00" tells nobody
// anything, and at four cents a million the honest number has four decimals.
func money(v float64) string {
	switch {
	case v >= 1:
		return fmt.Sprintf("$%.2f", v)
	case v >= 0.01:
		return fmt.Sprintf("$%.3f", v)
	case v > 0:
		return fmt.Sprintf("$%.4f", v)
	default:
		return "$0"
	}
}

func humanBytes(n int64) string {
	switch {
	case n < 0:
		return "0 B"
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	case n < 1<<30:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	}
}

func humanCount(n int64) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return fmt.Sprintf("%.0fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%.2fM", float64(n)/1e6)
	}
}

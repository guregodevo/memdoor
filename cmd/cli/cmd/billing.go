package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"memdoor/gateway/billingsvc"
	"memdoor/gateway/telemetry"
	"memdoor/pkg/shared"
)

// `memdoor billing` — the money service. SAME BINARY, separate process:
// it runs only on memdoor.ai, holds the Stripe secrets, and is never started
// by a self-hosted gateway.
var billingCmd = &cobra.Command{
	Use:   "billing",
	Short: "Run the billing service (memdoor.ai only: sign-in, Pro seats, Stripe)",
	Long: `Start the billing service: email sign-in, Pro seats through Stripe
Checkout and its webhook, and the remote-control relay's plan check. No seat
supplies a model key: gateways run on their user's own.

This process is intentionally separate from ` + "`memdoor gateway`" + `: money
endpoints never share a process with agent and tool execution, and a
self-hosted gateway carries no billing code or secrets at all.

Environment:
  MEMDOOR_BILLING_TOKEN   required — service auth for /v1 endpoints
  STRIPE_SECRET_KEY       sk_… — creates Checkout Sessions
  STRIPE_WEBHOOK_SECRET   whsec_… — verifies webhook signatures`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// The billing service is a SEPARATE systemd unit from the same binary,
		// so it does not run the gateway command and does not inherit its
		// setup. Without this, /v1/health reports version "" — which is what
		// the 2026-08-29 deploy showed: the field shipped and was empty, and
		// only deploying revealed it. Same reason as gateway.go's assignment,
		// same cycle-free direction (cmd -> telemetry).
		telemetry.GatewayVersion = Version

		port, _ := cmd.Flags().GetInt("port")
		dbPath, _ := cmd.Flags().GetString("db")
		if dbPath == "" {
			dbPath = shared.MemdoorHome("billing.db")
		}
		if os.Getenv("MEMDOOR_BILLING_TOKEN") == "" {
			return fmt.Errorf("MEMDOOR_BILLING_TOKEN is required — the billing service refuses to start unauthenticated")
		}
		srv, err := billingsvc.New(dbPath, port)
		if err != nil {
			return err
		}
		fmt.Printf("Billing service — db %s, port %d\n", dbPath, port)
		if os.Getenv("STRIPE_SECRET_KEY") == "" {
			fmt.Println("⚠ STRIPE_SECRET_KEY unset — Pro checkout will fail")
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return srv.Start(ctx)
	},
}

func init() {
	billingCmd.Flags().Int("port", 18790, "Port to serve the billing API on")
	billingCmd.Flags().String("db", "", "Billing database path (default ~/.memdoor/billing.db)")
	rootCmd.AddCommand(billingCmd)
}

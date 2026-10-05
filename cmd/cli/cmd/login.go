package cmd

import (
	"github.com/spf13/cobra"
)

// `memdoor login` is the sign-in, in the two words a person types first
// (Greg, 2026-09-29: "shortcut memdoor login"): `memdoor account login` under
// its short name, same flags, same code by email. The browser device flow it
// once had was removed on 2026-10-05 (see gateway/billingsvc/auth.go).
var loginCmd = &cobra.Command{
	Use:   "login [email]",
	Short: "Sign in: a code is emailed, type it here",
	Long: `Sign in to memdoor.ai with your email: a six-digit code arrives there, and you
type it here. The same as "memdoor account login".`,
	Example: `  memdoor login you@example.com
  memdoor login you@example.com --code 482913`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return accountLoginCmd.RunE(cmd, args)
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Sign out of memdoor.ai on this Mac (the engine stays signed in)",
	Args:  cobra.NoArgs,
	RunE:  func(cmd *cobra.Command, args []string) error { return signOut() },
}

func init() {
	rootCmd.AddCommand(logoutCmd)
	loginCmd.Flags().StringVar(&accountCode, "code", "", "The six-digit code from the email")
	loginCmd.Flags().BoolVar(&accountSendOnly, "send-only", false, "Only mail the code; verify later with --code")
	loginCmd.Flags().BoolVar(&accountJSON, "json", false, "Print the result as JSON")
	rootCmd.AddCommand(loginCmd)
}

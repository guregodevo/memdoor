package cmd

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Memdoor",
}

var (
	loginEmail    string
	loginPassword string
)

var authLoginDirectCmd = &cobra.Command{
	Use:   "login-direct",
	Short: "Authenticate with email and password (token persists 30 days)",
	Long: `Log in with email + password and save the session token to
~/.memdoor/credentials.json.

The token survives gateway restarts (sessions are stored hashed in
SQLite, not in memory) and stays valid for 30 days. To clear it
early, run 'memdoor auth logout'. To check current status and
expiry, run 'memdoor auth status'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		body := map[string]string{
			"email":    loginEmail,
			"password": loginPassword,
		}

		var result struct {
			Token string `json:"token"`
			User  struct {
				Email string `json:"email"`
				Name  string `json:"name"`
				Role  string `json:"role"`
			} `json:"user"`
		}
		if err := c.PostJSON("/api/auth/login", body, &result); err != nil {
			return err
		}

		creds := &credentials{
			Token: result.Token,
			Email: result.User.Email,
		}
		if err := saveCredentials(creds); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}

		fmt.Printf("Logged in as: %s (%s)\n", result.User.Email, result.User.Name)
		fmt.Printf("Role: %s\n", result.User.Role)
		return nil
	},
}

var (
	registerEmail       string
	registerPassword    string
	registerUsername    string
	registerName        string
	registerInviteToken string
)

var authRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a new account using an invite token",
	Long: `Self-register a new account from the CLI.

Memdoor uses invite-only public registration. Get an invite token
from a workspace admin (the admin can run 'memdoor invite send
--email ...' or use the web UI invite flow), then register with
that token via this command.

If you ARE the admin, use 'memdoor users create' instead — it
provisions accounts directly without needing an invite token.

Username defaults to the part of the email before '@'. Display name
defaults to the username. On success, you are auto-logged-in and
the credentials are saved for subsequent CLI commands.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		body := map[string]interface{}{
			"email":        registerEmail,
			"password":     registerPassword,
			"username":     registerUsername,
			"name":         registerName,
			"invite_token": registerInviteToken,
		}
		var result struct {
			Token string `json:"token"`
			User  struct {
				Email string `json:"email"`
				Name  string `json:"name"`
				Role  string `json:"role"`
			} `json:"user"`
		}
		if err := c.PostJSON("/api/auth/register", body, &result); err != nil {
			return err
		}
		creds := &credentials{
			Token: result.Token,
			Email: result.User.Email,
		}
		if err := saveCredentials(creds); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}
		fmt.Printf("Registered and logged in as: %s (%s)\n", result.User.Email, result.User.Name)
		fmt.Printf("Role: %s\n", result.User.Role)
		return nil
	},
}

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		var result struct {
			User struct {
				Email string `json:"email"`
				Name  string `json:"name"`
				Role  string `json:"role"`
			} `json:"user"`
		}
		if err := c.GetJSON("/api/auth/whoami", &result); err != nil {
			return fmt.Errorf("not authenticated: run 'memdoor auth login-direct --email <email> --password <pass>'")
		}

		fmt.Printf("Logged in as: %s (%s)\n", result.User.Email, result.User.Name)
		fmt.Printf("Role: %s\n", result.User.Role)
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear saved credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = deleteCredentials()
		fmt.Println("Logged out successfully")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, err := loadCredentials()
		if err != nil {
			fmt.Println("Not authenticated")
			fmt.Println("  Run: memdoor login you@example.com")
			return nil
		}
		fmt.Printf("Authenticated as: %s\n", creds.Email)
		if !creds.ExpiresAt.IsZero() {
			remaining := time.Until(creds.ExpiresAt)
			if remaining <= 0 {
				fmt.Printf("Token expired %s ago — sign in again with 'memdoor login'\n",
					humanizeDuration(-remaining))
			} else {
				fmt.Printf("Token expires in %s (%s)\n",
					humanizeDuration(remaining),
					creds.ExpiresAt.Format("2006-01-02 15:04"))
			}
		}
		return nil
	},
}

// humanizeDuration formats a duration as "29 days", "3 hours", "12 minutes" —
// whichever bucket fits best. Used by 'auth status' so the user sees a
// human-readable expiry without parsing a timestamp.
func humanizeDuration(d time.Duration) string {
	if d >= 24*time.Hour {
		days := int(d / (24 * time.Hour))
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
	if d >= time.Hour {
		hours := int(d / time.Hour)
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	if d >= time.Minute {
		mins := int(d / time.Minute)
		return fmt.Sprintf("%d minutes", mins)
	}
	return fmt.Sprintf("%d seconds", int(d/time.Second))
}

var (
	changePasswordOld string
	changePasswordNew string
)

var authChangePasswordCmd = &cobra.Command{
	Use:   "change-password",
	Short: "Change your own password (requires current password)",
	Long: `Change your own password while logged in. You must prove your
current password with --old. Your current session stays valid, so
you do NOT need to re-login afterward.

  memdoor auth change-password --old 'current123' --new 'newsekret123'
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if changePasswordOld == "" || changePasswordNew == "" {
			return fmt.Errorf("both --old and --new are required")
		}
		c := NewClient()
		body := map[string]string{
			"old_password": changePasswordOld,
			"new_password": changePasswordNew,
		}
		if err := c.PostExpectOK("/api/auth/change-password", body); err != nil {
			return err
		}
		fmt.Println("Password changed.")
		return nil
	},
}

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Print current auth token (for use in curl/scripts)",
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, err := loadCredentials()
		if err != nil {
			return fmt.Errorf("not authenticated: run 'memdoor auth login-direct --email <email> --password <pass>'")
		}
		fmt.Print(creds.Token)
		return nil
	},
}

func init() {
	authLoginDirectCmd.Flags().StringVar(&loginEmail, "email", "", "Email address")
	authLoginDirectCmd.Flags().StringVar(&loginPassword, "password", "", "Password")
	_ = authLoginDirectCmd.MarkFlagRequired("email")
	_ = authLoginDirectCmd.MarkFlagRequired("password")

	authRegisterCmd.Flags().StringVar(&registerEmail, "email", "", "Email address")
	authRegisterCmd.Flags().StringVar(&registerPassword, "password", "", "Password (min 8 chars)")
	authRegisterCmd.Flags().StringVar(&registerUsername, "username", "", "Username (defaults to email prefix)")
	authRegisterCmd.Flags().StringVar(&registerName, "name", "", "Display name (defaults to username)")
	authRegisterCmd.Flags().StringVar(&registerInviteToken, "invite-token", "", "Invite token from a workspace admin")
	_ = authRegisterCmd.MarkFlagRequired("email")
	_ = authRegisterCmd.MarkFlagRequired("password")
	_ = authRegisterCmd.MarkFlagRequired("invite-token")

	authChangePasswordCmd.Flags().StringVar(&changePasswordOld, "old", "", "Current password")
	authChangePasswordCmd.Flags().StringVar(&changePasswordNew, "new", "", "New password (min 8 chars)")
	_ = authChangePasswordCmd.MarkFlagRequired("old")
	_ = authChangePasswordCmd.MarkFlagRequired("new")

	authCmd.AddCommand(authLoginDirectCmd)
	authCmd.AddCommand(authRegisterCmd)
	authCmd.AddCommand(authWhoamiCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authChangePasswordCmd)
	authCmd.AddCommand(authTokenCmd)
	rootCmd.AddCommand(authCmd)
}

func openBrowser(rawURL string) error {
	_, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "linux":
		return exec.Command("xdg-open", rawURL).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}

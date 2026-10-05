package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage workspace users (admin only)",
}

var usersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		users, err := fetchUsers(c)
		if err != nil {
			return err
		}

		fmt.Printf("%-40s %-30s %-10s %s\n", "EMAIL", "NAME", "ROLE", "VERIFIED")
		fmt.Println("------------------------------------------------------------------------------------")

		admins := 0
		regular := 0
		for _, u := range users {
			email := fmt.Sprintf("%v", u["email"])
			name := fmt.Sprintf("%v", u["name"])
			role := fmt.Sprintf("%v", u["role"])
			verified := ""
			if v, ok := u["email_verified"].(bool); ok && v {
				verified = "yes"
			}

			if role == "admin" {
				admins++
			} else {
				regular++
			}
			fmt.Printf("%-40s %-30s %-10s %s\n", email, name, role, verified)
		}

		fmt.Printf("\nTotal: %d users (%d admins, %d regular users)\n", len(users), admins, regular)
		return nil
	},
}

var (
	createEmail    string
	createUsername string
	createPassword string
	createName     string
	createAdmin    bool
)

var usersCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new user (admin only)",
	Long: `Provision a user account from the CLI — admin-only.

Replaces the web-UI signup form for the common admin-driven flow:
the admin runs this command, hands the new user their credentials,
and the new user logs in via 'memdoor auth login-direct'. Bypasses
the invite-token gate that 'memdoor auth register' requires.

Username defaults to the part of the email before '@'. Display name
defaults to the username. Use --admin to promote the new user to
workspace admin in the same call. The new account is marked
email_verified=true since the admin is vouching for it.

Examples:
  memdoor users create --email you@work.com --password 'sekret123' --admin
  memdoor users create --email teammate@work.com --password 'sekret123' --name "Teammate"
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		body := map[string]interface{}{
			"email":    createEmail,
			"password": createPassword,
			"username": createUsername,
			"name":     createName,
			"admin":    createAdmin,
		}
		var result struct {
			User map[string]interface{} `json:"user"`
		}
		if err := c.PostJSON("/api/admin/users", body, &result); err != nil {
			return err
		}
		role := "user"
		if createAdmin {
			role = "admin"
		}
		fmt.Printf("Created %s (%s)\n", createEmail, role)
		return nil
	},
}

var usersGrantAdminCmd = &cobra.Command{
	Use:   "grant-admin [email]",
	Short: "Promote user to admin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return updateUserRole(args[0], "admin", "granted admin role to")
	},
}

var usersRevokeAdminCmd = &cobra.Command{
	Use:   "revoke-admin [email]",
	Short: "Demote admin to regular user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return updateUserRole(args[0], "user", "revoked admin role from")
	},
}

var setPasswordValue string

var usersSetPasswordCmd = &cobra.Command{
	Use:   "set-password [email]",
	Short: "Set a user's password (admin only)",
	Long: `Set any user's password — admin-only. This is the rotation /
offboarding lever: the admin sets a new password and all of the
target user's sessions are invalidated, forcing them to re-login.

  memdoor users set-password teammate@work.com --password 'newsekret123'
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		email := args[0]
		if len(setPasswordValue) < 8 {
			return fmt.Errorf("password must be at least 8 characters")
		}

		c := NewClient()
		users, err := fetchUsers(c)
		if err != nil {
			return err
		}

		var userID string
		for _, u := range users {
			if fmt.Sprintf("%v", u["email"]) == email {
				userID = fmt.Sprintf("%v", u["id"])
				break
			}
		}
		if userID == "" {
			return fmt.Errorf("user not found: %s", email)
		}

		body := map[string]string{"new_password": setPasswordValue}
		if err := c.PostExpectOK("/api/admin/users/"+userID+"/password", body); err != nil {
			return err
		}
		fmt.Printf("Successfully set password for %s\n", email)
		return nil
	},
}

func fetchUsers(c *Client) ([]map[string]interface{}, error) {
	var result struct {
		Users []map[string]interface{} `json:"users"`
	}
	if err := c.GetJSON("/api/admin/users", &result); err != nil {
		return nil, err
	}
	return result.Users, nil
}

func updateUserRole(email, role, action string) error {
	c := NewClient()
	users, err := fetchUsers(c)
	if err != nil {
		return err
	}

	var userID string
	for _, u := range users {
		if fmt.Sprintf("%v", u["email"]) == email {
			userID = fmt.Sprintf("%v", u["id"])
			break
		}
	}
	if userID == "" {
		return fmt.Errorf("user not found: %s", email)
	}

	body := map[string]string{"role": role}
	if err := c.PutExpectOK("/api/admin/users/"+userID, body); err != nil {
		return err
	}
	fmt.Printf("Successfully %s %s\n", action, email)
	return nil
}

func init() {
	usersCreateCmd.Flags().StringVar(&createEmail, "email", "", "Email address")
	usersCreateCmd.Flags().StringVar(&createPassword, "password", "", "Password (min 8 chars)")
	usersCreateCmd.Flags().StringVar(&createUsername, "username", "", "Username (defaults to email prefix)")
	usersCreateCmd.Flags().StringVar(&createName, "name", "", "Display name (defaults to username)")
	usersCreateCmd.Flags().BoolVar(&createAdmin, "admin", false, "Promote new user to workspace admin")
	_ = usersCreateCmd.MarkFlagRequired("email")
	_ = usersCreateCmd.MarkFlagRequired("password")

	usersSetPasswordCmd.Flags().StringVar(&setPasswordValue, "password", "", "New password (min 8 chars)")
	_ = usersSetPasswordCmd.MarkFlagRequired("password")

	usersDeleteCmd.Flags().BoolVar(&usersDeleteYes, "yes", false, "Confirm the deletion (required)")

	usersCmd.AddCommand(usersCreateCmd)
	usersCmd.AddCommand(usersListCmd)
	usersCmd.AddCommand(usersGrantAdminCmd)
	usersCmd.AddCommand(usersRevokeAdminCmd)
	usersCmd.AddCommand(usersSetPasswordCmd)
	usersCmd.AddCommand(usersDeleteCmd)
	rootCmd.AddCommand(usersCmd)
}

var usersDeleteYes bool

var usersDeleteCmd = &cobra.Command{
	Use:   "delete [email]",
	Short: "Delete a user and all their data (admin only)",
	Long: `Permanently remove a user — their account, sessions, SSH keys, channel
memberships, and pending tokens — in one transaction. Admin-only and
destructive, so it requires --yes.

Guards: you can't delete your own account, a user in another workspace, or
the last admin of a workspace.

  memdoor users delete teammate@work.com --yes`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		email := args[0]
		c := NewClient()
		users, err := fetchUsers(c)
		if err != nil {
			return err
		}
		var userID string
		for _, u := range users {
			if fmt.Sprintf("%v", u["email"]) == email {
				userID = fmt.Sprintf("%v", u["id"])
				break
			}
		}
		if userID == "" {
			return fmt.Errorf("user not found: %s", email)
		}
		if !usersDeleteYes {
			fmt.Printf("Would delete %s (%s) and all their data. Re-run with --yes to confirm.\n", email, userID)
			return nil
		}
		if err := c.DeleteExpectOK("/api/admin/users/" + userID); err != nil {
			return err
		}
		fmt.Printf("✓ deleted %s\n", email)
		return nil
	},
}

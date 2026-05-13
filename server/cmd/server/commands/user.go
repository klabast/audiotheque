package commands

import (
	"audiod/internal/auth"
	"audiod/internal/branding"
	"audiod/internal/cli"
	"fmt"

	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "User management commands",
	Long:  "Manage " + branding.AppName + " users: create, list, delete users.",
}

var userCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new user",
	Long: `Create a new user account.

In setup mode (no users exist): First user must be an admin
In normal mode: New users are non-admin by default unless --admin flag is provided

Examples:
  audiod user create --username alice --password secret123 --admin
  audiod user create -u bob -p bobpass
  audiod user create  # Interactive mode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		isAdmin, _ := cmd.Flags().GetBool("admin")

		// Prompt for missing values
		if username == "" {
			username = cli.PromptForInput("Username: ")
		}
		if password == "" {
			password = cli.PromptForPassword("Password: ")
		}

		// Validate inputs
		if username == "" {
			return fmt.Errorf("username is required")
		}
		if password == "" {
			return fmt.Errorf("password is required")
		}

		// Create service
		repo := auth.NewRepository(db)
		service := auth.NewService(repo)

		// Call service layer
		user, err := service.CreateUser(username, password, isAdmin)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		adminStatus := ""
		if user.IsAdmin {
			adminStatus = " (admin)"
		}
		fmt.Printf("✓ User created successfully%s\n", adminStatus)
		fmt.Printf("  Username: %s\n", user.Username)
		return nil
	},
}

func init() {
	userCmd.AddCommand(userCreateCmd)

	// Flags for user create
	userCreateCmd.Flags().StringP("username", "u", "", "Username for the new user")
	userCreateCmd.Flags().StringP("password", "p", "", "Password for the new user")
	userCreateCmd.Flags().Bool("admin", false, "Make user an admin")
}

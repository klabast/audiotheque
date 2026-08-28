package commands

import (
	"audiod/internal/branding"
	"audiod/internal/cli"
	"audiod/internal/config"
	"audiod/internal/settings"
	"audiod/internal/system"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "System management commands",
	Long:  "Manage " + branding.AppName + " system: reset, status, maintenance.",
}

var systemResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset system to initial state",
	Long: "Reset the " + branding.AppName + " system to initial setup state.\n\n" +
		`WARNING: This will DELETE ALL USERS and reset the system!
Use with caution - this operation cannot be undone.

Examples:
  audiod system reset --confirm  # Skip confirmation prompt
  audiod system reset            # Interactive with confirmation`,
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")

		if !confirm {
			fmt.Println("╔════════════════════════════════════════════════════════════════════╗")
			fmt.Println("║                    ⚠️  DESTRUCTIVE OPERATION  ⚠️                    ║")
			fmt.Println("╠════════════════════════════════════════════════════════════════════╣")
			fmt.Println("║                                                                    ║")
			fmt.Println("║  This command will COMPLETELY WIPE your " + branding.AppName + " system:        ║")
			fmt.Println("║                                                                    ║")
			fmt.Println("║    • ALL user accounts will be DELETED                            ║")
			fmt.Println("║    • ALL authentication data will be LOST                         ║")
			fmt.Println("║    • System will reset to initial setup state                     ║")
			fmt.Println("║                                                                    ║")
			fmt.Println("║  ⚠️  BACKUP YOUR DATA BEFORE PROCEEDING  ⚠️                         ║")
			fmt.Println("║                                                                    ║")
			fmt.Println("║  This action CANNOT be undone!                                    ║")
			fmt.Println("║                                                                    ║")
			fmt.Println("╚════════════════════════════════════════════════════════════════════╝")
			fmt.Println()

			if !cli.PromptForConfirmation("Are you sure you want to reset the system?") {
				fmt.Println("❌ Operation cancelled")
				return nil
			}
		}

		repo := system.NewRepository(db)
		if err := repo.ResetAll(); err != nil {
			return fmt.Errorf("failed to reset system: %w", err)
		}

		// Reset codes live as JSON files under <data-dir>/reset_codes — wipe
		// them too, otherwise stale codes from a previous test run leak past
		// reset and confuse the password-reset E2E flow.
		resetCodesDir := filepath.Join(config.GetDataDir(), "reset_codes")
		if entries, err := os.ReadDir(resetCodesDir); err == nil {
			for _, e := range entries {
				_ = os.Remove(filepath.Join(resetCodesDir, e.Name()))
			}
		}

		fmt.Println()
		fmt.Println("✓ System has been reset to initial setup state")
		fmt.Println("  Database state cleared (users, libraries, devices, sessions, settings, reset codes)")
		fmt.Println("  Navigate to the application to create your admin account")
		return nil
	},
}

// systemAuthCmd is the recovery path for auth-disabled mode. Turning login off
// and then losing the browser session leaves nobody holding a real admin
// session, and the HTTP toggle deliberately requires one — so re-enabling has
// to be reachable from the shell on the box.
var systemAuthCmd = &cobra.Command{
	Use:   "auth [on|off]",
	Short: "Show or set whether browser login is required",
	Long: `Show or set whether browser login is required.

With no argument, prints the current setting. This is the recovery path when
login has been disabled and no admin session is available to turn it back on.

Examples:
  audiod system auth      # show current setting
  audiod system auth on   # require login
  audiod system auth off  # disable login`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		service := settings.NewService(settings.NewRepository(db))

		if len(args) == 0 {
			enabled, err := service.IsAuthEnabled()
			if err != nil {
				return fmt.Errorf("failed to read auth setting: %w", err)
			}
			fmt.Printf("Login required: %t\n", enabled)
			return nil
		}

		var enabled bool
		switch args[0] {
		case "on", "true", "enabled":
			enabled = true
		case "off", "false", "disabled":
			enabled = false
		default:
			return fmt.Errorf("invalid value %q: use \"on\" or \"off\"", args[0])
		}

		if err := service.SetAuthEnabled(enabled); err != nil {
			return fmt.Errorf("failed to save auth setting: %w", err)
		}
		if enabled {
			fmt.Println("✓ Login is now required")
		} else {
			fmt.Println("✓ Login is now disabled")
		}
		return nil
	},
}

func init() {
	systemCmd.AddCommand(systemResetCmd)
	systemCmd.AddCommand(systemAuthCmd)

	// Flags for system reset
	systemResetCmd.Flags().Bool("confirm", false, "Skip confirmation prompt (use with caution)")
}

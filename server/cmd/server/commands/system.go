package commands

import (
	"audiod/internal/branding"
	"audiod/internal/cli"
	"audiod/internal/config"
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

func init() {
	systemCmd.AddCommand(systemResetCmd)

	// Flags for system reset
	systemResetCmd.Flags().Bool("confirm", false, "Skip confirmation prompt (use with caution)")
}

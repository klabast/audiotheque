package commands

import (
	"database/sql"
	"fmt"

	"audiod/internal/branding"
	"audiod/internal/database"

	"github.com/spf13/cobra"
)

var (
	db *sql.DB // Shared database connection for all commands
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "audiod",
	Short: branding.AppName + " — " + branding.AppTagline,
	Long: branding.AppName + " is a self-hosted music streaming server with a focus on hi-res audio,\n" +
		"offline sync, and MPD integration.\n\n" +
		"By default, running 'audiod' without any commands will start the HTTP server.\n" +
		"Use subcommands for administrative tasks and management.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default behavior: start the server
		return startServer()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Open database connection for all commands (except server which handles it separately)
		if cmd.Use != "audiod" {
			var err error
			db, err = database.Open()
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		// Close database connection after command completes
		if db != nil {
			return db.Close()
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Add subcommands
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(libraryCmd)
	rootCmd.AddCommand(userCmd)
	rootCmd.AddCommand(systemCmd)
	rootCmd.AddCommand(deviceCmd)

	// Add completion command
	rootCmd.AddCommand(completionCmd)
}

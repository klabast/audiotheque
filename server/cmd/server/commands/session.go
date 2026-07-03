package commands

import (
	"audiod/internal/auth"
	"audiod/internal/branding"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// sessionCmd is the umbrella for session-management subcommands. The only
// subcommand today is `expire-soon`, which exists so the e2e suite can
// simulate "session past halfway through its window" without sleeping
// half a TTL. Production users will never run these.
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Session management commands (mostly test fixtures)",
	Long:  "Manage " + branding.AppName + " browser sessions. Primarily used by the E2E suite.",
}

// sessionExpireSoonCmd backdates the user's sessions so that the next
// authenticated request triggers sliding-renewal in Service.ValidateSession.
// Used by the @sliding-renewal e2e scenario: log in, then run this CLI, then
// browse — the cookie expiry should jump back up to the full window.
var sessionExpireSoonCmd = &cobra.Command{
	Use:   "expire-soon",
	Short: "Force a user's sessions to expire in the near future (test fixture)",
	Long: `Set expires_at on every session for the named user to now + 1 minute.

This puts the session well past the halfway point of either the 30-day
default or 90-day remember-me window, so the next authenticated request
fires sliding-renewal. Intended for the e2e suite — production code paths
never need to call this.

Example:
  audiod session expire-soon --username alice`,
	RunE: func(cmd *cobra.Command, args []string) error {
		username, _ := cmd.Flags().GetString("username")
		if username == "" {
			return fmt.Errorf("--username is required")
		}

		repo := auth.NewRepository(db)
		user, err := repo.GetByUsername(username)
		if err != nil {
			return fmt.Errorf("look up user %q: %w", username, err)
		}

		sessions := auth.NewSessionRepository(db)
		soon := time.Now().UTC().Add(1 * time.Minute)
		if err := sessions.SetExpiryForUser(user.ID, soon); err != nil {
			return fmt.Errorf("update sessions: %w", err)
		}
		fmt.Printf("✓ Sessions for %q now expire at %s\n", username, soon.Format(time.RFC3339))
		return nil
	},
}

func init() {
	sessionCmd.AddCommand(sessionExpireSoonCmd)
	sessionExpireSoonCmd.Flags().StringP("username", "u", "", "Username whose sessions to bump")
}

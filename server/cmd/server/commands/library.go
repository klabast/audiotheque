package commands

import (
	"audiod/internal/auth"
	"audiod/internal/cli"
	"audiod/internal/library"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var libraryCmd = &cobra.Command{
	Use:   "library",
	Short: "Library management commands",
	Long:  `Manage music libraries: create, list, scan, delete, and modify library paths.`,
}

var libraryCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new music library",
	Long: `Create a new music library with one or more filesystem paths.

You can specify multiple paths using the --path flag multiple times.
Authentication is required (admin only).

Examples:
  audiod library create --name "Music" --path "/music" --user alice --password secret
  audiod library create -n "Jazz" -p "/jazz" -p "/jazz2" -u alice -p secret
  audiod library create  # Interactive mode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		paths, _ := cmd.Flags().GetStringSlice("path")
		username, _ := cmd.Flags().GetString("user")
		password, _ := cmd.Flags().GetString("password")

		// Prompt for missing values
		if name == "" {
			name = cli.PromptForInput("Library name: ")
		}
		if len(paths) == 0 {
			paths = cli.PromptForPaths()
		}
		if username == "" {
			username = cli.PromptForInput("Username: ")
		}
		if password == "" {
			password = cli.PromptForPassword("Password: ")
		}

		// Validate inputs
		if name == "" {
			return fmt.Errorf("library name is required")
		}
		if len(paths) == 0 {
			return fmt.Errorf("at least one library path is required")
		}
		if username == "" {
			return fmt.Errorf("username is required for authentication")
		}
		if password == "" {
			return fmt.Errorf("password is required for authentication")
		}

		// Authenticate user
		authRepo := auth.NewRepository(db)
		authService := auth.NewService(authRepo, nil)
		authenticatedUser, err := authService.Authenticate(username, password)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Check admin permissions
		if !authenticatedUser.IsAdmin {
			return fmt.Errorf("only admin users can create libraries")
		}

		// Create library
		repo := library.NewRepository(db)
		service := library.NewService(repo)
		lib, err := service.CreateLibrary(authenticatedUser.ID, name, paths)
		if err != nil {
			return fmt.Errorf("failed to create library: %w", err)
		}

		fmt.Printf("✓ Library created successfully\n")
		fmt.Printf("  ID: %d\n", lib.ID)
		fmt.Printf("  Name: %s\n", lib.Name)
		fmt.Printf("  Paths:\n")
		for _, path := range lib.Paths {
			fmt.Printf("    - %s\n", path)
		}
		return nil
	},
}

var libraryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all libraries",
	Long: `List all music libraries accessible to the authenticated user.

Authentication is required.

Examples:
  audiod library list --user alice --password secret
  audiod library list  # Interactive mode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		username, _ := cmd.Flags().GetString("user")
		password, _ := cmd.Flags().GetString("password")

		// Prompt for missing credentials
		if username == "" {
			username = cli.PromptForInput("Username: ")
		}
		if password == "" {
			password = cli.PromptForPassword("Password: ")
		}

		// Validate inputs
		if username == "" {
			return fmt.Errorf("username is required for authentication")
		}
		if password == "" {
			return fmt.Errorf("password is required for authentication")
		}

		// Authenticate user
		authRepo := auth.NewRepository(db)
		authService := auth.NewService(authRepo, nil)
		authenticatedUser, err := authService.Authenticate(username, password)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// List libraries
		repo := library.NewRepository(db)
		service := library.NewService(repo)
		libraries, err := service.ListLibraries(authenticatedUser.ID)
		if err != nil {
			return fmt.Errorf("failed to list libraries: %w", err)
		}

		if len(libraries) == 0 {
			fmt.Println("No libraries found")
			return nil
		}

		fmt.Printf("Found %d library(ies):\n\n", len(libraries))
		for _, lib := range libraries {
			fmt.Printf("ID: %d\n", lib.ID)
			fmt.Printf("Name: %s\n", lib.Name)
			fmt.Printf("Paths:\n")
			for _, path := range lib.Paths {
				fmt.Printf("  - %s\n", path)
			}
			fmt.Println()
		}

		return nil
	},
}

var libraryScanCmd = &cobra.Command{
	Use:   "scan [library-id]",
	Short: "Scan a music library",
	Long: `Start scanning a library for new/modified audio files.

With --follow flag, continuously displays scan progress until completion.
Without --follow, starts the scan and exits immediately.

Authentication is required.

Examples:
  audiod library scan 1 --user alice --password secret
  audiod library scan 1 --follow --user alice --password secret
  audiod library scan 1  # Interactive mode`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		libraryID := args[0]
		username, _ := cmd.Flags().GetString("user")
		password, _ := cmd.Flags().GetString("password")
		follow, _ := cmd.Flags().GetBool("follow")

		// Prompt for missing credentials
		if username == "" {
			username = cli.PromptForInput("Username: ")
		}
		if password == "" {
			password = cli.PromptForPassword("Password: ")
		}

		// Validate inputs
		if username == "" {
			return fmt.Errorf("username is required for authentication")
		}
		if password == "" {
			return fmt.Errorf("password is required for authentication")
		}

		// Authenticate user
		authRepo := auth.NewRepository(db)
		authService := auth.NewService(authRepo, nil)
		_, err := authService.Authenticate(username, password)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Parse library ID
		var id int64
		_, err = fmt.Sscanf(libraryID, "%d", &id)
		if err != nil {
			return fmt.Errorf("invalid library ID: %s", libraryID)
		}

		// Start scan
		repo := library.NewRepository(db)
		service := library.NewService(repo)
		err = service.StartScan(id)
		if err == library.ErrScanAlreadyInProgress {
			fmt.Println("⚠ Scan already in progress")
			if !follow {
				return nil
			}
		} else if err != nil {
			return fmt.Errorf("failed to start scan: %w", err)
		} else {
			fmt.Printf("✓ Scan started for library %d\n", id)
		}

		// Follow progress if requested
		if follow {
			return followScanProgress(service, id)
		}

		return nil
	},
}

func followScanProgress(service *library.Service, libraryID int64) error {
	fmt.Println("\nScanning...")
	for {
		progress, err := service.GetScanProgress(libraryID)
		if err == library.ErrNoScanInProgress {
			fmt.Println("\n✓ Scan completed")
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to get scan progress: %w", err)
		}

		// Display progress
		if progress.TotalFiles > 0 {
			percentage := float64(progress.ProcessedFiles) / float64(progress.TotalFiles) * 100
			fmt.Printf("\rProgress: %d/%d files (%.1f%%) | Added: %d | Updated: %d | Errors: %d",
				progress.ProcessedFiles, progress.TotalFiles, percentage,
				progress.TracksAdded, progress.TracksUpdated, progress.Errors)
		} else {
			fmt.Printf("\rProcessed: %d files | Added: %d | Updated: %d | Errors: %d",
				progress.ProcessedFiles, progress.TracksAdded, progress.TracksUpdated, progress.Errors)
		}

		// Check if completed
		if progress.Status == "completed" || progress.Status == "failed" {
			fmt.Printf("\n\n✓ Scan %s\n", progress.Status)
			fmt.Printf("  Files processed: %d\n", progress.ProcessedFiles)
			fmt.Printf("  Tracks added: %d\n", progress.TracksAdded)
			fmt.Printf("  Tracks updated: %d\n", progress.TracksUpdated)
			if progress.Errors > 0 {
				fmt.Printf("  Errors: %d\n", progress.Errors)
			}
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
}

var libraryDeleteCmd = &cobra.Command{
	Use:   "delete [library-id]",
	Short: "Delete a music library",
	Long: `Delete a music library and all its associated data.

This removes the library configuration and all indexed tracks, albums, and artists.
Authentication is required (admin only).

Examples:
  audiod library delete 1 --user alice --password secret
  audiod library delete 1  # Interactive mode`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		libraryID := args[0]
		username, _ := cmd.Flags().GetString("user")
		password, _ := cmd.Flags().GetString("password")

		// Prompt for missing credentials
		if username == "" {
			username = cli.PromptForInput("Username: ")
		}
		if password == "" {
			password = cli.PromptForPassword("Password: ")
		}

		// Validate inputs
		if username == "" {
			return fmt.Errorf("username is required for authentication")
		}
		if password == "" {
			return fmt.Errorf("password is required for authentication")
		}

		// Authenticate user
		authRepo := auth.NewRepository(db)
		authService := auth.NewService(authRepo, nil)
		authenticatedUser, err := authService.Authenticate(username, password)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Check admin permissions
		if !authenticatedUser.IsAdmin {
			return fmt.Errorf("only admin users can delete libraries")
		}

		// Parse library ID
		var id int64
		_, err = fmt.Sscanf(libraryID, "%d", &id)
		if err != nil {
			return fmt.Errorf("invalid library ID: %s", libraryID)
		}

		// Delete library
		repo := library.NewRepository(db)
		service := library.NewService(repo)
		err = service.DeleteLibrary(id)
		if err != nil {
			return fmt.Errorf("failed to delete library: %w", err)
		}

		fmt.Printf("✓ Library %d deleted successfully\n", id)
		return nil
	},
}

var libraryAccessCmd = &cobra.Command{
	Use:   "access",
	Short: "Manage library access",
	Long:  `Grant or revoke user access to libraries.`,
}

var libraryAccessGrantCmd = &cobra.Command{
	Use:   "grant",
	Short: "Grant a user access to a library",
	Long: `Grant a user access to a library.

Authentication is required (admin only).

Examples:
  audiod library access grant --library 1 --target-user bob --user alice --password secret`,
	RunE: func(cmd *cobra.Command, args []string) error {
		libraryID, _ := cmd.Flags().GetInt64("library")
		targetUser, _ := cmd.Flags().GetString("target-user")
		username, _ := cmd.Flags().GetString("user")
		password, _ := cmd.Flags().GetString("password")

		// Validate inputs
		if libraryID == 0 {
			return fmt.Errorf("library ID is required")
		}
		if targetUser == "" {
			return fmt.Errorf("target-user is required")
		}
		if username == "" {
			return fmt.Errorf("username is required for authentication")
		}
		if password == "" {
			return fmt.Errorf("password is required for authentication")
		}

		// Authenticate user
		authRepo := auth.NewRepository(db)
		authService := auth.NewService(authRepo, nil)
		authenticatedUser, err := authService.Authenticate(username, password)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Check admin permissions
		if !authenticatedUser.IsAdmin {
			return fmt.Errorf("only admin users can grant library access")
		}

		// Get target user ID
		targetUserRecord, err := authRepo.GetByUsername(targetUser)
		if err != nil {
			return fmt.Errorf("user not found: %s", targetUser)
		}

		// Grant access
		repo := library.NewRepository(db)
		err = repo.GrantAccess(targetUserRecord.ID, libraryID)
		if err != nil {
			return fmt.Errorf("failed to grant access: %w", err)
		}

		fmt.Printf("✓ Granted user %q access to library %d\n", targetUser, libraryID)
		return nil
	},
}

var libraryAccessRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke a user's access to a library",
	Long: `Revoke a user's access to a library.

Authentication is required (admin only).

Examples:
  audiod library access revoke --library 1 --target-user bob --user alice --password secret`,
	RunE: func(cmd *cobra.Command, args []string) error {
		libraryID, _ := cmd.Flags().GetInt64("library")
		targetUser, _ := cmd.Flags().GetString("target-user")
		username, _ := cmd.Flags().GetString("user")
		password, _ := cmd.Flags().GetString("password")

		// Validate inputs
		if libraryID == 0 {
			return fmt.Errorf("library ID is required")
		}
		if targetUser == "" {
			return fmt.Errorf("target-user is required")
		}
		if username == "" {
			return fmt.Errorf("username is required for authentication")
		}
		if password == "" {
			return fmt.Errorf("password is required for authentication")
		}

		// Authenticate user
		authRepo := auth.NewRepository(db)
		authService := auth.NewService(authRepo, nil)
		authenticatedUser, err := authService.Authenticate(username, password)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Check admin permissions
		if !authenticatedUser.IsAdmin {
			return fmt.Errorf("only admin users can revoke library access")
		}

		// Get target user ID
		targetUserRecord, err := authRepo.GetByUsername(targetUser)
		if err != nil {
			return fmt.Errorf("user not found: %s", targetUser)
		}

		// Revoke access
		repo := library.NewRepository(db)
		err = repo.RevokeAccess(targetUserRecord.ID, libraryID)
		if err != nil {
			return fmt.Errorf("failed to revoke access: %w", err)
		}

		fmt.Printf("✓ Revoked user %q access to library %d\n", targetUser, libraryID)
		return nil
	},
}

func init() {
	libraryCmd.AddCommand(libraryCreateCmd)
	libraryCmd.AddCommand(libraryListCmd)
	libraryCmd.AddCommand(libraryScanCmd)
	libraryCmd.AddCommand(libraryDeleteCmd)
	libraryCmd.AddCommand(libraryAccessCmd)

	libraryAccessCmd.AddCommand(libraryAccessGrantCmd)
	libraryAccessCmd.AddCommand(libraryAccessRevokeCmd)

	// Flags for library create
	libraryCreateCmd.Flags().StringP("name", "n", "", "Library name")
	libraryCreateCmd.Flags().StringSliceP("path", "p", []string{}, "Library paths (can be specified multiple times)")
	libraryCreateCmd.Flags().StringP("user", "u", "", "Username for authentication")
	libraryCreateCmd.Flags().String("password", "", "Password for authentication")

	// Flags for library list
	libraryListCmd.Flags().StringP("user", "u", "", "Username for authentication")
	libraryListCmd.Flags().String("password", "", "Password for authentication")

	// Flags for library scan
	libraryScanCmd.Flags().StringP("user", "u", "", "Username for authentication")
	libraryScanCmd.Flags().String("password", "", "Password for authentication")
	libraryScanCmd.Flags().BoolP("follow", "f", false, "Follow scan progress until completion")

	// Flags for library delete
	libraryDeleteCmd.Flags().StringP("user", "u", "", "Username for authentication")
	libraryDeleteCmd.Flags().String("password", "", "Password for authentication")

	// Flags for library access grant
	libraryAccessGrantCmd.Flags().Int64P("library", "l", 0, "Library ID")
	libraryAccessGrantCmd.Flags().StringP("target-user", "t", "", "Username to grant access to")
	libraryAccessGrantCmd.Flags().StringP("user", "u", "", "Username for authentication")
	libraryAccessGrantCmd.Flags().String("password", "", "Password for authentication")

	// Flags for library access revoke
	libraryAccessRevokeCmd.Flags().Int64P("library", "l", 0, "Library ID")
	libraryAccessRevokeCmd.Flags().StringP("target-user", "t", "", "Username to revoke access from")
	libraryAccessRevokeCmd.Flags().StringP("user", "u", "", "Username for authentication")
	libraryAccessRevokeCmd.Flags().String("password", "", "Password for authentication")
}

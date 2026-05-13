package commands

import (
	"audiod/internal/auth"
	"audiod/internal/cli"
	"audiod/internal/settings"
	"fmt"

	"github.com/spf13/cobra"
)

var deviceCmd = &cobra.Command{
	Use:   "device",
	Short: "Device management commands",
	Long:  `Manage playback devices: create, list, update, and delete MPD devices.`,
}

var deviceCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new playback device",
	Long: `Create a new MPD playback device.

Authentication is required (admin only).

Examples:
  audiod device create --name "Living Room" --type mpd --address "192.168.1.10:6600" --user alice --password secret
  audiod device create -n "Kitchen" -a "192.168.1.11:6600" -u alice --password secret
  audiod device create  # Interactive mode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		deviceType, _ := cmd.Flags().GetString("type")
		address, _ := cmd.Flags().GetString("address")
		username, _ := cmd.Flags().GetString("user")
		password, _ := cmd.Flags().GetString("password")

		// Prompt for missing values
		if name == "" {
			name = cli.PromptForInput("Device name: ")
		}
		if address == "" {
			address = cli.PromptForInput("MPD address (host:port): ")
		}
		if username == "" {
			username = cli.PromptForInput("Username: ")
		}
		if password == "" {
			password = cli.PromptForPassword("Password: ")
		}

		// Validate inputs
		if name == "" {
			return fmt.Errorf("device name is required")
		}
		if address == "" {
			return fmt.Errorf("device address is required")
		}
		if username == "" {
			return fmt.Errorf("username is required for authentication")
		}
		if password == "" {
			return fmt.Errorf("password is required for authentication")
		}

		// Authenticate user
		authRepo := auth.NewRepository(db)
		authService := auth.NewService(authRepo)
		_, authenticatedUser, err := authService.Login(username, password)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Check admin permissions
		if !authenticatedUser.IsAdmin {
			return fmt.Errorf("only admin users can create devices")
		}

		// Create device
		repo := settings.NewRepository(db)
		service := settings.NewService(repo)
		device, err := service.CreateDevice(name, deviceType, address)
		if err != nil {
			return fmt.Errorf("failed to create device: %w", err)
		}

		fmt.Printf("✓ Device created successfully\n")
		fmt.Printf("  ID: %s\n", device.ID)
		fmt.Printf("  Name: %s\n", device.Name)
		fmt.Printf("  Type: %s\n", device.Type)
		fmt.Printf("  Address: %s\n", device.Address)
		return nil
	},
}

var deviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all playback devices",
	Long: `List all configured playback devices.

Authentication is required (admin only).

Examples:
  audiod device list --user alice --password secret
  audiod device list  # Interactive mode`,
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
		authService := auth.NewService(authRepo)
		_, authenticatedUser, err := authService.Login(username, password)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Check admin permissions
		if !authenticatedUser.IsAdmin {
			return fmt.Errorf("only admin users can list devices")
		}

		// List devices
		repo := settings.NewRepository(db)
		service := settings.NewService(repo)
		devices, err := service.ListDevices()
		if err != nil {
			return fmt.Errorf("failed to list devices: %w", err)
		}

		if len(devices) == 0 {
			fmt.Println("No devices found")
			return nil
		}

		fmt.Printf("Found %d device(s):\n\n", len(devices))
		for _, device := range devices {
			fmt.Printf("ID: %s\n", device.ID)
			fmt.Printf("Name: %s\n", device.Name)
			fmt.Printf("Type: %s\n", device.Type)
			fmt.Printf("Address: %s\n", device.Address)
			fmt.Println()
		}

		return nil
	},
}

var deviceUpdateCmd = &cobra.Command{
	Use:   "update [device-id]",
	Short: "Update a playback device",
	Long: `Update an existing playback device's name or address.

Authentication is required (admin only).

Examples:
  audiod device update abc-123 --name "New Name" --user alice --password secret
  audiod device update abc-123 --address "192.168.1.20:6600" -u alice --password secret`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceID := args[0]
		name, _ := cmd.Flags().GetString("name")
		address, _ := cmd.Flags().GetString("address")
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
		authService := auth.NewService(authRepo)
		_, authenticatedUser, err := authService.Login(username, password)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Check admin permissions
		if !authenticatedUser.IsAdmin {
			return fmt.Errorf("only admin users can update devices")
		}

		// Update device
		repo := settings.NewRepository(db)
		service := settings.NewService(repo)
		_, err = service.UpdateDevice(deviceID, name, address)
		if err != nil {
			return fmt.Errorf("failed to update device: %w", err)
		}

		fmt.Printf("✓ Device %s updated successfully\n", deviceID)
		return nil
	},
}

var deviceDeleteCmd = &cobra.Command{
	Use:   "delete [device-id]",
	Short: "Delete a playback device",
	Long: `Delete an existing playback device.

Authentication is required (admin only).

Examples:
  audiod device delete abc-123 --user alice --password secret
  audiod device delete abc-123  # Interactive mode`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceID := args[0]
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
		authService := auth.NewService(authRepo)
		_, authenticatedUser, err := authService.Login(username, password)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Check admin permissions
		if !authenticatedUser.IsAdmin {
			return fmt.Errorf("only admin users can delete devices")
		}

		// Delete device
		repo := settings.NewRepository(db)
		service := settings.NewService(repo)
		err = service.DeleteDevice(deviceID)
		if err != nil {
			return fmt.Errorf("failed to delete device: %w", err)
		}

		fmt.Printf("✓ Device %s deleted successfully\n", deviceID)
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceCreateCmd)
	deviceCmd.AddCommand(deviceListCmd)
	deviceCmd.AddCommand(deviceUpdateCmd)
	deviceCmd.AddCommand(deviceDeleteCmd)

	// Flags for device create
	deviceCreateCmd.Flags().StringP("name", "n", "", "Device name")
	deviceCreateCmd.Flags().StringP("type", "t", "mpd", "Device type (default: mpd)")
	deviceCreateCmd.Flags().StringP("address", "a", "", "Device address (host:port)")
	deviceCreateCmd.Flags().StringP("user", "u", "", "Username for authentication")
	deviceCreateCmd.Flags().String("password", "", "Password for authentication")

	// Flags for device list
	deviceListCmd.Flags().StringP("user", "u", "", "Username for authentication")
	deviceListCmd.Flags().String("password", "", "Password for authentication")

	// Flags for device update
	deviceUpdateCmd.Flags().StringP("name", "n", "", "New device name")
	deviceUpdateCmd.Flags().StringP("address", "a", "", "New device address (host:port)")
	deviceUpdateCmd.Flags().StringP("user", "u", "", "Username for authentication")
	deviceUpdateCmd.Flags().String("password", "", "Password for authentication")

	// Flags for device delete
	deviceDeleteCmd.Flags().StringP("user", "u", "", "Username for authentication")
	deviceDeleteCmd.Flags().String("password", "", "Password for authentication")
}

package auth

import (
	"database/sql"
	"fmt"
)

// CreateUserCommand creates a user via CLI
// Usage: audiod user create --username <name> --password <pass> [--admin]
//
// In setup mode (no users exist): --admin flag is required (first user must be admin)
// In normal mode (users exist): --admin flag is optional (defaults to false)
func CreateUserCommand(db *sql.DB, username, password string, isAdmin bool) error {
	// Create service (follows layered architecture)
	repo := NewRepository(db)
	service := NewService(repo)

	// Call service layer (business logic handles setup vs normal mode)
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
}

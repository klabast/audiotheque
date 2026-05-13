package auth

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"audiod/internal/config"
)

// Repository defines the interface for user data access
type Repository interface {
	GetByUsername(username string) (*User, error)
	GetByID(id int64) (*User, error)
	GetUserCount() (int, error)
	GetAdminCount() (int, error)
	Create(username, passwordHash string, isAdmin bool) (*User, error)
	UpdatePassword(userID int64, newPasswordHash string) error

	// Reset code management
	StoreResetCode(code string, userID int64, expiresAt time.Time) error
	GetResetCode(code string) (*ResetCode, error)
	DeleteResetCode(code string) error
	DeleteResetCodesByUserID(userID int64) error
	DeleteExpiredResetCodes() error
}

// Service handles authentication business logic
type Service struct {
	repo Repository
}

// NewService creates a new auth service
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Login validates credentials and returns a JWT token and user
func (s *Service) Login(username, password string) (string, *User, error) {
	// Get user from repository
	user, err := s.repo.GetByUsername(username)
	if err != nil {
		return "", nil, ErrInvalidPassword
	}

	// Verify password
	valid, err := VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return "", nil, err
	}

	if !valid {
		return "", nil, ErrInvalidPassword
	}

	// Generate JWT token
	token, err := GenerateToken(user.ID, user.Username)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}

// GetUserByID retrieves a user by ID
func (s *Service) GetUserByID(id int64) (*User, error) {
	return s.repo.GetByID(id)
}

// GetUserCount returns the total number of users in the system
func (s *Service) GetUserCount() (int, error) {
	return s.repo.GetUserCount()
}

func (s *Service) DoesAdminUserExist() (bool, error) {
	count, err := s.repo.GetAdminCount()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateFirstUser creates the first user (admin) account
// Returns an error if users already exist
func (s *Service) CreateFirstUser(username, password string) (string, *User, error) {
	// Check if any users exist
	count, err := s.repo.GetUserCount()
	if err != nil {
		return "", nil, err
	}

	if count > 0 {
		return "", nil, fmt.Errorf("setup already completed: users already exist")
	}

	// Hash the password
	passwordHash, err := HashPassword(password)
	if err != nil {
		return "", nil, err
	}

	// Create user with is_admin=true
	user, err := s.repo.Create(username, passwordHash, true)
	if err != nil {
		return "", nil, err
	}

	// Generate JWT token for auto-login
	token, err := GenerateToken(user.ID, user.Username)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}

// CreateUser creates a new user account
// In setup mode (no users exist): requires isAdmin=true, creates first admin
// In normal mode (users exist): creates user with specified isAdmin flag
func (s *Service) CreateUser(username, password string, isAdmin bool) (*User, error) {
	// Check if we're in setup mode
	count, err := s.repo.GetUserCount()
	if err != nil {
		return nil, err
	}

	// Setup mode: first user must be admin
	if count == 0 {
		if !isAdmin {
			return nil, fmt.Errorf("first user must be an admin")
		}
	}

	// Hash the password
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	// Create user
	user, err := s.repo.Create(username, passwordHash, isAdmin)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// UpdatePassword updates a user's password after verifying the current password
func (s *Service) UpdatePassword(userID int64, currentPassword, newPassword string) error {
	// Get the user
	user, err := s.repo.GetByID(userID)
	if err != nil {
		return err
	}

	// Verify current password
	valid, err := VerifyPassword(currentPassword, user.PasswordHash)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidPassword
	}

	// Hash the new password
	newPasswordHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	// Update in repository
	return s.repo.UpdatePassword(userID, newPasswordHash)
}

// RequestPasswordReset generates a reset code for the specified user
func (s *Service) RequestPasswordReset(username string) (string, error) {
	// Get the user
	user, err := s.repo.GetByUsername(username)
	if err != nil {
		return "", err
	}

	// Generate crypto-secure 8-character Base32 code
	code, err := generateResetCode()
	if err != nil {
		return "", err
	}

	// Delete any existing reset codes for this user
	err = s.repo.DeleteResetCodesByUserID(user.ID)
	if err != nil {
		return "", err
	}

	// Store new code with 30-minute expiration
	expiresAt := time.Now().Add(30 * time.Minute)
	err = s.repo.StoreResetCode(code, user.ID, expiresAt)
	if err != nil {
		return "", err
	}

	return code, nil
}

// RequestPasswordResetWithFile generates a reset code, writes it to a file, and logs to console
func (s *Service) RequestPasswordResetWithFile(username string) (string, string, error) {
	// Generate the reset code using existing method
	code, err := s.RequestPasswordReset(username)
	if err != nil {
		return "", "", err
	}

	// Get user info
	user, err := s.repo.GetByUsername(username)
	if err != nil {
		return "", "", err
	}

	// Get data directory from central config
	dataDir := config.GetDataDir()

	// Create reset_codes subdirectory
	resetCodesDir := filepath.Join(dataDir, "reset_codes")
	if err := os.MkdirAll(resetCodesDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create reset codes directory: %w", err)
	}

	// Create filename with unix timestamp
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%d_pw_reset_code_%s.json", timestamp, user.Username)
	filePath := filepath.Join(resetCodesDir, filename)

	// Create file content
	fileContent := map[string]interface{}{
		"code":       code,
		"username":   user.Username,
		"created_at": time.Now().Format(time.RFC3339),
		"expires_at": time.Now().Add(30 * time.Minute).Format(time.RFC3339),
	}

	// Write to file
	fileData, err := json.MarshalIndent(fileContent, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal reset code data: %w", err)
	}

	if err := os.WriteFile(filePath, fileData, 0600); err != nil {
		return "", "", fmt.Errorf("failed to write reset code file: %w", err)
	}

	// Log to console
	log.Printf("========================================")
	log.Printf("PASSWORD RESET CODE GENERATED")
	log.Printf("========================================")
	log.Printf("Username: %s", user.Username)
	log.Printf("Code: %s", code)
	log.Printf("File: %s", filePath)
	log.Printf("Expires: %s (30 minutes)", time.Now().Add(30*time.Minute).Format(time.RFC3339))
	log.Printf("========================================")

	return filePath, user.Username, nil
}

// ConfirmPasswordReset validates a reset code and sets a new password for the user
func (s *Service) ConfirmPasswordReset(code string, newPassword string) (*User, error) {
	// Get reset code from repository
	resetCode, err := s.repo.GetResetCode(code)
	if err != nil {
		return nil, err
	}

	// Check if code is expired
	if time.Now().After(resetCode.ExpiresAt) {
		return nil, ErrInvalidResetCode
	}

	// Get the user associated with this reset code
	user, err := s.repo.GetByID(resetCode.UserID)
	if err != nil {
		return nil, err
	}

	// Hash the new password
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return nil, err
	}

	// Update user's password
	err = s.repo.UpdatePassword(user.ID, passwordHash)
	if err != nil {
		return nil, err
	}

	// Delete the reset code (consumed)
	err = s.repo.DeleteResetCode(code)
	if err != nil {
		return nil, err
	}

	// Return the reset user
	return s.repo.GetByID(user.ID)
}

// CleanupExpiredResetCodes deletes all expired reset codes from the database
// This method is intended to be called periodically by a background job
func (s *Service) CleanupExpiredResetCodes() error {
	return s.repo.DeleteExpiredResetCodes()
}

// generateResetCode creates a crypto-secure 8-character Base32 code
func generateResetCode() (string, error) {
	// Generate 5 random bytes (40 bits)
	// Base32 encoding: 5 bits per character, so 40 bits = 8 characters
	b := make([]byte, 5)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	// Encode to Base32 (uppercase, no padding)
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	code := encoder.EncodeToString(b)

	// Take first 8 characters (should be exactly 8, but be defensive)
	if len(code) > 8 {
		code = code[:8]
	}

	return code, nil
}


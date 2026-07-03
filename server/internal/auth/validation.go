package auth

import "fmt"

// Credential validation rules, shared by every handler that accepts a
// username, password, or reset code (login, setup, password change/reset,
// admin user management). Centralized here so the rules live in exactly one
// place — see ui/src/lib/utils/validation.ts for the frontend mirror; the
// two must stay in sync.
const (
	// MinPasswordLength is 8. Below this, credential-set/reset endpoints
	// reject the password outright. assessPassword (frontend) separately
	// nudges users toward 12+ characters via a non-blocking warning — that
	// UX layer is independent of this hard floor.
	MinPasswordLength = 8
	MaxPasswordLength = 64

	MinUsernameLength = 2
	MaxUsernameLength = 32

	// ResetCodeLength is the fixed length of a password-reset code
	// (8-character Base32 — see generateResetCode in service.go).
	ResetCodeLength = 8
)

// ValidatePassword checks a candidate password against the length policy.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength || len(password) > MaxPasswordLength {
		return fmt.Errorf("password must be between %d and %d characters", MinPasswordLength, MaxPasswordLength)
	}
	return nil
}

// ValidateUsername checks a candidate username against the length policy.
func ValidateUsername(username string) error {
	if len(username) < MinUsernameLength || len(username) > MaxUsernameLength {
		return fmt.Errorf("username must be between %d and %d characters", MinUsernameLength, MaxUsernameLength)
	}
	return nil
}

// ValidateResetCode checks a candidate password-reset code against the
// expected fixed length.
func ValidateResetCode(code string) error {
	if len(code) != ResetCodeLength {
		return fmt.Errorf("reset code must be %d characters", ResetCodeLength)
	}
	return nil
}

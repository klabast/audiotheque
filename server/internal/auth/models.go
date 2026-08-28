package auth

import (
	"errors"
	"time"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrInvalidResetCode = errors.New("invalid or expired reset code")
	ErrSessionNotFound  = errors.New("session not found or expired")

	ErrSetupAlreadyCompleted = errors.New("setup already completed: users already exist")
	ErrUsernameTaken         = errors.New("username already taken")
	ErrFirstUserMustBeAdmin  = errors.New("first user must be an admin")
	ErrCannotDeleteSelf      = errors.New("cannot delete the currently signed-in user")
	ErrCannotDeleteLastAdmin = errors.New("cannot delete the last admin user")
	ErrRateLimited           = errors.New("too many attempts")
)

// SessionWindowDefault is how long a session lasts when "Keep me logged in"
// is unchecked. SessionWindowRemember is the longer window for the opt-in.
// Both renew on activity (sliding) — see Service.ValidateSession.
const (
	SessionWindowDefault  = 30 * 24 * time.Hour
	SessionWindowRemember = 90 * 24 * time.Hour
)

// ResetCodeTTL is how long a password-reset code (and its notification file)
// stays valid.
const ResetCodeTTL = 30 * time.Minute

// SessionWindowFor returns the lifetime of a session opened — or renewed —
// under the given "Keep me logged in" choice. Single source of truth for the
// cookie Max-Age, the session row's expires_at, and sliding renewal.
func SessionWindowFor(rememberMe bool) time.Duration {
	if rememberMe {
		return SessionWindowRemember
	}
	return SessionWindowDefault
}

// Session represents an active browser sign-in. The id is an opaque random
// value stored in the audiod_token cookie; the row is the source of truth
// for "is this cookie still valid". Logout deletes the row.
type Session struct {
	ID         string    `db:"id"`
	UserID     int64     `db:"user_id"`
	CreatedAt  time.Time `db:"created_at"`
	LastSeenAt time.Time `db:"last_seen_at"`
	ExpiresAt  time.Time `db:"expires_at"`
	RememberMe bool      `db:"remember_me"`
	UserAgent  string    `db:"user_agent"`
	LastIP     string    `db:"last_ip"`
}

// User represents a user in the system
type User struct {
	ID           int64     `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	PasswordHash string    `json:"-" db:"password_hash"`
	IsAdmin      bool      `json:"is_admin" db:"is_admin"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// UserResponse is the API response format (hides sensitive fields)
type UserResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		IsAdmin:   u.IsAdmin,
		CreatedAt: u.CreatedAt,
	}
}

// ResetCode represents a password reset code
type ResetCode struct {
	Code      string    `json:"code" db:"code"`
	UserID    int64     `json:"user_id" db:"user_id"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

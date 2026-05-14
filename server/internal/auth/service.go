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
	repo     Repository
	sessions SessionRepository
}

// NewService creates a new auth service. The session repository may be nil
// only when sessions are not needed (e.g. CLI commands that don't issue
// cookies) — every HTTP path that authenticates a browser request must
// construct the Service with a real SessionRepository.
func NewService(repo Repository, sessions SessionRepository) *Service {
	return &Service{repo: repo, sessions: sessions}
}

// SessionContext carries the request metadata recorded on a new session.
// All fields are optional — empty strings are stored as empty strings.
type SessionContext struct {
	RememberMe bool
	UserAgent  string
	IP         string
}

// Authenticate verifies credentials and returns the user without creating
// a session. CLI commands and other non-HTTP paths use this to gate admin
// operations on a username/password without writing a cookie-backed session.
func (s *Service) Authenticate(username, password string) (*User, error) {
	user, err := s.repo.GetByUsername(username)
	if err != nil {
		return nil, ErrInvalidPassword
	}
	valid, err := VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, ErrInvalidPassword
	}
	return user, nil
}

// Login validates credentials and returns an opaque session ID (cookie value)
// plus the authenticated user. The session row is persisted; logout/revoke
// drops it.
func (s *Service) Login(username, password string, ctx SessionContext) (string, *User, error) {
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

	// Open a session for this login
	sessionID, err := s.CreateSession(user.ID, ctx)
	if err != nil {
		return "", nil, err
	}

	return sessionID, user, nil
}

// CreateSession inserts a new session row for userID and returns the cookie
// value (the row's opaque id). Window is 30d by default, 90d when
// RememberMe is set — see SessionWindowDefault / SessionWindowRemember.
func (s *Service) CreateSession(userID int64, ctx SessionContext) (string, error) {
	id, err := generateSessionID()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	window := SessionWindowDefault
	if ctx.RememberMe {
		window = SessionWindowRemember
	}
	sess := &Session{
		ID:         id,
		UserID:     userID,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(window),
		RememberMe: ctx.RememberMe,
		UserAgent:  ctx.UserAgent,
		LastIP:     ctx.IP,
	}
	if err := s.sessions.Create(sess); err != nil {
		return "", err
	}
	return id, nil
}

// ValidateSession resolves a cookie value to its user and the (possibly
// renewed) session row. The session's last_seen_at and last_ip are updated
// on every successful call. When less than half the configured window
// remains, expires_at is bumped to now + window (sliding renewal). Callers
// that hold a ResponseWriter should re-Set-Cookie after this returns so
// the browser's cookie expiry tracks the bumped row.
//
// Returns ErrSessionNotFound for unknown or expired sessions, and lazily
// deletes the row when expired.
func (s *Service) ValidateSession(id, ip string) (*User, *Session, error) {
	if id == "" {
		return nil, nil, ErrSessionNotFound
	}
	sess, err := s.sessions.GetByID(id)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	if now.After(sess.ExpiresAt) {
		// Best-effort cleanup; ignore delete errors so we always return the
		// right auth signal to the caller.
		_ = s.sessions.Delete(id)
		return nil, nil, ErrSessionNotFound
	}

	// Sliding renewal: bump expires_at when less than half the window remains.
	// Window is determined per-session by remember_me so the two TTLs (30d /
	// 90d) renew consistently with the original login choice.
	window := SessionWindowDefault
	if sess.RememberMe {
		window = SessionWindowRemember
	}
	newExpiresAt := sess.ExpiresAt
	if sess.ExpiresAt.Sub(now) < window/2 {
		newExpiresAt = now.Add(window)
	}

	// Per §7 of the roadmap, every authenticated request updates last_seen_at
	// and last_ip. UpdateExpiry failures are logged but not fatal — auth
	// must still succeed so the user isn't kicked out by a transient DB hiccup.
	if err := s.sessions.UpdateExpiry(id, now, newExpiresAt, ip); err == nil {
		sess.LastSeenAt = now
		sess.ExpiresAt = newExpiresAt
		sess.LastIP = ip
	}

	user, err := s.repo.GetByID(sess.UserID)
	if err != nil {
		return nil, nil, err
	}
	return user, sess, nil
}

// DeleteSession removes a single session row (logout from one device).
func (s *Service) DeleteSession(id string) error {
	if id == "" {
		return nil
	}
	return s.sessions.Delete(id)
}

// ListUserSessions returns the user's active sessions, ordered most-recent
// first. Used by the Active Devices UI in Settings → Security.
func (s *Service) ListUserSessions(userID int64) ([]*Session, error) {
	return s.sessions.ListForUser(userID)
}

// DeleteUserSessionByPublicID revokes a single session of userID, identified
// by its API-safe public id (SHA-256 hash of the raw session id). Returns
// ErrSessionNotFound if no matching session belongs to userID — callers
// surface this as 404. We iterate the user's sessions rather than indexing
// by hash because typical N is small (one user, a handful of devices) and
// this avoids a new column / migration.
func (s *Service) DeleteUserSessionByPublicID(userID int64, publicID string) error {
	sessions, err := s.sessions.ListForUser(userID)
	if err != nil {
		return err
	}
	for _, sess := range sessions {
		if SessionPublicID(sess.ID) == publicID {
			return s.sessions.Delete(sess.ID)
		}
	}
	return ErrSessionNotFound
}

// DeleteOtherUserSessions revokes every session of userID except keepID
// ("log out of all other devices"). Cookie on the current browser stays
// valid; every other browser sees its next authenticated request rejected.
func (s *Service) DeleteOtherUserSessions(userID int64, keepID string) error {
	return s.sessions.DeleteAllForUserExcept(userID, keepID)
}

// DeleteAllUserSessions revokes every session of userID, including the
// caller's current one ("log out of all devices"). The HTTP handler should
// follow up by clearing the response cookie.
func (s *Service) DeleteAllUserSessions(userID int64) error {
	return s.sessions.DeleteAllForUser(userID)
}

// CleanupExpiredSessions drops session rows whose expires_at has passed.
// Intended for the background jobs runner.
func (s *Service) CleanupExpiredSessions() (int64, error) {
	return s.sessions.DeleteExpired(time.Now().UTC())
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

// CreateFirstUser creates the first user (admin) account and opens a session
// for them. Returns the session id (cookie value) plus the user.
// Errors if users already exist.
func (s *Service) CreateFirstUser(username, password string, ctx SessionContext) (string, *User, error) {
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

	// Open a session for the freshly-created admin
	sessionID, err := s.CreateSession(user.ID, ctx)
	if err != nil {
		return "", nil, err
	}

	return sessionID, user, nil
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


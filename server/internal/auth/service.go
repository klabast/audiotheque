package auth

import (
	"crypto/rand"
	"encoding/base32"
	"log"
	"sync"
	"time"
)

// Repository defines the interface for user data access
type Repository interface {
	GetByUsername(username string) (*User, error)
	GetByID(id int64) (*User, error)
	GetUserCount() (int, error)
	GetAdminCount() (int, error)
	// GetFirstAdmin returns the lowest-id admin user — the canonical "system"
	// admin that auth-disabled mode resolves every request to. Returns
	// ErrUserNotFound if no admin exists.
	GetFirstAdmin() (*User, error)
	// ListUsers returns every user row in id order. Feeds the admin Users
	// settings panel.
	ListUsers() ([]*User, error)
	Create(username, passwordHash string, isAdmin bool) (*User, error)
	// Delete removes a user row. Cascades to dependent rows (sessions,
	// reset codes, etc.) via FK ON DELETE CASCADE.
	Delete(userID int64) error
	UpdatePassword(userID int64, newPasswordHash string) error

	// Reset code management
	StoreResetCode(code string, userID int64, expiresAt time.Time) error
	GetResetCode(code string) (*ResetCode, error)
	DeleteResetCode(code string) error
	DeleteResetCodesByUserID(userID int64) error
	DeleteExpiredResetCodes() error
}

// firstAdminCreator inserts the initial admin only if the user table is still
// empty, in one statement. Kept off the Repository interface so the fakes in
// other packages keep compiling; the SQLite repository implements it, and
// CreateFirstUser falls back to the racy check-then-insert only for a
// repository that doesn't (test doubles).
type firstAdminCreator interface {
	CreateFirstAdmin(username, passwordHash string) (*User, error)
}

// Service handles authentication business logic
type Service struct {
	repo           Repository
	sessions       SessionRepository
	resetCodeFiles resetCodeFiles
	authEnabledFn  func() (bool, error)
}

// NewService creates a new auth service. The session repository may be nil
// only when sessions are not needed (e.g. CLI commands that don't issue
// cookies) — every HTTP path that authenticates a browser request must
// construct the Service with a real SessionRepository.
func NewService(repo Repository, sessions SessionRepository) *Service {
	return &Service{repo: repo, sessions: sessions, resetCodeFiles: newResetCodeFileStore()}
}

// SetAuthEnabledFn wires the settings-backed auth-enabled lookup. Kept as a
// setter rather than a constructor arg so the auth package stays free of any
// dependency on the settings package (cmd/server is the only thing that knows
// about both). When unset, AuthEnabled() returns true — the safe default.
func (s *Service) SetAuthEnabledFn(fn func() (bool, error)) {
	s.authEnabledFn = fn
}

// AuthEnabled reports whether this instance currently requires browser auth.
// Errors from the injected fn are treated as "enabled" so a DB hiccup never
// silently opens the app up.
func (s *Service) AuthEnabled() bool {
	if s.authEnabledFn == nil {
		return true
	}
	enabled, err := s.authEnabledFn()
	if err != nil {
		return true
	}
	return enabled
}

// GetCanonicalAdmin returns the lowest-id admin user. Auth-disabled mode
// resolves every authenticated handler to this user, so the rest of the
// system continues to see a User pointer (audit logs, ownership checks,
// etc.) without conditional plumbing.
func (s *Service) GetCanonicalAdmin() (*User, error) {
	return s.repo.GetFirstAdmin()
}

// SessionContext carries the request metadata recorded on a new session.
// All fields are optional — empty strings are stored as empty strings.
type SessionContext struct {
	RememberMe bool
	UserAgent  string
	IP         string
}

// decoyPasswordHash is a valid Argon2id hash of a value nobody can supply. An
// unknown username is verified against it so the response time of "no such
// user" matches "wrong password" instead of returning ~100ms sooner and
// confirming which usernames exist.
var decoyPasswordHash = sync.OnceValue(func() string {
	decoy, err := generateSessionID()
	if err != nil {
		return ""
	}
	hash, err := HashPassword(decoy)
	if err != nil {
		return ""
	}
	return hash
})

func verifyAgainstDecoy(password string) {
	if hash := decoyPasswordHash(); hash != "" {
		_, _ = VerifyPassword(password, hash)
	}
}

// Authenticate verifies credentials and returns the user without creating
// a session. CLI commands and other non-HTTP paths use this to gate admin
// operations on a username/password without writing a cookie-backed session.
func (s *Service) Authenticate(username, password string) (*User, error) {
	user, err := s.repo.GetByUsername(username)
	if err != nil {
		verifyAgainstDecoy(password)
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
	user, err := s.Authenticate(username, password)
	if err != nil {
		return "", nil, err
	}

	sessionID, err := s.CreateSession(user.ID, ctx)
	if err != nil {
		return "", nil, err
	}

	return sessionID, user, nil
}

// CreateSession inserts a new session row for userID and returns the cookie
// value (the row's opaque id). Window is 30d by default, 90d when
// RememberMe is set — see SessionWindowFor.
func (s *Service) CreateSession(userID int64, ctx SessionContext) (string, error) {
	id, err := generateSessionID()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	sess := &Session{
		ID:         id,
		UserID:     userID,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(SessionWindowFor(ctx.RememberMe)),
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
	window := SessionWindowFor(sess.RememberMe)
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

// SessionRememberMe reports the "keep me logged in" choice recorded on a
// session. Unknown sessions answer false — the shorter window.
func (s *Service) SessionRememberMe(id string) bool {
	if id == "" || s.sessions == nil {
		return false
	}
	sess, err := s.sessions.GetByID(id)
	if err != nil {
		return false
	}
	return sess.RememberMe
}

// DeleteSession removes a single session row (logout from one device).
func (s *Service) DeleteSession(id string) error {
	if id == "" {
		return nil
	}
	return s.sessions.Delete(id)
}

// ListUserSessions returns the user's unexpired sessions, ordered
// most-recent first. Used by the Active Devices UI in Settings → Security.
func (s *Service) ListUserSessions(userID int64) ([]*Session, error) {
	return s.sessions.ListForUser(userID, time.Now().UTC())
}

// DeleteUserSessionByPublicID revokes a single session of userID, identified
// by its API-safe public id (SHA-256 hash of the raw session id). Returns
// ErrSessionNotFound if no matching session belongs to userID — callers
// surface this as 404. We iterate the user's sessions rather than indexing
// by hash because typical N is small (one user, a handful of devices) and
// this avoids a new column / migration.
func (s *Service) DeleteUserSessionByPublicID(userID int64, publicID string) error {
	sessions, err := s.sessions.ListForUser(userID, time.Now().UTC())
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

// revokeAllSessions drops every session of userID. Tolerates the nil session
// repository that CLI-only Services are constructed with.
func (s *Service) revokeAllSessions(userID int64) error {
	if s.sessions == nil {
		return nil
	}
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
// Returns ErrSetupAlreadyCompleted if users already exist.
func (s *Service) CreateFirstUser(username, password string, ctx SessionContext) (string, *User, error) {
	if err := ValidateUsername(username); err != nil {
		return "", nil, err
	}
	if err := ValidatePassword(password); err != nil {
		return "", nil, err
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return "", nil, err
	}

	user, err := s.createFirstAdmin(username, passwordHash)
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

// createFirstAdmin inserts the initial admin. /api/auth/setup is
// unauthenticated and hashing takes ~100ms, so a check-then-insert leaves a
// wide window in which two requests both see an empty table and both succeed
// — the second one an attacker's admin. Repositories that can do the check
// and the insert in one statement do so.
func (s *Service) createFirstAdmin(username, passwordHash string) (*User, error) {
	if creator, ok := s.repo.(firstAdminCreator); ok {
		return creator.CreateFirstAdmin(username, passwordHash)
	}
	count, err := s.repo.GetUserCount()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSetupAlreadyCompleted
	}
	return s.repo.Create(username, passwordHash, true)
}

// ListUsers returns every user row, ordered by id. Caller is expected to
// gate this on admin via auth.RequireRealAdmin — the service itself doesn't
// know who's asking.
func (s *Service) ListUsers() ([]*User, error) {
	return s.repo.ListUsers()
}

// DeleteUser removes targetID. Refuses to delete the actor's own row and
// refuses to remove the system's last admin. Both checks happen here (in
// the service layer) so the handler can stay thin and the CLI gets the
// same guarantees for free.
func (s *Service) DeleteUser(actorID, targetID int64) error {
	if actorID == targetID {
		return ErrCannotDeleteSelf
	}
	target, err := s.repo.GetByID(targetID)
	if err != nil {
		return err
	}
	if target.IsAdmin {
		count, err := s.repo.GetAdminCount()
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrCannotDeleteLastAdmin
		}
	}
	return s.repo.Delete(targetID)
}

// AdminResetUserPassword replaces targetID's password without verifying the
// existing one — distinct from Service.UpdatePassword which requires the
// user's current password. The action is gated on a real admin session at the
// handler layer. Every session of the target is revoked: a password reset
// that leaves a stolen cookie working isn't a reset.
func (s *Service) AdminResetUserPassword(targetID int64, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	if _, err := s.repo.GetByID(targetID); err != nil {
		return err
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePassword(targetID, hash); err != nil {
		return err
	}
	return s.revokeAllSessions(targetID)
}

// CreateUser creates a new user account
// In setup mode (no users exist): requires isAdmin=true, creates first admin
// In normal mode (users exist): creates user with specified isAdmin flag
func (s *Service) CreateUser(username, password string, isAdmin bool) (*User, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}

	count, err := s.repo.GetUserCount()
	if err != nil {
		return nil, err
	}

	// Setup mode: first user must be admin
	if count == 0 && !isAdmin {
		return nil, ErrFirstUserMustBeAdmin
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	if count == 0 {
		return s.createFirstAdmin(username, passwordHash)
	}
	return s.repo.Create(username, passwordHash, isAdmin)
}

// UpdatePassword replaces a user's password after verifying the current one,
// revokes every session they had, and opens a fresh one for the caller. The
// returned session id belongs to that new session: the tab that made the
// change stays signed in while every other device — including one holding a
// stolen cookie — is logged out.
func (s *Service) UpdatePassword(userID int64, currentPassword, newPassword string, ctx SessionContext) (string, error) {
	if err := ValidatePassword(newPassword); err != nil {
		return "", err
	}

	user, err := s.repo.GetByID(userID)
	if err != nil {
		return "", err
	}

	valid, err := VerifyPassword(currentPassword, user.PasswordHash)
	if err != nil {
		return "", err
	}
	if !valid {
		return "", ErrInvalidPassword
	}

	newPasswordHash, err := HashPassword(newPassword)
	if err != nil {
		return "", err
	}

	if err := s.repo.UpdatePassword(userID, newPasswordHash); err != nil {
		return "", err
	}

	// A failed revoke is reported even though the password already changed:
	// the caller has to know that the old sessions may still be live.
	if err := s.revokeAllSessions(userID); err != nil {
		return "", err
	}
	if s.sessions == nil {
		return "", nil
	}
	return s.CreateSession(userID, ctx)
}

// RequestPasswordReset generates a reset code for the specified user
func (s *Service) RequestPasswordReset(username string) (string, error) {
	user, err := s.repo.GetByUsername(username)
	if err != nil {
		return "", err
	}

	code, err := generateResetCode()
	if err != nil {
		return "", err
	}

	// Only one code per user may be live at a time.
	if err := s.repo.DeleteResetCodesByUserID(user.ID); err != nil {
		return "", err
	}

	if err := s.repo.StoreResetCode(code, user.ID, time.Now().Add(ResetCodeTTL)); err != nil {
		return "", err
	}

	return code, nil
}

// RequestPasswordResetWithFile generates a reset code and writes it to a file
// the operator can read off the server. The code itself never reaches the log:
// it is password-equivalent, the endpoint that triggers it is unauthenticated,
// and logs are routinely readable by more people than the account is.
func (s *Service) RequestPasswordResetWithFile(username string) error {
	code, err := s.RequestPasswordReset(username)
	if err != nil {
		return err
	}

	user, err := s.repo.GetByUsername(username)
	if err != nil {
		return err
	}

	createdAt := time.Now()
	filePath, err := s.resetCodeFiles.Write(user.Username, code, createdAt)
	if err != nil {
		return err
	}

	log.Printf("Password reset code generated for %q - read it from %s (expires %s)",
		user.Username, filePath, createdAt.Add(ResetCodeTTL).Format(time.RFC3339))

	return nil
}

// ConfirmPasswordReset validates a reset code and sets a new password for the
// user, revoking every session they had.
func (s *Service) ConfirmPasswordReset(code string, newPassword string) (*User, error) {
	if err := ValidatePassword(newPassword); err != nil {
		return nil, err
	}

	resetCode, err := s.repo.GetResetCode(code)
	if err != nil {
		return nil, err
	}

	if time.Now().After(resetCode.ExpiresAt) {
		return nil, ErrInvalidResetCode
	}

	user, err := s.repo.GetByID(resetCode.UserID)
	if err != nil {
		return nil, err
	}

	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdatePassword(user.ID, passwordHash); err != nil {
		return nil, err
	}

	if err := s.revokeAllSessions(user.ID); err != nil {
		return nil, err
	}

	// The password is already changed by this point, so a failed delete must
	// not be reported as a failed reset. The code expires on its own, and the
	// cleanup job sweeps it.
	if err := s.repo.DeleteResetCode(code); err != nil {
		log.Printf("Failed to consume reset code for user %d: %v", user.ID, err)
	}

	return s.repo.GetByID(user.ID)
}

// CleanupExpiredResetCodes deletes expired reset codes — both the DB rows and
// the notification files, which nothing else removes between CLI resets.
// Intended to be called periodically by a background job.
func (s *Service) CleanupExpiredResetCodes() error {
	if err := s.repo.DeleteExpiredResetCodes(); err != nil {
		return err
	}
	if _, err := s.resetCodeFiles.DeleteExpired(time.Now()); err != nil {
		return err
	}
	return nil
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

	// Take first ResetCodeLength characters (should be exact already, but be defensive)
	if len(code) > ResetCodeLength {
		code = code[:ResetCodeLength]
	}

	return code, nil
}

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// SessionPublicID derives an API-safe identifier from a session ID by
// hashing it. Stable (same session.id → same public id) so it can be used
// in URLs without exposing the raw cookie value. Required because the
// session.id IS the cookie; returning it in JSON would let XSS steal it
// even though the cookie itself is HttpOnly. 128 bits of the SHA-256
// output gives plenty of collision resistance for a per-user list.
func SessionPublicID(sessionID string) string {
	h := sha256.Sum256([]byte(sessionID))
	return base64.RawURLEncoding.EncodeToString(h[:16])
}

// SessionIDByteLength is the entropy of a session ID before base64-encoding.
// 32 bytes = 256 bits of randomness; cookie value ends up ~43 chars.
const SessionIDByteLength = 32

// generateSessionID returns an opaque, URL-safe, unpadded base64 random ID
// suitable as a cookie value. Errors are bubbled up — crypto/rand failing
// is fatal-class, but we still let the caller decide.
func generateSessionID() (string, error) {
	buf := make([]byte, SessionIDByteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// SessionRepository is the DB-access contract for the session table.
// Service is the only caller; handlers and middleware go through Service.
type SessionRepository interface {
	Create(s *Session) error
	GetByID(id string) (*Session, error)
	UpdateExpiry(id string, lastSeenAt, expiresAt time.Time, lastIP string) error
	Delete(id string) error
	DeleteAllForUser(userID int64) error
	DeleteAllForUserExcept(userID int64, exceptID string) error
	// ListForUser returns the user's sessions that are still valid at now.
	// Expired rows survive until the cleanup job runs; they must never show
	// up as an active device.
	ListForUser(userID int64, now time.Time) ([]*Session, error)
	DeleteExpired(now time.Time) (int64, error)
	// SetExpiryForUser bulk-sets expires_at on every session belonging to
	// userID. Used by the `audiod session expire-soon` CLI fixture command
	// to simulate "session past halfway through its window" for the
	// @sliding-renewal e2e scenario without sleeping half a TTL.
	SetExpiryForUser(userID int64, expiresAt time.Time) error
}

// sessionRepository is the SQLite-backed implementation.
type sessionRepository struct {
	db *sql.DB
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(db *sql.DB) SessionRepository {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) Create(s *Session) error {
	//language=SQL
	query := `
INSERT INTO session (id, user_id, created_at, last_seen_at, expires_at, remember_me, user_agent, last_ip)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query, s.ID, s.UserID, s.CreatedAt, s.LastSeenAt, s.ExpiresAt, s.RememberMe, s.UserAgent, s.LastIP)
	if err != nil {
		return fmt.Errorf("session create: %w", err)
	}
	return nil
}

func (r *sessionRepository) GetByID(id string) (*Session, error) {
	//language=SQL
	query := `
SELECT id, user_id, created_at, last_seen_at, expires_at, remember_me, user_agent, last_ip
FROM session
WHERE id = ?`
	s := &Session{}
	err := r.db.QueryRow(query, id).Scan(
		&s.ID, &s.UserID, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt, &s.RememberMe, &s.UserAgent, &s.LastIP,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("session get: %w", err)
	}
	return s, nil
}

func (r *sessionRepository) UpdateExpiry(id string, lastSeenAt, expiresAt time.Time, lastIP string) error {
	//language=SQL
	query := `
UPDATE session
SET last_seen_at = ?, expires_at = ?, last_ip = ?
WHERE id = ?`
	_, err := r.db.Exec(query, lastSeenAt, expiresAt, lastIP, id)
	if err != nil {
		return fmt.Errorf("session update expiry: %w", err)
	}
	return nil
}

func (r *sessionRepository) Delete(id string) error {
	//language=SQL
	query := `DELETE FROM session WHERE id = ?`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("session delete: %w", err)
	}
	return nil
}

func (r *sessionRepository) DeleteAllForUser(userID int64) error {
	//language=SQL
	query := `DELETE FROM session WHERE user_id = ?`
	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("session delete all for user: %w", err)
	}
	return nil
}

func (r *sessionRepository) DeleteAllForUserExcept(userID int64, exceptID string) error {
	//language=SQL
	query := `DELETE FROM session WHERE user_id = ? AND id != ?`
	_, err := r.db.Exec(query, userID, exceptID)
	if err != nil {
		return fmt.Errorf("session delete all for user except: %w", err)
	}
	return nil
}

func (r *sessionRepository) ListForUser(userID int64, now time.Time) ([]*Session, error) {
	//language=SQL
	query := `
SELECT id, user_id, created_at, last_seen_at, expires_at, remember_me, user_agent, last_ip
FROM session
WHERE user_id = ?
  AND expires_at > ?
ORDER BY last_seen_at DESC`
	rows, err := r.db.Query(query, userID, now)
	if err != nil {
		return nil, fmt.Errorf("session list for user: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		s := &Session{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt, &s.RememberMe, &s.UserAgent, &s.LastIP); err != nil {
			return nil, fmt.Errorf("session list scan: %w", err)
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session list rows: %w", err)
	}
	return sessions, nil
}

func (r *sessionRepository) DeleteExpired(now time.Time) (int64, error) {
	//language=SQL
	query := `DELETE FROM session WHERE expires_at < ?`
	res, err := r.db.Exec(query, now)
	if err != nil {
		return 0, fmt.Errorf("session delete expired: %w", err)
	}
	return res.RowsAffected()
}

func (r *sessionRepository) SetExpiryForUser(userID int64, expiresAt time.Time) error {
	//language=SQL
	query := `UPDATE session SET expires_at = ? WHERE user_id = ?`
	_, err := r.db.Exec(query, expiresAt, userID)
	if err != nil {
		return fmt.Errorf("session set expiry for user: %w", err)
	}
	return nil
}

// inMemorySessionRepository is a SessionRepository backed by a Go map.
// Used by tests that mock the auth Repository — production code uses
// NewSessionRepository against the SQLite-backed DB.
type inMemorySessionRepository struct {
	sessions map[string]*Session
}

// NewInMemorySessionRepository returns a SessionRepository that keeps
// sessions in process memory. Safe for sequential test use; not goroutine-safe.
func NewInMemorySessionRepository() SessionRepository {
	return &inMemorySessionRepository{sessions: map[string]*Session{}}
}

func (m *inMemorySessionRepository) Create(s *Session) error {
	copy := *s
	m.sessions[s.ID] = &copy
	return nil
}

func (m *inMemorySessionRepository) GetByID(id string) (*Session, error) {
	s, ok := m.sessions[id]
	if !ok {
		return nil, ErrSessionNotFound
	}
	copy := *s
	return &copy, nil
}

func (m *inMemorySessionRepository) UpdateExpiry(id string, lastSeenAt, expiresAt time.Time, lastIP string) error {
	s, ok := m.sessions[id]
	if !ok {
		return ErrSessionNotFound
	}
	s.LastSeenAt = lastSeenAt
	s.ExpiresAt = expiresAt
	s.LastIP = lastIP
	return nil
}

func (m *inMemorySessionRepository) Delete(id string) error {
	delete(m.sessions, id)
	return nil
}

func (m *inMemorySessionRepository) DeleteAllForUser(userID int64) error {
	for id, s := range m.sessions {
		if s.UserID == userID {
			delete(m.sessions, id)
		}
	}
	return nil
}

func (m *inMemorySessionRepository) DeleteAllForUserExcept(userID int64, exceptID string) error {
	for id, s := range m.sessions {
		if s.UserID == userID && id != exceptID {
			delete(m.sessions, id)
		}
	}
	return nil
}

func (m *inMemorySessionRepository) ListForUser(userID int64, now time.Time) ([]*Session, error) {
	var sessions []*Session
	for _, s := range m.sessions {
		if s.UserID == userID && s.ExpiresAt.After(now) {
			copy := *s
			sessions = append(sessions, &copy)
		}
	}
	return sessions, nil
}

func (m *inMemorySessionRepository) DeleteExpired(now time.Time) (int64, error) {
	var n int64
	for id, s := range m.sessions {
		if s.ExpiresAt.Before(now) {
			delete(m.sessions, id)
			n++
		}
	}
	return n, nil
}

func (m *inMemorySessionRepository) SetExpiryForUser(userID int64, expiresAt time.Time) error {
	for _, s := range m.sessions {
		if s.UserID == userID {
			s.ExpiresAt = expiresAt
		}
	}
	return nil
}

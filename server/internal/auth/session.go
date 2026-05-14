package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

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
	DeleteExpired(now time.Time) (int64, error)
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

func (r *sessionRepository) DeleteExpired(now time.Time) (int64, error) {
	//language=SQL
	query := `DELETE FROM session WHERE expires_at < ?`
	res, err := r.db.Exec(query, now)
	if err != nil {
		return 0, fmt.Errorf("session delete expired: %w", err)
	}
	return res.RowsAffected()
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

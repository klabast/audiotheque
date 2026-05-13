package playback

import (
	"sync"
)

// InMemorySessionRepository stores sessions in memory
// For MVP - later can add DB persistence
type InMemorySessionRepository struct {
	sessions map[int64]*Session // userID -> session
	mu       sync.RWMutex
}

// NewInMemorySessionRepository creates a new in-memory session repository
func NewInMemorySessionRepository() *InMemorySessionRepository {
	return &InMemorySessionRepository{
		sessions: make(map[int64]*Session),
	}
}

// Save stores or updates a session
func (r *InMemorySessionRepository) Save(session *Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.UserID] = session
	return nil
}

// GetByUserID retrieves a session by user ID
func (r *InMemorySessionRepository) GetByUserID(userID int64) (*Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessions[userID], nil
}

// Delete removes a session
func (r *InMemorySessionRepository) Delete(userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, userID)
	return nil
}

// DeleteWithoutDevice removes any sessions whose DeviceID is empty.
// Returns the number of rows removed.
func (r *InMemorySessionRepository) DeleteWithoutDevice() (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for userID, session := range r.sessions {
		if session != nil && session.DeviceID == "" {
			delete(r.sessions, userID)
			n++
		}
	}
	return n, nil
}

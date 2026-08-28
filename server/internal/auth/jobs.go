package auth

import (
	"context"
	"log"
	"time"
)

// ResetCodeCleanupJob periodically removes expired reset codes
type ResetCodeCleanupJob struct {
	service *Service
}

// NewResetCodeCleanupJob creates a new cleanup job for expired reset codes
func NewResetCodeCleanupJob(service *Service) *ResetCodeCleanupJob {
	return &ResetCodeCleanupJob{
		service: service,
	}
}

// Name returns the job name
func (j *ResetCodeCleanupJob) Name() string {
	return "ResetCodeCleanup"
}

// Run executes the cleanup
func (j *ResetCodeCleanupJob) Run(ctx context.Context) error {
	return j.service.CleanupExpiredResetCodes()
}

// Interval returns how often to run (every 1 hour)
func (j *ResetCodeCleanupJob) Interval() time.Duration {
	return 1 * time.Hour
}

// SessionCleanupJob removes session rows whose window has run out. Expiry is
// enforced per request either way, so this is housekeeping: without it,
// abandoned sessions accumulate forever.
type SessionCleanupJob struct {
	service *Service
}

// NewSessionCleanupJob creates a new cleanup job for expired sessions.
func NewSessionCleanupJob(service *Service) *SessionCleanupJob {
	return &SessionCleanupJob{service: service}
}

// Name returns the job name
func (j *SessionCleanupJob) Name() string {
	return "SessionCleanup"
}

// Run executes the cleanup
func (j *SessionCleanupJob) Run(ctx context.Context) error {
	removed, err := j.service.CleanupExpiredSessions()
	if err != nil {
		return err
	}
	if removed > 0 {
		log.Printf("[Jobs] SessionCleanup removed %d expired sessions", removed)
	}
	return nil
}

// Interval returns how often to run (every 1 hour)
func (j *SessionCleanupJob) Interval() time.Duration {
	return 1 * time.Hour
}

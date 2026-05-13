package auth

import (
	"context"
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

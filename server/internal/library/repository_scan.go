package library

import (
	"database/sql"
	"time"
)

// =============================================================================
// Scan queue methods
// =============================================================================

// QueueScan adds a scan job to the queue for a library
func (r *repository) QueueScan(libraryID int64) (*ScanJob, error) {
	//language=SQL
	query := `INSERT INTO scan_queue (library_id, status) VALUES (?, 'pending')`
	result, err := r.db.Exec(query, libraryID)
	if err != nil {
		return nil, err
	}

	jobID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.getScanJobByID(jobID)
}

// GetPendingScan retrieves the oldest pending scan job (FIFO)
func (r *repository) GetPendingScan() (*ScanJob, error) {
	//language=SQL
	query := `
SELECT id, library_id, requested_at, started_at, updated_at, status,
       total_files, processed_files, tracks_added, tracks_updated, errors, current_file
FROM scan_queue
WHERE status = 'pending'
ORDER BY requested_at ASC
LIMIT 1`

	job := &ScanJob{}
	var startedAt sql.NullTime
	err := r.db.QueryRow(query).Scan(
		&job.ID, &job.LibraryID, &job.RequestedAt, &startedAt, &job.UpdatedAt, &job.Status,
		&job.TotalFiles, &job.ProcessedFiles, &job.TracksAdded, &job.TracksUpdated,
		&job.Errors, &job.CurrentFile,
	)
	if err == sql.ErrNoRows {
		return nil, nil // No pending jobs
	}
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	return job, nil
}

// GetScanJobByLibrary retrieves active (pending/running) scan job for a library
func (r *repository) GetScanJobByLibrary(libraryID int64) (*ScanJob, error) {
	//language=SQL
	query := `
SELECT id, library_id, requested_at, started_at, updated_at, status,
       total_files, processed_files, tracks_added, tracks_updated, errors, current_file
FROM scan_queue
WHERE library_id = ? AND status IN ('pending', 'running')
ORDER BY requested_at DESC
LIMIT 1`

	job := &ScanJob{}
	var startedAt sql.NullTime
	err := r.db.QueryRow(query, libraryID).Scan(
		&job.ID, &job.LibraryID, &job.RequestedAt, &startedAt, &job.UpdatedAt, &job.Status,
		&job.TotalFiles, &job.ProcessedFiles, &job.TracksAdded, &job.TracksUpdated,
		&job.Errors, &job.CurrentFile,
	)
	if err == sql.ErrNoRows {
		return nil, nil // No active job for this library
	}
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	return job, nil
}

// UpdateScanJob updates a scan job's progress (also updates heartbeat timestamp)
func (r *repository) UpdateScanJob(job *ScanJob) error {
	//language=SQL
	query := `
UPDATE scan_queue SET
    started_at = ?, status = ?,
    total_files = ?, processed_files = ?,
    tracks_added = ?, tracks_updated = ?,
    errors = ?, current_file = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?`

	_, err := r.db.Exec(query,
		job.StartedAt, job.Status,
		job.TotalFiles, job.ProcessedFiles,
		job.TracksAdded, job.TracksUpdated,
		job.Errors, job.CurrentFile,
		job.ID,
	)
	return err
}

// DeleteScanJob removes a scan job from the queue
func (r *repository) DeleteScanJob(jobID int64) error {
	//language=SQL
	query := `DELETE FROM scan_queue WHERE id = ?`
	_, err := r.db.Exec(query, jobID)
	return err
}

// getScanJobByID is a helper to retrieve a scan job by ID
func (r *repository) getScanJobByID(id int64) (*ScanJob, error) {
	//language=SQL
	query := `
SELECT id, library_id, requested_at, started_at, updated_at, status,
       total_files, processed_files, tracks_added, tracks_updated, errors, current_file
FROM scan_queue
WHERE id = ?`

	job := &ScanJob{}
	var startedAt sql.NullTime
	err := r.db.QueryRow(query, id).Scan(
		&job.ID, &job.LibraryID, &job.RequestedAt, &startedAt, &job.UpdatedAt, &job.Status,
		&job.TotalFiles, &job.ProcessedFiles, &job.TracksAdded, &job.TracksUpdated,
		&job.Errors, &job.CurrentFile,
	)
	if err == sql.ErrNoRows {
		return nil, ErrScanJobNotFound
	}
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	return job, nil
}

// ResetOrphanedJobs resets "running" jobs that haven't been updated within the timeout
// (orphaned due to crash/restart). Returns number of jobs reset.
func (r *repository) ResetOrphanedJobs(timeout time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-timeout)
	//language=SQL
	query := `
UPDATE scan_queue
SET status = 'pending', started_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE status = 'running' AND updated_at < ?`

	result, err := r.db.Exec(query, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ResetAllRunningJobs resets every row with status='running' to 'pending',
// regardless of heartbeat freshness. This is the boot-time reset: a fresh
// worker process implies no scan is actually running, so all 'running' rows
// are by definition stale (the previous worker is dead).
func (r *repository) ResetAllRunningJobs() (int64, error) {
	//language=SQL
	query := `
UPDATE scan_queue
SET status = 'pending', started_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE status = 'running'`

	result, err := r.db.Exec(query)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

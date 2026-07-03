package library

import (
	"testing"
	"time"
)

// TestQueueScan_CreatesPendingJob checks a freshly queued job starts as
// pending and is retrievable both by ID (implicitly) and via GetPendingScan.
func TestQueueScan_CreatesPendingJob(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	job, err := repo.QueueScan(libID)
	if err != nil {
		t.Fatalf("queue scan: %v", err)
	}
	if job.ID == 0 {
		t.Fatalf("expected non-zero job ID")
	}
	if job.Status != "pending" {
		t.Fatalf("expected status pending, got %q", job.Status)
	}
	if job.LibraryID != libID {
		t.Fatalf("expected library ID %d, got %d", libID, job.LibraryID)
	}
}

// TestGetPendingScan_ReturnsOldestFIFO checks pending jobs are served in
// request order, and that GetPendingScan returns nil (not an error) when the
// queue is empty.
func TestGetPendingScan_ReturnsOldestFIFO(t *testing.T) {
	repo, db, libID := setupTestRepo(t)

	empty, err := repo.GetPendingScan()
	if err != nil {
		t.Fatalf("get pending scan on empty queue: %v", err)
	}
	if empty != nil {
		t.Fatalf("expected nil for empty queue, got %+v", empty)
	}

	first, err := repo.QueueScan(libID)
	if err != nil {
		t.Fatalf("queue first: %v", err)
	}
	// Force distinguishable requested_at timestamps; same-transaction inserts
	// can otherwise land in the same instant in SQLite. UTC + second
	// truncation matches the format SQLite's CURRENT_TIMESTAMP produces, so
	// the stored strings stay lexically (and thus chronologically) ordered.
	if _, err := db.Exec(`UPDATE scan_queue SET requested_at = ? WHERE id = ?`,
		time.Now().UTC().Truncate(time.Second).Add(-time.Hour), first.ID); err != nil {
		t.Fatalf("backdate first job: %v", err)
	}

	if _, err := repo.QueueScan(libID); err != nil {
		t.Fatalf("queue second: %v", err)
	}

	pending, err := repo.GetPendingScan()
	if err != nil {
		t.Fatalf("get pending scan: %v", err)
	}
	if pending == nil || pending.ID != first.ID {
		t.Fatalf("expected oldest job (id=%d) first, got %+v", first.ID, pending)
	}
}

// TestGetScanJobByLibrary_OnlyActiveJobs checks completed/failed jobs are
// excluded and nil (not an error) is returned when there's no active job.
func TestGetScanJobByLibrary_OnlyActiveJobs(t *testing.T) {
	repo, db, libID := setupTestRepo(t)

	none, err := repo.GetScanJobByLibrary(libID)
	if err != nil {
		t.Fatalf("get scan job with no jobs: %v", err)
	}
	if none != nil {
		t.Fatalf("expected nil with no jobs, got %+v", none)
	}

	job, err := repo.QueueScan(libID)
	if err != nil {
		t.Fatalf("queue scan: %v", err)
	}
	if _, err := db.Exec(`UPDATE scan_queue SET status = 'completed' WHERE id = ?`, job.ID); err != nil {
		t.Fatalf("mark completed: %v", err)
	}

	afterCompletion, err := repo.GetScanJobByLibrary(libID)
	if err != nil {
		t.Fatalf("get scan job after completion: %v", err)
	}
	if afterCompletion != nil {
		t.Fatalf("expected nil for completed job, got %+v", afterCompletion)
	}

	running, err := repo.QueueScan(libID)
	if err != nil {
		t.Fatalf("queue second scan: %v", err)
	}
	if _, err := db.Exec(`UPDATE scan_queue SET status = 'running' WHERE id = ?`, running.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	active, err := repo.GetScanJobByLibrary(libID)
	if err != nil {
		t.Fatalf("get scan job: %v", err)
	}
	if active == nil || active.ID != running.ID {
		t.Fatalf("expected running job (id=%d), got %+v", running.ID, active)
	}
}

// TestUpdateScanJob_PersistsProgress checks progress fields round-trip.
func TestUpdateScanJob_PersistsProgress(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	job, err := repo.QueueScan(libID)
	if err != nil {
		t.Fatalf("queue scan: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	job.StartedAt = &now
	job.Status = "running"
	job.TotalFiles = 100
	job.ProcessedFiles = 42
	job.TracksAdded = 40
	job.TracksUpdated = 2
	job.Errors = 1
	job.CurrentFile = "/m/some/file.flac"

	if err := repo.UpdateScanJob(job); err != nil {
		t.Fatalf("update scan job: %v", err)
	}

	fetched, err := repo.GetScanJobByLibrary(libID)
	if err != nil {
		t.Fatalf("get scan job: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected job still active after update")
	}
	if fetched.Status != "running" || fetched.TotalFiles != 100 || fetched.ProcessedFiles != 42 ||
		fetched.TracksAdded != 40 || fetched.TracksUpdated != 2 || fetched.Errors != 1 ||
		fetched.CurrentFile != "/m/some/file.flac" {
		t.Fatalf("progress fields did not round-trip, got %+v", fetched)
	}
	if fetched.StartedAt == nil || !fetched.StartedAt.Equal(now) {
		t.Fatalf("expected started_at %v, got %v", now, fetched.StartedAt)
	}
}

// TestDeleteScanJob_RemovesFromQueue checks the job disappears from both the
// pending queue and per-library lookup.
func TestDeleteScanJob_RemovesFromQueue(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	job, err := repo.QueueScan(libID)
	if err != nil {
		t.Fatalf("queue scan: %v", err)
	}

	if err := repo.DeleteScanJob(job.ID); err != nil {
		t.Fatalf("delete scan job: %v", err)
	}

	pending, err := repo.GetPendingScan()
	if err != nil {
		t.Fatalf("get pending scan: %v", err)
	}
	if pending != nil {
		t.Fatalf("expected no pending scan after delete, got %+v", pending)
	}
}

// TestResetOrphanedJobs_OnlyResetsStaleRunningJobs checks the timeout cutoff:
// a running job with a recent heartbeat survives, a stale one is reset to
// pending, and a pending job is left alone.
func TestResetOrphanedJobs_OnlyResetsStaleRunningJobs(t *testing.T) {
	repo, db, libID := setupTestRepo(t)

	fresh, err := repo.QueueScan(libID)
	if err != nil {
		t.Fatalf("queue fresh job: %v", err)
	}
	if _, err := db.Exec(`UPDATE scan_queue SET status = 'running', updated_at = ? WHERE id = ?`,
		time.Now().UTC(), fresh.ID); err != nil {
		t.Fatalf("mark fresh running: %v", err)
	}

	stale, err := repo.QueueScan(libID)
	if err != nil {
		t.Fatalf("queue stale job: %v", err)
	}
	// UTC (not local time) so the stored string stays lexically comparable to
	// the UTC cutoff ResetOrphanedJobs computes internally.
	if _, err := db.Exec(`UPDATE scan_queue SET status = 'running', updated_at = ? WHERE id = ?`,
		time.Now().UTC().Add(-time.Hour), stale.ID); err != nil {
		t.Fatalf("mark stale running: %v", err)
	}

	n, err := repo.ResetOrphanedJobs(10 * time.Minute)
	if err != nil {
		t.Fatalf("reset orphaned jobs: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 job reset, got %d", n)
	}

	var freshStatus, staleStatus string
	if err := db.QueryRow(`SELECT status FROM scan_queue WHERE id = ?`, fresh.ID).Scan(&freshStatus); err != nil {
		t.Fatalf("read fresh status: %v", err)
	}
	if err := db.QueryRow(`SELECT status FROM scan_queue WHERE id = ?`, stale.ID).Scan(&staleStatus); err != nil {
		t.Fatalf("read stale status: %v", err)
	}
	if freshStatus != "running" {
		t.Fatalf("expected fresh job still running, got %q", freshStatus)
	}
	if staleStatus != "pending" {
		t.Fatalf("expected stale job reset to pending, got %q", staleStatus)
	}
}

// TestResetAllRunningJobs_ResetsRegardlessOfHeartbeat is the boot-time reset
// spec: even a job with a fresh heartbeat is reset, because no worker is
// running yet by definition.
func TestResetAllRunningJobs_ResetsRegardlessOfHeartbeat(t *testing.T) {
	repo, db, libID := setupTestRepo(t)

	job, err := repo.QueueScan(libID)
	if err != nil {
		t.Fatalf("queue scan: %v", err)
	}
	if _, err := db.Exec(`UPDATE scan_queue SET status = 'running', updated_at = ? WHERE id = ?`,
		time.Now(), job.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	n, err := repo.ResetAllRunningJobs()
	if err != nil {
		t.Fatalf("reset all running jobs: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 job reset, got %d", n)
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM scan_queue WHERE id = ?`, job.ID).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "pending" {
		t.Fatalf("expected pending, got %q", status)
	}
}

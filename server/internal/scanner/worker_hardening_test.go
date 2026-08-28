package scanner

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"audiod/internal/library"
)

// failingUpdateRepo makes UpdateScanJob fail while delegating everything else
// to a real repository.
type failingUpdateRepo struct {
	library.Repository
	calls atomic.Int64
}

func (r *failingUpdateRepo) UpdateScanJob(job *library.ScanJob) error {
	r.calls.Add(1)
	return errors.New("simulated write failure")
}

// TestProcessPendingScans_DoesNotSpinWhenJobUpdateFails is the regression guard
// for a hot loop that could take the whole server down.
//
// processJob returned early on an UpdateScanJob error without changing the
// job's status or deleting it, so processPendingScans re-fetched the same
// still-pending row and retried forever with no backoff and no stop check.
// Because the SQLite pool is pinned to one connection, that spin monopolised
// the server's only connection and starved every HTTP request behind it.
func TestProcessPendingScans_DoesNotSpinWhenJobUpdateFails(t *testing.T) {
	// Given a queued scan whose first status write will fail
	w, repo, _, libID := setupTestWorker(t)
	if _, err := repo.QueueScan(libID); err != nil {
		t.Fatalf("QueueScan() failed: %v", err)
	}
	failing := &failingUpdateRepo{Repository: repo}
	w.repo = failing

	// When the worker drains the queue
	done := make(chan struct{})
	go func() {
		w.processPendingScans()
		close(done)
	}()

	// Then it gives up rather than looping on the same row
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("processPendingScans() still running after 2s; UpdateScanJob called %d times",
			failing.calls.Load())
	}

	if calls := failing.calls.Load(); calls > 5 {
		t.Errorf("UpdateScanJob called %d times, want a small bounded number", calls)
	}
}

// TestResetAllRunningJobs_ZeroesProgressCounters covers resumed-scan progress.
//
// The reset queries restored status/started_at but left processed_files,
// tracks_added, tracks_updated and errors at their crash-time values, while
// processJob recomputes total_files from scratch. A scan that died at 4321 of
// 5000 files resumed counting from 4321 up to 9321 — a progress bar reading
// 186%, with error and track counts double-reported.
func TestResetAllRunningJobs_ZeroesProgressCounters(t *testing.T) {
	// Given a running job that recorded partial progress before dying
	_, repo, db, libID := setupTestWorker(t)
	_, err := db.Exec(
		`INSERT INTO scan_queue
		   (library_id, status, total_files, processed_files, tracks_added, tracks_updated, errors, started_at, updated_at)
		 VALUES (?, 'running', 5000, 4321, 400, 20, 7, ?, ?)`,
		libID, time.Now().UTC(), time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("insert running job: %v", err)
	}

	// When the worker resets it at boot
	if _, err := repo.ResetAllRunningJobs(); err != nil {
		t.Fatalf("ResetAllRunningJobs() failed: %v", err)
	}

	// Then the per-run counters start clean
	job, err := repo.GetScanJobByLibrary(libID)
	if err != nil {
		t.Fatalf("GetScanJobByLibrary() failed: %v", err)
	}
	if job == nil {
		t.Fatal("expected the reset job to still exist")
	}
	if job.Status != "pending" {
		t.Errorf("status = %q, want \"pending\"", job.Status)
	}
	for _, c := range []struct {
		name string
		got  int
	}{
		{"processed_files", job.ProcessedFiles},
		{"tracks_added", job.TracksAdded},
		{"tracks_updated", job.TracksUpdated},
		{"errors", job.Errors},
	} {
		if c.got != 0 {
			t.Errorf("%s = %d after reset, want 0", c.name, c.got)
		}
	}
}

// TestProcessAudioFile_SurvivesMetadataPanic covers the crash-loop.
//
// Nothing in the server recovers, and metadata extraction parses arbitrary
// ID3/FLAC/MP4 headers off disk — a known source of slice-bounds panics on
// malformed input. One truncated file killed the whole process, which then
// restarted, reached the same file, and crash-looped.
func TestProcessAudioFile_SurvivesMetadataPanic(t *testing.T) {
	// Given an extractor that panics the way a corrupt header would
	w, _, _, libID := setupTestWorker(t)
	w.extractMetadata = func(string) (*library.AudioMetadata, error) {
		panic("simulated slice bounds out of range")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.flac")
	if err := os.WriteFile(path, []byte("not really a flac"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}

	job := &library.ScanJob{LibraryID: libID}

	// When the scan reaches that file, it must not take the process down
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic escaped processAudioFile: %v", r)
		}
	}()
	w.processAudioFile(libID, path, info, job)

	// Then the file is counted as an error and the scan moves on
	if job.Errors != 1 {
		t.Errorf("job.Errors = %d, want 1", job.Errors)
	}
	if job.ProcessedFiles != 1 {
		t.Errorf("job.ProcessedFiles = %d, want 1", job.ProcessedFiles)
	}
}

// TestProcessPendingScans_StopsWhenWorkerStops covers shutdown.
//
// processPendingScans drained the entire pending queue with no stop check, and
// wg.Add lived inside processJob so the counter hit zero between jobs. Stop()
// could therefore observe zero and return while the next job was still
// starting, letting main reach db.Close() mid-scan.
func TestProcessPendingScans_StopsWhenWorkerStops(t *testing.T) {
	// Given a worker that has already been told to stop
	w, repo, _, libID := setupTestWorker(t)
	if _, err := repo.QueueScan(libID); err != nil {
		t.Fatalf("QueueScan() failed: %v", err)
	}
	close(w.stop)

	// When it drains the queue
	w.processPendingScans()

	// Then the pending job is left for the next process rather than started
	job, err := repo.GetScanJobByLibrary(libID)
	if err != nil {
		t.Fatalf("GetScanJobByLibrary() failed: %v", err)
	}
	if job == nil {
		t.Fatal("expected the queued job to still exist")
	}
	if job.Status != "pending" {
		t.Errorf("status = %q, want \"pending\" (job should not have been started)", job.Status)
	}
}

// recordingDeleteRepo captures DeleteTracksByPaths calls.
type recordingDeleteRepo struct {
	library.Repository
	deleted [][]string
}

func (r *recordingDeleteRepo) DeleteTracksByPaths(libraryID int64, paths []string) (int64, error) {
	r.deleted = append(r.deleted, paths)
	return int64(len(paths)), nil
}

// TestRemoveVanishedTracks covers pruning tracks whose files are gone.
//
// Scanning was add/update-only, so an album deleted from disk stayed in the
// database and the FTS index forever — visible in the grid, and a 500 on play.
// The guard matters just as much: an unreadable path (unmounted share, changed
// permissions) is indistinguishable from "everything was deleted", and acting
// on that is unrecoverable.
func TestRemoveVanishedTracks(t *testing.T) {
	existing := map[string]time.Time{
		"/music/gone.flac": time.Now(),
		"/music/kept.flac": time.Now(),
	}
	seen := map[string]bool{"/music/kept.flac": true}

	t.Run("removes tracks the walk did not find", func(t *testing.T) {
		w, repo, _, libID := setupTestWorker(t)
		rec := &recordingDeleteRepo{Repository: repo}
		w.repo = rec

		w.removeVanishedTracks(libID, existing, seen, false)

		if len(rec.deleted) != 1 {
			t.Fatalf("DeleteTracksByPaths called %d times, want 1", len(rec.deleted))
		}
		if got := rec.deleted[0]; len(got) != 1 || got[0] != "/music/gone.flac" {
			t.Errorf("deleted %v, want [/music/gone.flac]", got)
		}
	})

	t.Run("deletes nothing when the walk could not read every path", func(t *testing.T) {
		w, repo, _, libID := setupTestWorker(t)
		rec := &recordingDeleteRepo{Repository: repo}
		w.repo = rec

		w.removeVanishedTracks(libID, existing, seen, true)

		if len(rec.deleted) != 0 {
			t.Errorf("DeleteTracksByPaths called %d times after a failed walk, want 0", len(rec.deleted))
		}
	})
}

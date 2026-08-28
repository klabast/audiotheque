package scanner

import (
	"io/fs"
	"log"
	"path/filepath"
	"sync"
	"time"

	"audiod/internal/config"
	"audiod/internal/library"
	"audiod/internal/websocket"
)

const (
	// OrphanTimeout is how long a job can go without updates before being considered orphaned
	OrphanTimeout = 2 * time.Minute
	// MaintenanceInterval is how often the worker sweeps for in-process orphans
	// (running jobs whose heartbeat has gone stale while the worker is still up).
	MaintenanceInterval = 30 * time.Second
	// ShutdownTimeout is how long to wait for current job to finish during shutdown
	ShutdownTimeout = 30 * time.Second
	// ProgressThrottle bounds how often a 'scan-progress' event is emitted.
	ProgressThrottle = 500 * time.Millisecond
	// LibraryUpdatedThrottle bounds how often a per-library 'library-updated'
	// event is emitted during a busy scan. The UI debounces its own refetch on
	// top of this, but throttling at the source keeps the WS quiet.
	LibraryUpdatedThrottle = 2 * time.Second
)

// Broadcaster is the subset of *websocket.Hub the scanner actually needs.
// Defined as an interface so tests can substitute a recording mock.
type Broadcaster interface {
	Broadcast(msg websocket.Message) error
}

// Worker is a background worker that processes scan jobs from the queue
type Worker struct {
	broadcaster         Broadcaster
	repo                library.Repository
	dataDir             string
	interval            time.Duration
	maintenanceInterval time.Duration
	stop                chan struct{}
	wg                  sync.WaitGroup // tracks running job for graceful shutdown

	// extractMetadata is library.ExtractMetadata in production. It is a field so
	// tests can drive the panic path, which real corrupt files reach but a
	// fixture cannot reliably reproduce.
	extractMetadata func(path string) (*library.AudioMetadata, error)

	// Progress throttling (per-library)
	lastBroadcast map[int64]time.Time

	// library-updated throttling (per-library)
	libraryUpdatedThrottle      time.Duration
	lastLibraryUpdatedBroadcast map[int64]time.Time
}

// NewWorker creates a new scanner worker.
// Pass *websocket.Hub for production; any Broadcaster works (tests substitute a mock).
func NewWorker(repo library.Repository, broadcaster Broadcaster) *Worker {
	return &Worker{
		broadcaster:                 broadcaster,
		repo:                        repo,
		extractMetadata:             library.ExtractMetadata,
		dataDir:                     config.GetDataDir(),
		interval:                    2 * time.Second,
		maintenanceInterval:         MaintenanceInterval,
		stop:                        make(chan struct{}),
		lastBroadcast:               make(map[int64]time.Time),
		libraryUpdatedThrottle:      LibraryUpdatedThrottle,
		lastLibraryUpdatedBroadcast: make(map[int64]time.Time),
	}
}

// Start begins the background worker loop
func (w *Worker) Start() {
	// On boot, every 'running' row is stale by definition — the previous worker
	// process is gone. Reset them unconditionally so a fresh scan can be queued
	// after a container restart. (The timeout-based ResetOrphanedJobs is run
	// periodically in the loop to catch in-process orphans, not boot-time ones.)
	w.resetAllRunningJobs()

	go w.run()
	log.Println("Scanner worker started")
}

// Stop gracefully stops the worker, waiting for current job to finish
func (w *Worker) Stop() {
	close(w.stop)

	// Wait for current job to finish (with timeout)
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Scanner worker: current job finished")
	case <-time.After(ShutdownTimeout):
		log.Println("Scanner worker: timeout waiting for job to finish")
	}
	log.Println("Scanner worker stopped")
}

// resetAllRunningJobs unconditionally clears every 'running' row to 'pending'.
// Called once at worker boot.
func (w *Worker) resetAllRunningJobs() {
	count, err := w.repo.ResetAllRunningJobs()
	if err != nil {
		log.Printf("Error resetting running jobs at boot: %v", err)
		return
	}
	if count > 0 {
		log.Printf("Reset %d stale running scan job(s) at boot", count)
	}
}

// resetOrphanedJobs resets any "running" jobs that weren't updated within
// OrphanTimeout. Called periodically by run() to handle in-process orphans
// (stuck goroutines whose heartbeat has stopped while the worker is still up).
func (w *Worker) resetOrphanedJobs() {
	count, err := w.repo.ResetOrphanedJobs(OrphanTimeout)
	if err != nil {
		log.Printf("Error resetting orphaned jobs: %v", err)
		return
	}
	if count > 0 {
		log.Printf("Reset %d orphaned scan job(s)", count)
	}
}

// run is the main worker loop
func (w *Worker) run() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	maintTicker := time.NewTicker(w.maintenanceInterval)
	defer maintTicker.Stop()

	// Process any pending jobs immediately on startup
	w.processPendingScans()

	for {
		select {
		case <-ticker.C:
			w.processPendingScans()
		case <-maintTicker.C:
			w.resetOrphanedJobs()
		case <-w.stop:
			return
		}
	}
}

// processPendingScans picks up and processes pending scan jobs
func (w *Worker) processPendingScans() {
	for {
		// Shutdown was requested: leave anything still queued for the next
		// process rather than starting work main is about to close the DB under.
		select {
		case <-w.stop:
			return
		default:
		}

		// Get oldest pending job
		job, err := w.repo.GetPendingScan()
		if err != nil {
			log.Printf("Error getting pending scan: %v", err)
			return
		}
		if job == nil {
			return // No pending jobs
		}

		// A job that could not be claimed stays pending, so continuing the loop
		// would re-fetch the same row forever. Back off to the next tick instead.
		if !w.processJob(job) {
			return
		}
	}
}

// processJob processes a single scan job, reporting whether the queue can be
// drained further.
func (w *Worker) processJob(job *library.ScanJob) bool {
	w.wg.Add(1)
	defer w.wg.Done()

	log.Printf("Starting scan for library %d (job %d)", job.LibraryID, job.ID)

	// Mark job as running. Counters are per-run: a job resumed after a crash
	// carries the previous run's values, and total_files is recomputed below.
	now := time.Now().UTC()
	job.StartedAt = &now
	job.Status = "running"
	job.ProcessedFiles = 0
	job.TracksAdded = 0
	job.TracksUpdated = 0
	job.Errors = 0
	if err := w.repo.UpdateScanJob(job); err != nil {
		log.Printf("Error updating job status for job %d: %v", job.ID, err)
		return false
	}

	// Get library details
	lib, err := w.repo.GetLibraryByID(job.LibraryID)
	if err != nil {
		log.Printf("Error getting library %d: %v", job.LibraryID, err)
		if err := w.repo.DeleteScanJob(job.ID); err != nil {
			log.Printf("Error deleting job %d for missing library: %v", job.ID, err)
			return false
		}
		return true
	}

	// Phase 1: Count files
	job.TotalFiles = w.countAudioFiles(lib.Paths)
	w.updateAndBroadcast(job)

	// Phase 2: Get existing tracks for incremental scanning
	existingTracks, err := w.repo.GetTrackPathsForLibrary(lib.ID)
	if err != nil {
		log.Printf("Error getting existing tracks: %v", err)
		existingTracks = make(map[string]time.Time)
	}

	// Phase 3: Walk and process files
	seen := make(map[string]bool, len(existingTracks))
	walkFailed := false
	for _, basePath := range lib.Paths {
		err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("Error accessing %s: %v", path, err)
				job.Errors++
				walkFailed = true
				return nil // Continue walking
			}

			if d.IsDir() {
				return nil
			}

			if !library.IsSupportedAudioFile(path) {
				return nil
			}

			// Check if file changed (incremental scan)
			info, err := d.Info()
			if err != nil {
				job.Errors++
				walkFailed = true
				return nil
			}

			seen[path] = true

			if modTime, exists := existingTracks[path]; exists {
				if info.ModTime().Equal(modTime) {
					job.ProcessedFiles++
					w.throttledUpdateAndBroadcast(job)
					return nil // Skip unchanged
				}
			}

			// Process the audio file
			w.processAudioFile(lib.ID, path, info, job)

			return nil
		})

		if err != nil {
			log.Printf("Error walking %s: %v", basePath, err)
			walkFailed = true
		}
	}

	w.removeVanishedTracks(lib.ID, existingTracks, seen, walkFailed)

	// Phase 4: Complete - broadcast final progress then delete job
	job.CurrentFile = ""
	w.broadcastProgress(job, "completed")
	// Always emit a final library-updated so the UI catches anything the
	// throttled per-track emissions skipped.
	w.broadcastLibraryUpdated(job.LibraryID)

	log.Printf("Scan completed for library %d: %d files, %d added, %d updated, %d errors",
		job.LibraryID, job.ProcessedFiles, job.TracksAdded, job.TracksUpdated, job.Errors)

	// Delete the job from the queue. A row left behind still reads as 'running',
	// which makes StartScan return ErrScanAlreadyInProgress for that library
	// until the orphan sweep catches it.
	if err := w.repo.DeleteScanJob(job.ID); err != nil {
		log.Printf("Error deleting completed job %d: %v", job.ID, err)
		return false
	}
	return true
}

// processAudioFile extracts metadata and creates/updates database records.
//
// It recovers from panics: metadata parsing reads attacker- or
// corruption-supplied tag headers, and this runs on the worker goroutine, so an
// unrecovered panic takes the whole server down and then crash-loops on the
// same file after restart.
func (w *Worker) processAudioFile(libraryID int64, path string, info fs.FileInfo, job *library.ScanJob) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic processing %s: %v", path, r)
			job.Errors++
			job.ProcessedFiles++
			w.throttledUpdateAndBroadcast(job)
		}
	}()

	job.CurrentFile = path

	// Extract metadata
	meta, err := w.extractMetadata(path)
	if err != nil {
		log.Printf("Error extracting metadata from %s: %v", path, err)
		job.Errors++
		job.ProcessedFiles++
		w.throttledUpdateAndBroadcast(job)
		return
	}

	// Use filename as title fallback
	if meta.Title == "" {
		meta.Title = filepath.Base(path)
	}

	// Determine album artist (prefer AlbumArtist tag, fall back to Artist)
	albumArtistName := meta.AlbumArtist
	if albumArtistName == "" {
		albumArtistName = meta.Artist
	}
	if albumArtistName == "" {
		albumArtistName = "Unknown Artist"
	}

	// Create or get album artist
	var albumArtistID *int64
	albumArtist, err := w.repo.GetOrCreateArtist(libraryID, albumArtistName)
	if err != nil {
		log.Printf("Error creating album artist: %v", err)
		job.Errors++
	} else {
		albumArtistID = &albumArtist.ID
	}

	// Create or get track artist (may differ from album artist for compilations/features)
	var trackArtistID *int64
	trackArtistName := meta.Artist
	if trackArtistName == "" {
		trackArtistName = albumArtistName
	}
	if trackArtistName != albumArtistName {
		// Different track artist - create separate artist entry
		trackArtist, err := w.repo.GetOrCreateArtist(libraryID, trackArtistName)
		if err != nil {
			log.Printf("Error creating track artist: %v", err)
		} else {
			trackArtistID = &trackArtist.ID

			// Update MusicBrainz ID if available
			if meta.MusicBrainzArtistID != "" && trackArtist.MusicBrainzID == "" {
				_ = w.repo.UpdateArtistMusicBrainzID(trackArtist.ID, meta.MusicBrainzArtistID)
			}
		}
	} else {
		// Same artist for track and album
		trackArtistID = albumArtistID
	}

	// Create or get album (using album artist, not track artist)
	var albumID *int64
	albumTitle := meta.Album
	if albumTitle == "" {
		albumTitle = "Unknown Album"
	}
	album, err := w.repo.GetOrCreateAlbum(libraryID, albumArtistID, albumTitle, "", filepath.Dir(path))
	if err != nil {
		log.Printf("Error creating album: %v", err)
		job.Errors++
	} else {
		albumID = &album.ID

		// Extract cover art if album doesn't have one
		if album.CoverArtPath == "" && meta.HasPicture && w.dataDir != "" {
			coverPath, err := library.ExtractCoverArt(meta, album.ID, w.dataDir)
			if err != nil {
				log.Printf("Error extracting cover art: %v", err)
			} else if coverPath != "" {
				_ = w.repo.UpdateAlbumCoverArt(album.ID, coverPath)
			}
		}
	}

	// Create or update track
	track := &library.Track{
		LibraryID:     libraryID,
		AlbumID:       albumID,
		ArtistID:      trackArtistID,
		FilePath:      path,
		FileName:      filepath.Base(path),
		FileSize:      info.Size(),
		FileModified:  info.ModTime(),
		Title:         meta.Title,
		TrackNumber:   meta.TrackNumber,
		DiscNumber:    meta.DiscNumber,
		Duration:      meta.Duration,
		Year:          meta.Year,
		Genre:         meta.Genre,
		MusicBrainzID: meta.MusicBrainzTrackID,
		Codec:         meta.Codec,
		Bitrate:       meta.Bitrate,
		SampleRate:    meta.SampleRate,
		BitDepth:      meta.BitDepth,
		Channels:      meta.Channels,
		IsLossless:    meta.IsLossless,
		IsHiRes:       meta.IsHiRes,
	}

	existing, _ := w.repo.GetTrackByPath(libraryID, path)
	savedTrack, err := w.repo.CreateOrUpdateTrack(track)
	trackChanged := err == nil
	if err != nil {
		log.Printf("Error saving track: %v", err)
		job.Errors++
	} else {
		if existing != nil {
			job.TracksUpdated++
		} else {
			job.TracksAdded++
		}

		// Add track-artist relationship (primary artist)
		if trackArtistID != nil && savedTrack != nil {
			if err := w.repo.AddTrackArtist(savedTrack.ID, *trackArtistID, "primary", 0); err != nil {
				log.Printf("Error adding track-artist relationship: %v", err)
			}
		}
	}

	job.ProcessedFiles++
	w.throttledUpdateAndBroadcast(job)
	if trackChanged {
		// Signal "you should refetch albums for this library" — throttled so
		// big scans don't flood the WS. The UI also debounces on its side.
		w.maybeBroadcastLibraryUpdated(libraryID)
	}
}

// removeVanishedTracks deletes tracks whose files the walk did not visit.
//
// Scanning used to be add/update-only, so deleting an album from disk left its
// tracks in the database and the search index forever: they kept showing in the
// grid and streaming them returned a server error.
//
// It refuses to delete anything if the walk hit an error, because an unreadable
// path — an unmounted NFS share, a permissions change — looks exactly like
// "every file was removed", and that mistake is unrecoverable.
func (w *Worker) removeVanishedTracks(libraryID int64, existing map[string]time.Time, seen map[string]bool, walkFailed bool) {
	if walkFailed {
		log.Printf("Skipping removal of missing tracks for library %d: the scan could not read every path", libraryID)
		return
	}

	var vanished []string
	for path := range existing {
		if !seen[path] {
			vanished = append(vanished, path)
		}
	}
	if len(vanished) == 0 {
		return
	}

	deleted, err := w.repo.DeleteTracksByPaths(libraryID, vanished)
	if err != nil {
		log.Printf("Error removing %d missing track(s) from library %d: %v", len(vanished), libraryID, err)
		return
	}
	log.Printf("Removed %d track(s) no longer on disk from library %d", deleted, libraryID)
}

// countAudioFiles counts total audio files in paths
func (w *Worker) countAudioFiles(paths []string) int {
	count := 0
	for _, basePath := range paths {
		_ = filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if library.IsSupportedAudioFile(path) {
				count++
			}
			return nil
		})
	}
	return count
}

// updateAndBroadcast saves job progress to DB and broadcasts to WebSocket
func (w *Worker) updateAndBroadcast(job *library.ScanJob) {
	if err := w.repo.UpdateScanJob(job); err != nil {
		log.Printf("Error updating job progress: %v", err)
	}
	w.broadcastProgress(job, job.Status)
}

// throttledUpdateAndBroadcast rate-limits progress updates.
//
// The every-10-files arm used to fire unconditionally, so a fast local-disk
// scan emitted thousands of messages per second and overran the per-client
// send buffers, which the hub treats as a dead client. Time is the bound; the
// file count only decides when it is worth checking the clock.
func (w *Worker) throttledUpdateAndBroadcast(job *library.ScanJob) {
	now := time.Now()
	if now.Sub(w.lastBroadcast[job.LibraryID]) < ProgressThrottle {
		return
	}
	w.updateAndBroadcast(job)
	w.lastBroadcast[job.LibraryID] = now
}

// broadcastProgress sends scan progress to WebSocket clients
func (w *Worker) broadcastProgress(job *library.ScanJob, status string) {
	if w.broadcaster == nil {
		return
	}

	var startedAt time.Time
	if job.StartedAt != nil {
		startedAt = *job.StartedAt
	}

	progress := &library.ScanProgress{
		LibraryID:      job.LibraryID,
		Status:         status,
		TotalFiles:     job.TotalFiles,
		ProcessedFiles: job.ProcessedFiles,
		TracksAdded:    job.TracksAdded,
		TracksUpdated:  job.TracksUpdated,
		Errors:         job.Errors,
		CurrentFile:    job.CurrentFile,
		StartedAt:      startedAt,
	}

	msg := websocket.Message{
		Type: "scan-progress",
		Data: progress,
	}

	if err := w.broadcaster.Broadcast(msg); err != nil {
		log.Printf("Failed to broadcast scan progress: %v", err)
	}
}

// broadcastLibraryUpdated emits a 'library-updated' event for the given
// library. The UI uses it as a "you should refetch albums" signal and
// debounces its own refresh.
func (w *Worker) broadcastLibraryUpdated(libraryID int64) {
	if w.broadcaster == nil {
		return
	}
	msg := websocket.Message{
		Type: "library-updated",
		Data: map[string]int64{"libraryId": libraryID},
	}
	if err := w.broadcaster.Broadcast(msg); err != nil {
		log.Printf("Failed to broadcast library-updated for library %d: %v", libraryID, err)
	}
}

// maybeBroadcastLibraryUpdated emits at most one 'library-updated' event per
// library per libraryUpdatedThrottle window. Used inside the per-track loop
// so a 10k-track scan doesn't fan out 10k WS messages.
func (w *Worker) maybeBroadcastLibraryUpdated(libraryID int64) {
	now := time.Now()
	last := w.lastLibraryUpdatedBroadcast[libraryID]
	if now.Sub(last) < w.libraryUpdatedThrottle {
		return
	}
	w.broadcastLibraryUpdated(libraryID)
	w.lastLibraryUpdatedBroadcast[libraryID] = now
}

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
		// Get oldest pending job
		job, err := w.repo.GetPendingScan()
		if err != nil {
			log.Printf("Error getting pending scan: %v", err)
			return
		}
		if job == nil {
			return // No pending jobs
		}

		// Process the job
		w.processJob(job)
	}
}

// processJob processes a single scan job
func (w *Worker) processJob(job *library.ScanJob) {
	w.wg.Add(1)
	defer w.wg.Done()

	log.Printf("Starting scan for library %d (job %d)", job.LibraryID, job.ID)

	// Mark job as running
	now := time.Now()
	job.StartedAt = &now
	job.Status = "running"
	if err := w.repo.UpdateScanJob(job); err != nil {
		log.Printf("Error updating job status: %v", err)
		return
	}

	// Get library details
	lib, err := w.repo.GetLibraryByID(job.LibraryID)
	if err != nil {
		log.Printf("Error getting library %d: %v", job.LibraryID, err)
		w.repo.DeleteScanJob(job.ID)
		return
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
	for _, basePath := range lib.Paths {
		err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("Error accessing %s: %v", path, err)
				job.Errors++
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
				return nil
			}

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
		}
	}

	// Phase 4: Complete - broadcast final progress then delete job
	job.CurrentFile = ""
	w.broadcastProgress(job, "completed")
	// Always emit a final library-updated so the UI catches anything the
	// throttled per-track emissions skipped.
	w.broadcastLibraryUpdated(job.LibraryID)

	log.Printf("Scan completed for library %d: %d files, %d added, %d updated, %d errors",
		job.LibraryID, job.ProcessedFiles, job.TracksAdded, job.TracksUpdated, job.Errors)

	// Delete the job from the queue
	if err := w.repo.DeleteScanJob(job.ID); err != nil {
		log.Printf("Error deleting completed job: %v", err)
	}
}

// processAudioFile extracts metadata and creates/updates database records
func (w *Worker) processAudioFile(libraryID int64, path string, info fs.FileInfo, job *library.ScanJob) {
	job.CurrentFile = path

	// Extract metadata
	meta, err := library.ExtractMetadata(path)
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

// throttledUpdateAndBroadcast limits updates to every 500ms or every 10 files
func (w *Worker) throttledUpdateAndBroadcast(job *library.ScanJob) {
	now := time.Now()
	lastBroadcast := w.lastBroadcast[job.LibraryID]

	if job.ProcessedFiles%10 == 0 || now.Sub(lastBroadcast) >= 500*time.Millisecond {
		w.updateAndBroadcast(job)
		w.lastBroadcast[job.LibraryID] = now
	}
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

package library

import (
	"io/fs"
	"log"
	"path/filepath"
	"sync"
	"time"

	"audiod/internal/config"
	"audiod/internal/websocket"
)

// Scanner handles library scanning with progress tracking
type Scanner struct {
	repo    Repository
	hub     *websocket.Hub
	dataDir string // For storing cover art

	// In-memory state
	activeScans  sync.Map // map[libraryID]bool
	scanProgress sync.Map // map[libraryID]*ScanProgress

	// Progress throttling
	lastBroadcast time.Time
}

// NewScanner creates a new scanner instance
func NewScanner(repo Repository, hub *websocket.Hub) *Scanner {
	return &Scanner{
		repo:    repo,
		hub:     hub,
		dataDir: config.GetDataDir(),
	}
}

// StartScan initiates a library scan
// Returns ErrScanAlreadyInProgress if a scan is already running for this library
func (s *Scanner) StartScan(libraryID int64) error {
	// Check if already scanning (atomic operation)
	if _, exists := s.activeScans.LoadOrStore(libraryID, true); exists {
		return ErrScanAlreadyInProgress
	}

	// Verify library exists
	library, err := s.repo.GetLibraryByID(libraryID)
	if err != nil {
		s.activeScans.Delete(libraryID)
		return err
	}

	// Initialize progress
	progress := &ScanProgress{
		LibraryID: libraryID,
		Status:    "running",
		StartedAt: time.Now(),
	}
	s.scanProgress.Store(libraryID, progress)

	// Start scan in background goroutine
	go s.scan(library)

	return nil
}

// GetProgress returns current scan progress, or ErrNoScanInProgress if not scanning
func (s *Scanner) GetProgress(libraryID int64) (*ScanProgress, error) {
	if progress, ok := s.scanProgress.Load(libraryID); ok {
		return progress.(*ScanProgress), nil
	}
	return nil, ErrNoScanInProgress
}

// scan performs the actual scanning (runs in goroutine)
func (s *Scanner) scan(library *Library) {
	defer s.activeScans.Delete(library.ID)

	progress, _ := s.scanProgress.Load(library.ID)
	p := progress.(*ScanProgress)

	// Phase 1: Count files for progress bar
	p.Status = "counting"
	s.broadcastProgress(p)
	p.TotalFiles = s.countAudioFiles(library.Paths)

	// Phase 2: Get existing tracks for incremental scanning
	existingTracks, err := s.repo.GetTrackPathsForLibrary(library.ID)
	if err != nil {
		log.Printf("Error getting existing tracks: %v", err)
		existingTracks = make(map[string]time.Time)
	}

	// Phase 3: Walk and process
	p.Status = "running"
	s.broadcastProgress(p)

	for _, basePath := range library.Paths {
		err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("Error accessing %s: %v", path, err)
				p.Errors++
				return nil // Continue walking
			}

			if d.IsDir() {
				return nil
			}

			if !IsSupportedAudioFile(path) {
				return nil
			}

			// Check if file changed (incremental scan)
			info, err := d.Info()
			if err != nil {
				p.Errors++
				return nil
			}

			if modTime, exists := existingTracks[path]; exists {
				if info.ModTime().Equal(modTime) {
					p.ProcessedFiles++
					s.throttledBroadcast(p)
					return nil // Skip unchanged
				}
			}

			// Process the audio file
			s.processAudioFile(library.ID, path, info, p)

			return nil
		})

		if err != nil {
			log.Printf("Error walking %s: %v", basePath, err)
		}
	}

	// Phase 4: Complete
	p.Status = "completed"
	p.CurrentFile = ""
	s.broadcastProgress(p)

	// Keep progress around briefly so clients can see completion
	time.Sleep(3 * time.Second)
	s.scanProgress.Delete(library.ID)
}

// processAudioFile extracts metadata and creates/updates database records
func (s *Scanner) processAudioFile(libraryID int64, path string, info fs.FileInfo, p *ScanProgress) {
	p.CurrentFile = path

	// Extract metadata
	meta, err := ExtractMetadata(path)
	if err != nil {
		log.Printf("Error extracting metadata from %s: %v", path, err)
		p.Errors++
		p.ProcessedFiles++
		s.throttledBroadcast(p)
		return
	}

	// Use filename as title fallback
	if meta.Title == "" {
		meta.Title = filepath.Base(path)
	}

	// Create or get artist
	var artistID *int64
	artistName := meta.Artist
	if artistName == "" {
		artistName = "Unknown Artist"
	}
	artist, err := s.repo.GetOrCreateArtist(libraryID, artistName)
	if err != nil {
		log.Printf("Error creating artist: %v", err)
		p.Errors++
	} else {
		artistID = &artist.ID

		// Update MusicBrainz ID if available
		if meta.MusicBrainzArtistID != "" && artist.MusicBrainzID == "" {
			_ = s.repo.UpdateArtistMusicBrainzID(artist.ID, meta.MusicBrainzArtistID)
		}
	}

	// Create or get album
	var albumID *int64
	albumTitle := meta.Album
	if albumTitle == "" {
		albumTitle = "Unknown Album"
	}
	album, err := s.repo.GetOrCreateAlbum(libraryID, artistID, albumTitle, "", filepath.Dir(path))
	if err != nil {
		log.Printf("Error creating album: %v", err)
		p.Errors++
	} else {
		albumID = &album.ID

		// Extract cover art if album doesn't have one
		if album.CoverArtPath == "" && meta.HasPicture && s.dataDir != "" {
			coverPath, err := ExtractCoverArt(meta, album.ID, s.dataDir)
			if err != nil {
				log.Printf("Error extracting cover art: %v", err)
			} else if coverPath != "" {
				_ = s.repo.UpdateAlbumCoverArt(album.ID, coverPath)
			}
		}
	}

	// Create or update track
	track := &Track{
		LibraryID:     libraryID,
		AlbumID:       albumID,
		ArtistID:      artistID,
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

	existing, _ := s.repo.GetTrackByPath(libraryID, path)
	_, err = s.repo.CreateOrUpdateTrack(track)
	if err != nil {
		log.Printf("Error saving track: %v", err)
		p.Errors++
	} else {
		if existing != nil {
			p.TracksUpdated++
		} else {
			p.TracksAdded++
		}
	}

	p.ProcessedFiles++
	s.throttledBroadcast(p)
}

// countAudioFiles counts total audio files in paths
func (s *Scanner) countAudioFiles(paths []string) int {
	count := 0
	for _, basePath := range paths {
		_ = filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if IsSupportedAudioFile(path) {
				count++
			}
			return nil
		})
	}
	return count
}

// throttledBroadcast sends progress updates at most every 500ms or every 10 files
func (s *Scanner) throttledBroadcast(p *ScanProgress) {
	now := time.Now()
	if p.ProcessedFiles%10 == 0 || now.Sub(s.lastBroadcast) >= 500*time.Millisecond {
		s.broadcastProgress(p)
		s.lastBroadcast = now
	}
}

// broadcastProgress sends current scan progress to all WebSocket clients
func (s *Scanner) broadcastProgress(progress *ScanProgress) {
	if s.hub == nil {
		return
	}

	msg := websocket.Message{
		Type: "scan-progress",
		Data: progress,
	}

	if err := s.hub.Broadcast(msg); err != nil {
		log.Printf("Failed to broadcast scan progress: %v", err)
	}
}

package library

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"audiod/internal/config"
)

// Service handles library business logic
type Service struct {
	repo Repository
}

// NewService creates a new library service
func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// ListLibraries returns all libraries accessible to a user
func (s *Service) ListLibraries(userID int64) ([]*Library, error) {
	return s.repo.ListLibrariesForUser(userID)
}

// CreateLibrary creates a new library and grants owner access to the user
func (s *Service) CreateLibrary(userID int64, name string, paths []string) (*Library, error) {
	// Validate name
	if name == "" {
		return nil, ErrNameRequired
	}

	// Validate: at least one path required
	if len(paths) == 0 {
		return nil, ErrPathsRequired
	}

	// Validate all paths exist and are directories
	if err := validatePaths(paths); err != nil {
		return nil, err
	}

	// Create the library
	library, err := s.repo.CreateLibrary(name, paths)
	if err != nil {
		return nil, err
	}

	// Grant access to the creating user
	err = s.repo.GrantAccess(userID, library.ID)
	if err != nil {
		return nil, err
	}

	// Queue scan for the new library (scanner worker will pick it up)
	if _, err := s.repo.QueueScan(library.ID); err != nil {
		log.Printf("Failed to queue scan for new library %d: %v", library.ID, err)
	}

	return library, nil
}

// StartScan queues a scan for a library
// Returns immediately. Scanner worker will pick up the job.
func (s *Service) StartScan(libraryID int64) error {
	// Check if scan already pending/running for this library
	existing, err := s.repo.GetScanJobByLibrary(libraryID)
	if err != nil {
		return err
	}
	if existing != nil {
		return ErrScanAlreadyInProgress
	}

	// Queue the scan
	_, err = s.repo.QueueScan(libraryID)
	return err
}

// GetScanProgress returns the current scan progress for a library
// Returns ErrNoScanInProgress if no scan is running
func (s *Service) GetScanProgress(libraryID int64) (*ScanProgress, error) {
	job, err := s.repo.GetScanJobByLibrary(libraryID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, ErrNoScanInProgress
	}

	// Convert ScanJob to ScanProgress for API compatibility
	var startedAt time.Time
	if job.StartedAt != nil {
		startedAt = *job.StartedAt
	}

	return &ScanProgress{
		LibraryID:      job.LibraryID,
		Status:         job.Status,
		TotalFiles:     job.TotalFiles,
		ProcessedFiles: job.ProcessedFiles,
		TracksAdded:    job.TracksAdded,
		TracksUpdated:  job.TracksUpdated,
		Errors:         job.Errors,
		CurrentFile:    job.CurrentFile,
		StartedAt:      startedAt,
	}, nil
}

// DeleteLibrary deletes a library and all its associated data
func (s *Service) DeleteLibrary(libraryID int64) error {
	// Verify library exists
	_, err := s.repo.GetLibraryByID(libraryID)
	if err != nil {
		return err
	}

	return s.repo.DeleteLibrary(libraryID)
}

// UpdateLibrary updates a library's name and paths
func (s *Service) UpdateLibrary(libraryID int64, name string, paths []string) (*Library, error) {
	// Validate name
	if name == "" {
		return nil, ErrNameRequired
	}

	// Validate paths
	if len(paths) == 0 {
		return nil, ErrPathsRequired
	}

	// Validate all paths exist and are directories
	if err := validatePaths(paths); err != nil {
		return nil, err
	}

	// Verify library exists
	_, err := s.repo.GetLibraryByID(libraryID)
	if err != nil {
		return nil, err
	}

	library, err := s.repo.UpdateLibrary(libraryID, name, paths)
	if err != nil {
		return nil, err
	}

	// Queue scan to index any new paths (scanner worker will pick it up)
	if _, err := s.repo.QueueScan(library.ID); err != nil {
		log.Printf("Failed to queue scan for updated library %d: %v", library.ID, err)
	}

	return library, nil
}

// AlbumWithArtist combines album data with its artist name for display
type AlbumWithArtist struct {
	Album      *Album
	ArtistName string
}

// DefaultAlbumSort is applied when no sort is specified by the caller.
var DefaultAlbumSort = []SortSpec{
	{Field: SortFieldAlbumArtist, Direction: SortAsc},
	{Field: SortFieldYear, Direction: SortAsc},
}

// ListAlbums returns all albums in a library with artist information
func (s *Service) ListAlbums(libraryID int64, opts ListAlbumsOptions) ([]*AlbumWithArtist, error) {
	if len(opts.SortBy) == 0 {
		opts.SortBy = DefaultAlbumSort
	}
	albums, err := s.repo.ListAlbumsByLibrary(libraryID, opts)
	if err != nil {
		return nil, err
	}

	result := make([]*AlbumWithArtist, len(albums))
	for i, album := range albums {
		albumWithArtist := &AlbumWithArtist{
			Album:      album,
			ArtistName: "",
		}

		// Get artist name if album has an artist
		if album.ArtistID != nil {
			artist, err := s.repo.GetArtistByID(*album.ArtistID)
			if err == nil && artist != nil {
				albumWithArtist.ArtistName = artist.Name
			}
		}

		result[i] = albumWithArtist
	}

	return result, nil
}

// SearchResultLimit caps how many matches per entity type are returned.
const SearchResultLimit = 25

// Search finds albums, artists, and tracks matching the query across a single
// library, ordered by relevance. Matching is full-text (tokenized, accent- and
// case-insensitive, prefix/type-ahead) over title/name plus artist, genre and
// year, so a query finds an album by its artist as well as its title. An empty
// query returns an empty result.
func (s *Service) Search(libraryID int64, query string) (*SearchResult, error) {
	result := &SearchResult{}
	if query == "" {
		return result, nil
	}

	albums, err := s.repo.SearchAlbumsByLibrary(libraryID, query, SearchResultLimit)
	if err != nil {
		return nil, err
	}
	artists, err := s.repo.SearchArtistsByLibrary(libraryID, query, SearchResultLimit)
	if err != nil {
		return nil, err
	}
	tracks, err := s.repo.SearchTracksByLibrary(libraryID, query, SearchResultLimit)
	if err != nil {
		return nil, err
	}

	result.Albums = make([]*AlbumWithArtist, len(albums))
	for i, album := range albums {
		result.Albums[i] = &AlbumWithArtist{Album: album, ArtistName: s.artistName(album.ArtistID)}
	}
	result.Artists = artists
	result.Tracks = make([]*TrackWithArtist, len(tracks))
	for i, track := range tracks {
		result.Tracks[i] = &TrackWithArtist{Track: track, ArtistName: s.artistName(track.ArtistID)}
	}
	return result, nil
}

// artistName resolves an artist id to its display name, returning "" when the
// id is nil (compilations) or the lookup fails.
func (s *Service) artistName(artistID *int64) string {
	if artistID == nil {
		return ""
	}
	artist, err := s.repo.GetArtistByID(*artistID)
	if err != nil || artist == nil {
		return ""
	}
	return artist.Name
}

// GetAlbumCoverPath returns the full file path to an album's cover art
func (s *Service) GetAlbumCoverPath(albumID int64) (string, error) {
	album, err := s.repo.GetAlbumByID(albumID)
	if err != nil {
		return "", err
	}
	if album.CoverArtPath == "" {
		return "", nil
	}
	// Join data directory with relative cover path
	return filepath.Join(config.GetDataDir(), album.CoverArtPath), nil
}

// GetAlbum returns an album by ID with artist information
func (s *Service) GetAlbum(albumID int64) (*AlbumWithArtist, error) {
	album, err := s.repo.GetAlbumByID(albumID)
	if err != nil {
		return nil, err
	}

	result := &AlbumWithArtist{
		Album:      album,
		ArtistName: "",
	}

	// Get artist name if album has an artist
	if album.ArtistID != nil {
		artist, err := s.repo.GetArtistByID(*album.ArtistID)
		if err == nil && artist != nil {
			result.ArtistName = artist.Name
		}
	}

	return result, nil
}

// TrackWithArtist combines track data with its artist name for display
type TrackWithArtist struct {
	Track      *Track
	ArtistName string
}

// ListTracksByAlbum returns all tracks in an album with artist information
func (s *Service) ListTracksByAlbum(albumID int64) ([]*TrackWithArtist, error) {
	tracks, err := s.repo.ListTracksByAlbum(albumID)
	if err != nil {
		return nil, err
	}

	result := make([]*TrackWithArtist, len(tracks))
	for i, track := range tracks {
		trackWithArtist := &TrackWithArtist{
			Track:      track,
			ArtistName: "",
		}

		// Get artist name if track has an artist
		if track.ArtistID != nil {
			artist, err := s.repo.GetArtistByID(*track.ArtistID)
			if err == nil && artist != nil {
				trackWithArtist.ArtistName = artist.Name
			}
		}

		result[i] = trackWithArtist
	}

	return result, nil
}

// validatePaths checks that all paths exist and are directories
func validatePaths(paths []string) error {
	for _, path := range paths {
		if path == "" {
			continue // Skip empty paths
		}

		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("path does not exist: %s", path)
			}
			return fmt.Errorf("cannot access path %s: %w", path, err)
		}

		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", path)
		}
	}
	return nil
}

package library

import (
	"errors"
	"path/filepath"
	"strings"
	"time"
)

// Library represents a music library
type Library struct {
	ID         int64
	Name       string
	Paths      []string
	TrackCount int
	AlbumCount int
}

// Artist represents a music artist
type Artist struct {
	ID            int64
	LibraryID     int64
	Name          string
	SortName      string
	MusicBrainzID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Album represents a music album.
// IsHiRes is derived at query time (true if any track in the album is hi-res),
// not stored as a column.
//
// FolderPath is the directory the album's audio files live in, used to keep
// multiple versions of the same release (e.g. standard + 24-bit) as separate
// rows instead of merging into one. ReleaseType is a derived label (original,
// hi-res, remaster, deluxe, etc.) for UI display.
type Album struct {
	ID            int64
	LibraryID     int64
	ArtistID      *int64 // NULL for compilations
	Title         string
	SortTitle     string
	MusicBrainzID string
	ReleaseDate   string // YYYY or YYYY-MM-DD
	Genre         string
	TotalTracks   int
	TotalDiscs    int
	CoverArtPath  string
	IsCompilation bool
	IsHiRes       bool
	FolderPath    string
	ReleaseType   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// SortField identifies a column to sort albums by.
type SortField string

const (
	SortFieldAlbumArtist SortField = "album-artist"
	SortFieldArtist      SortField = "artist"
	SortFieldAlbumTitle  SortField = "album-title"
	SortFieldYear        SortField = "year"
)

// SortDirection is the ordering direction.
type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

// SortSpec is one ordering level.
type SortSpec struct {
	Field     SortField
	Direction SortDirection
}

// ListAlbumsOptions controls album listing filters and sort order.
type ListAlbumsOptions struct {
	HiResOnly bool
	// SortBy is an ordered list of sort levels (primary first, then tiebreakers).
	// Empty defaults to album-artist asc, year asc (handled by service).
	SortBy []SortSpec
}

// SearchResult groups library search matches by entity type. Albums and tracks
// carry their artist name for display ("album by X"); results are ordered by
// relevance, not alphabetically.
type SearchResult struct {
	Albums  []*AlbumWithArtist
	Artists []*Artist
	Tracks  []*TrackWithArtist
}

// Track represents a single audio track
type Track struct {
	ID            int64
	LibraryID     int64
	AlbumID       *int64
	ArtistID      *int64
	FilePath      string
	FileName      string
	FileSize      int64
	FileModified  time.Time
	Title         string
	SortTitle     string
	TrackNumber   int
	DiscNumber    int
	Duration      int // milliseconds
	Year          int
	Genre         string
	MusicBrainzID string
	Codec         string
	Bitrate       int
	SampleRate    int
	BitDepth      int
	Channels      int
	IsLossless    bool
	IsHiRes       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TrackArtist represents the many-to-many relationship between tracks and artists
type TrackArtist struct {
	TrackID  int64
	ArtistID int64
	Role     string // "primary", "featured", "remixer"
	Position int
}

// SupportedAudioExtensions defines supported audio file formats
var SupportedAudioExtensions = map[string]bool{
	".flac": true,
	".mp3":  true,
	".m4a":  true,
	".ogg":  true,
	".wav":  true,
	".aiff": true,
	".aif":  true,
	".dsf":  true,
}

// IsSupportedAudioFile checks if a file path is a supported audio format
func IsSupportedAudioFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return SupportedAudioExtensions[ext]
}

// ScanStats represents statistics from a library scan
type ScanStats struct {
	FilesScanned int
	TracksAdded  int
	Errors       int
}

// ScanProgress represents real-time scan progress (for WebSocket broadcast)
type ScanProgress struct {
	LibraryID      int64     `json:"libraryId"`
	Status         string    `json:"status"` // "pending", "running", "completed", "failed"
	TotalFiles     int       `json:"totalFiles"`
	ProcessedFiles int       `json:"processedFiles"`
	TracksAdded    int       `json:"tracksAdded"`
	TracksUpdated  int       `json:"tracksUpdated"`
	Errors         int       `json:"errors"`
	CurrentFile    string    `json:"currentFile"`
	StartedAt      time.Time `json:"startedAt"`
}

// ScanJob represents a scan job in the queue (persisted in DB)
type ScanJob struct {
	ID             int64
	LibraryID      int64
	RequestedAt    time.Time
	StartedAt      *time.Time
	UpdatedAt      time.Time // heartbeat for orphan detection
	Status         string    // "pending", "running"
	TotalFiles     int
	ProcessedFiles int
	TracksAdded    int
	TracksUpdated  int
	Errors         int
	CurrentFile    string
}

// Repository interface defines data access methods
type Repository interface {
	// Library methods
	ListLibrariesForUser(userID int64) ([]*Library, error)
	CreateLibrary(name string, paths []string) (*Library, error)
	GrantAccess(userID, libraryID int64) error
	RevokeAccess(userID, libraryID int64) error
	GetLibraryByID(libraryID int64) (*Library, error)
	DeleteLibrary(libraryID int64) error
	UpdateLibrary(libraryID int64, name string, paths []string) (*Library, error)

	// Artist methods
	GetOrCreateArtist(libraryID int64, name string) (*Artist, error)
	UpdateArtistMusicBrainzID(id int64, mbid string) error
	// BackfillArtistSortNames populates sort_name for any artist row where it's
	// missing, using the same article-stripping rule as new inserts. Idempotent.
	BackfillArtistSortNames() (int64, error)

	// Album methods
	GetOrCreateAlbum(libraryID int64, artistID *int64, title string, releaseDate string, folderPath string) (*Album, error)
	GetAlbumByID(albumID int64) (*Album, error)
	UpdateAlbumCoverArt(id int64, coverPath string) error
	ListAlbumsByLibrary(libraryID int64, opts ListAlbumsOptions) ([]*Album, error)
	SearchAlbumsByLibrary(libraryID int64, query string, limit int) ([]*Album, error)
	SearchArtistsByLibrary(libraryID int64, query string, limit int) ([]*Artist, error)
	SearchTracksByLibrary(libraryID int64, query string, limit int) ([]*Track, error)

	// Artist methods (for lookup)
	GetArtistByID(artistID int64) (*Artist, error)

	// Track methods
	CreateOrUpdateTrack(track *Track) (*Track, error)
	GetTrackByID(trackID int64) (*Track, error)
	GetTrackByPath(libraryID int64, filePath string) (*Track, error)
	ListTracksByAlbum(albumID int64) ([]*Track, error)
	ListTracksByLibrary(libraryID int64) ([]*Track, error)
	GetTrackPathsForLibrary(libraryID int64) (map[string]time.Time, error) // path -> modTime

	// Access control
	UserHasLibraryAccess(userID, libraryID int64) (bool, error)

	// Track-Artist relationship
	AddTrackArtist(trackID, artistID int64, role string, position int) error
	GetTrackArtists(trackID int64) ([]*TrackArtist, error)

	// Scan queue methods
	QueueScan(libraryID int64) (*ScanJob, error)
	GetPendingScan() (*ScanJob, error)
	GetScanJobByLibrary(libraryID int64) (*ScanJob, error)
	UpdateScanJob(job *ScanJob) error
	DeleteScanJob(jobID int64) error
	ResetOrphanedJobs(timeout time.Duration) (int64, error)
	// ResetAllRunningJobs unconditionally resets every row with status='running'
	// to 'pending'. Used at worker boot: by definition no worker process is
	// running yet, so any 'running' row is stale regardless of heartbeat.
	ResetAllRunningJobs() (int64, error)
}

// Domain errors
var (
	ErrLibraryNotFound       = errors.New("library not found")
	ErrScanAlreadyInProgress = errors.New("scan already in progress")
	ErrNoScanInProgress      = errors.New("no scan in progress")
	ErrNameRequired          = errors.New("library name is required")
	ErrPathsRequired         = errors.New("at least one path is required")
	ErrArtistNotFound        = errors.New("artist not found")
	ErrAlbumNotFound         = errors.New("album not found")
	ErrTrackNotFound         = errors.New("track not found")
	ErrScanJobNotFound       = errors.New("scan job not found")
)

package playback

import "errors"

// Domain errors
var (
	ErrTrackNotFound = errors.New("track not found")
	ErrAccessDenied  = errors.New("access denied")
)

// State represents the playback state
type State string

const (
	StatePlaying State = "playing"
	StatePaused  State = "paused"
	StateStopped State = "stopped"
)

// SourceType represents what kind of source is being played
type SourceType string

const (
	SourceTypeAlbum    SourceType = "album"
	SourceTypePlaylist SourceType = "playlist"
	SourceTypeArtist   SourceType = "artist"
	SourceTypeTracks   SourceType = "tracks"
)

// Track represents a playable track
type Track struct {
	ID       int64
	Title    string
	AlbumID  int64
	ArtistID int64
	Duration int // seconds
}

// Source represents what we're playing through (album/playlist)
type Source struct {
	Type      SourceType
	ID        int64   // Album/playlist ID
	Remaining []int64 // Track IDs remaining in source
}

// CurrentTrack represents the currently playing track with position
type CurrentTrack struct {
	TrackID  int64
	Position int // seconds into track
}

// QueueItem represents a track in the explicit queue
type QueueItem struct {
	TrackID     int64
	AddedFromID int64      // Album/playlist it was added from (for UI)
	AddedFrom   SourceType // Type of source it was added from
}

// DeviceType represents the type of playback device
type DeviceType string

const (
	DeviceTypeBrowser DeviceType = "browser"
	DeviceTypeMPD     DeviceType = "mpd"
)

// Device represents a playback device
type Device struct {
	ID      string
	Name    string
	Type    DeviceType
	Address string // MPD address (e.g., "192.168.1.10:6600"), empty for browser
	UserID  int64
}

// Domain errors for devices
var (
	ErrDeviceNotFound = errors.New("device not found")
)

// Session represents a user's playback session
type Session struct {
	ID            int64
	UserID        int64
	State         State
	Current       *CurrentTrack
	Queue         []QueueItem    // Explicit queue (Play Next / Add to Queue)
	Source        Source          // What we're playing through
	History       []int64        // Track IDs already played (for going back)
	IsPrivate     bool
	DeviceID      string         // Which device is playing (empty = browser, or MPD device ID)
	DeviceVolumes map[string]int // Per-device volume (device ID → 0-100). "" = browser.
}

package mpd

// Status represents the current state of an MPD player
type Status struct {
	State   string // "play", "pause", "stop"
	Elapsed int    // seconds into current song
	Volume  int    // 0-100
	// SongID is MPD's per-Add identifier for the currently active song.
	// Opaque string (MPD reports it as int but we don't need to parse).
	// Empty when MPD has nothing loaded or is fully idle.
	SongID string
}

// CurrentSong represents the currently loaded song in MPD
type CurrentSong struct {
	File string // URI/path of the song
}

// Client defines the interface for communicating with an MPD server
type Client interface {
	Play() error
	Pause() error
	Stop() error
	Status() (Status, error)
	CurrentSong() (CurrentSong, error)
	SetVolume(volume int) error
	Seek(position int) error
	LoadURL(url string) error // Clear playlist and add URL
	Close() error
}

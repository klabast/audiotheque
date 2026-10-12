package plays

// A play counts as a listen once it ran for half the track or this long,
// whichever comes first (the Last.fm scrobble rule).
const ListenThresholdSeconds = 240

// Stats sums up a user's listening over a period.
type Stats struct {
	Listens          int
	ListeningSeconds int // every play, skips included
	TopTracks        []TrackCount
	TopAlbums        []AlbumCount
	TopArtists       []ArtistCount
}

type TrackCount struct {
	TrackID    int64
	Title      string
	ArtistName string
	AlbumID    int64
	AlbumTitle string
	Plays      int
}

type AlbumCount struct {
	AlbumID    int64
	Title      string
	ArtistName string
	Plays      int
}

type ArtistCount struct {
	ArtistID int64
	Name     string
	Plays    int
}

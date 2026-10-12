package plays

import (
	"maps"
	"slices"
	"testing"
	"time"
)

// catalog is two albums by two artists in a library alice can read, plus a
// track in a library she can't.
type catalog struct {
	fixture
	short, long, other, hidden int64 // track ids
	albumA, albumB             int64
	artistA, artistB           int64
}

func setupCatalog(t *testing.T) catalog {
	t.Helper()
	c := catalog{fixture: setup(t)}
	db := c.db
	var libraryID int64
	if err := db.QueryRow(`SELECT library_id FROM track WHERE id = ?`, c.trackID).Scan(&libraryID); err != nil {
		t.Fatalf("library: %v", err)
	}
	insert(t, db, `INSERT INTO library_access (user_id, library_id) VALUES (?, ?)`, c.userID, libraryID)

	c.artistA = insert(t, db, `INSERT INTO artist (library_id, name) VALUES (?, 'Artist A')`, libraryID)
	c.artistB = insert(t, db, `INSERT INTO artist (library_id, name) VALUES (?, 'Artist B')`, libraryID)
	c.albumA = insert(t, db, `INSERT INTO album (library_id, artist_id, title, folder_path) VALUES (?, ?, 'Album A', '/m/a')`, libraryID, c.artistA)
	c.albumB = insert(t, db, `INSERT INTO album (library_id, artist_id, title, folder_path) VALUES (?, ?, 'Album B', '/m/b')`, libraryID, c.artistB)

	c.short = c.track(t, libraryID, c.albumA, c.artistA, "Short", 200_000)
	c.long = c.track(t, libraryID, c.albumA, c.artistA, "Long", 600_000)
	c.other = c.track(t, libraryID, c.albumB, c.artistB, "Other", 200_000)

	elsewhere := insert(t, db, `INSERT INTO library (name) VALUES ('private')`)
	c.hidden = c.track(t, elsewhere, 0, 0, "Hidden", 200_000)
	return c
}

func (c catalog) track(t *testing.T, libraryID, albumID, artistID int64, title string, durationMs int) int64 {
	t.Helper()
	return insert(t, c.db, `INSERT INTO track (library_id, album_id, artist_id, file_path, file_name, file_size, file_modified, title, duration, codec, sample_rate)
		VALUES (?, NULLIF(?, 0), NULLIF(?, 0), '/m/' || ?, ?, 1, CURRENT_TIMESTAMP, ?, ?, 'flac', 44100)`,
		libraryID, albumID, artistID, title, title, title, durationMs)
}

func (c catalog) play(t *testing.T, trackID int64, seconds int, endedAgo time.Duration) {
	t.Helper()
	endedAt := time.Now().UTC().Add(-endedAgo).Format(time.DateTime)
	insert(t, c.db, `INSERT INTO play (user_id, track_id, played_seconds, ended_at) VALUES (?, ?, ?, ?)`,
		c.userID, trackID, seconds, endedAt)
}

func (c catalog) stats(t *testing.T, since time.Time) *Stats {
	t.Helper()
	stats, err := c.repo.Stats(c.userID, since, 10)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	return stats
}

func trackPlays(s *Stats) map[string]int {
	got := map[string]int{}
	for _, tc := range s.TopTracks {
		got[tc.Title] = tc.Plays
	}
	return got
}

func TestStats_CountsListensNotSkips(t *testing.T) {
	tests := []struct {
		name   string
		track  func(catalog) int64
		played int
		counts bool
	}{
		{"half the track is a listen", func(c catalog) int64 { return c.short }, 100, true},
		{"less than half is a skip", func(c catalog) int64 { return c.short }, 99, false},
		{"four minutes of a long track is a listen", func(c catalog) int64 { return c.long }, 240, true},
		{"under four minutes of a long track is a skip", func(c catalog) int64 { return c.long }, 239, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := setupCatalog(t)
			c.play(t, tt.track(c), tt.played, time.Hour)

			got := c.stats(t, time.Time{})

			if (got.Listens == 1) != tt.counts {
				t.Errorf("listens = %d, want counted=%v", got.Listens, tt.counts)
			}
		})
	}
}

func TestStats_RanksTracksAlbumsAndArtistsByListens(t *testing.T) {
	// Given Other listened to three times, Short twice, Long once
	c := setupCatalog(t)
	for range 3 {
		c.play(t, c.other, 200, time.Hour)
	}
	for range 2 {
		c.play(t, c.short, 200, time.Hour)
	}
	c.play(t, c.long, 600, time.Hour)

	got := c.stats(t, time.Time{})

	var titles []string
	for _, tc := range got.TopTracks {
		titles = append(titles, tc.Title)
	}
	if !slices.Equal(titles, []string{"Other", "Short", "Long"}) {
		t.Errorf("top tracks = %v", titles)
	}
	first := got.TopTracks[0]
	if first.TrackID != c.other || first.ArtistName != "Artist B" || first.AlbumID != c.albumB || first.AlbumTitle != "Album B" || first.Plays != 3 {
		t.Errorf("top track = %+v", first)
	}

	wantAlbums := []AlbumCount{
		{AlbumID: c.albumA, Title: "Album A", ArtistName: "Artist A", Plays: 3},
		{AlbumID: c.albumB, Title: "Album B", ArtistName: "Artist B", Plays: 3},
	}
	if !slices.Equal(got.TopAlbums, wantAlbums) {
		t.Errorf("top albums = %+v, want %+v", got.TopAlbums, wantAlbums)
	}
	wantArtists := []ArtistCount{
		{ArtistID: c.artistA, Name: "Artist A", Plays: 3},
		{ArtistID: c.artistB, Name: "Artist B", Plays: 3},
	}
	if !slices.Equal(got.TopArtists, wantArtists) {
		t.Errorf("top artists = %+v, want %+v", got.TopArtists, wantArtists)
	}
}

func TestStats_TotalsCountListensAndAllTimeSpent(t *testing.T) {
	// Given one listen of 150s and one skip of 20s
	c := setupCatalog(t)
	c.play(t, c.short, 150, time.Hour)
	c.play(t, c.other, 20, time.Hour)

	got := c.stats(t, time.Time{})

	if got.Listens != 1 || got.ListeningSeconds != 170 {
		t.Errorf("listens = %d, seconds = %d; want 1, 170", got.Listens, got.ListeningSeconds)
	}
}

func TestStats_OnlyCountsPlaysSince(t *testing.T) {
	// Given a listen yesterday and one ten days ago
	c := setupCatalog(t)
	c.play(t, c.short, 200, 24*time.Hour)
	c.play(t, c.other, 200, 10*24*time.Hour)

	got := c.stats(t, time.Now().Add(-7*24*time.Hour))

	if want := map[string]int{"Short": 1}; !maps.Equal(trackPlays(got), want) {
		t.Errorf("tracks = %v, want %v", trackPlays(got), want)
	}
}

func TestStats_LeavesOutLibrariesTheUserCannotRead(t *testing.T) {
	// Given a listen of a track in a library alice has no access to
	c := setupCatalog(t)
	c.play(t, c.hidden, 200, time.Hour)

	got := c.stats(t, time.Time{})

	if got.Listens != 0 || len(got.TopTracks) != 0 {
		t.Errorf("stats = %+v, want nothing", got)
	}
}

func TestStats_OnlyCountsTheUsersOwnPlays(t *testing.T) {
	// Given bob listened, alice didn't
	c := setupCatalog(t)
	bob := insert(t, c.db, `INSERT INTO user (username, password_hash) VALUES ('bob', 'x')`)
	if err := c.repo.RecordPlay(bob, c.short, 200); err != nil {
		t.Fatalf("RecordPlay: %v", err)
	}

	got := c.stats(t, time.Time{})

	if got.Listens != 0 {
		t.Errorf("alice listens = %d, want 0", got.Listens)
	}
}

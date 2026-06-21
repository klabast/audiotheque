package library

import (
	"testing"
	"time"
)

// seedSearchLibrary fills a library with a small, varied catalogue used by the
// search behaviour specs: two Pink Floyd albums, one Björk album (accented
// artist), and a track per album.
func seedSearchLibrary(t *testing.T, repo Repository, libraryID int64) {
	t.Helper()

	floyd, err := repo.GetOrCreateArtist(libraryID, "Pink Floyd")
	if err != nil {
		t.Fatalf("create artist Pink Floyd: %v", err)
	}
	bjork, err := repo.GetOrCreateArtist(libraryID, "Björk")
	if err != nil {
		t.Fatalf("create artist Björk: %v", err)
	}

	wall, err := repo.GetOrCreateAlbum(libraryID, &floyd.ID, "The Wall", "1979", "/m/floyd/wall")
	if err != nil {
		t.Fatalf("create album The Wall: %v", err)
	}
	if _, err := repo.GetOrCreateAlbum(libraryID, &floyd.ID, "The Dark Side of the Moon", "1973", "/m/floyd/dsotm"); err != nil {
		t.Fatalf("create album Dark Side: %v", err)
	}
	homogenic, err := repo.GetOrCreateAlbum(libraryID, &bjork.ID, "Homogenic", "1997", "/m/bjork/homogenic")
	if err != nil {
		t.Fatalf("create album Homogenic: %v", err)
	}

	mkTrack := func(album *Album, artist *Artist, title, path string, year int) {
		t.Helper()
		_, err := repo.CreateOrUpdateTrack(&Track{
			LibraryID:    libraryID,
			AlbumID:      &album.ID,
			ArtistID:     &artist.ID,
			FilePath:     path,
			FileName:     title + ".flac",
			FileSize:     1,
			FileModified: time.Unix(0, 0),
			Title:        title,
			Year:         year,
			Codec:        "flac",
			SampleRate:   44100,
			Channels:     2,
		})
		if err != nil {
			t.Fatalf("create track %q: %v", title, err)
		}
	}
	mkTrack(wall, floyd, "Comfortably Numb", "/m/floyd/wall/01.flac", 1979)
	mkTrack(homogenic, bjork, "Jóga", "/m/bjork/homogenic/01.flac", 1997)
}

func albumTitles(albums []*Album) []string {
	out := make([]string, len(albums))
	for i, a := range albums {
		out[i] = a.Title
	}
	return out
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// TestSearchAlbums_ByArtistName is the headline fix: typing an artist surfaces
// their albums even though the query word never appears in the album title.
func TestSearchAlbums_ByArtistName(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	seedSearchLibrary(t, repo, libID)

	albums, err := repo.SearchAlbumsByLibrary(libID, "pink", SearchResultLimit)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	titles := albumTitles(albums)
	if !contains(titles, "The Wall") || !contains(titles, "The Dark Side of the Moon") {
		t.Fatalf("expected both Pink Floyd albums when searching by artist, got %v", titles)
	}
}

// TestSearchAlbums_MultiWord checks that space-separated words are ANDed across
// fields, not treated as one literal substring.
func TestSearchAlbums_MultiWord(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	seedSearchLibrary(t, repo, libID)

	// "pink" matches the artist, "moon" matches the title — neither is a
	// substring of the full credited string, so the old LIKE search found nothing.
	albums, err := repo.SearchAlbumsByLibrary(libID, "pink moon", SearchResultLimit)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	titles := albumTitles(albums)
	if len(titles) != 1 || titles[0] != "The Dark Side of the Moon" {
		t.Fatalf("expected only Dark Side for \"pink moon\", got %v", titles)
	}
}

// TestSearchAlbums_DiacriticsInsensitive checks accent folding: an ASCII query
// finds an accented artist's album.
func TestSearchAlbums_DiacriticsInsensitive(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	seedSearchLibrary(t, repo, libID)

	albums, err := repo.SearchAlbumsByLibrary(libID, "bjork", SearchResultLimit)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !contains(albumTitles(albums), "Homogenic") {
		t.Fatalf("expected Homogenic when searching \"bjork\", got %v", albumTitles(albums))
	}
}

// TestSearchAlbums_Prefix checks type-ahead: a partial word matches.
func TestSearchAlbums_Prefix(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	seedSearchLibrary(t, repo, libID)

	albums, err := repo.SearchAlbumsByLibrary(libID, "homog", SearchResultLimit)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !contains(albumTitles(albums), "Homogenic") {
		t.Fatalf("expected Homogenic for prefix \"homog\", got %v", albumTitles(albums))
	}
}

// TestSearchAlbums_RelevanceRanking checks a title hit ranks above an
// artist-only hit for the same term.
func TestSearchAlbums_RelevanceRanking(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	artist, err := repo.GetOrCreateArtist(libID, "Moonchild")
	if err != nil {
		t.Fatalf("artist: %v", err)
	}
	other, err := repo.GetOrCreateArtist(libID, "Someone Else")
	if err != nil {
		t.Fatalf("artist: %v", err)
	}
	// Album titled "Moon" by a different artist; album by "Moonchild" titled otherwise.
	if _, err := repo.GetOrCreateAlbum(libID, &other.ID, "Moon", "2000", "/m/a"); err != nil {
		t.Fatalf("album: %v", err)
	}
	if _, err := repo.GetOrCreateAlbum(libID, &artist.ID, "Ritual", "2001", "/m/b"); err != nil {
		t.Fatalf("album: %v", err)
	}

	albums, err := repo.SearchAlbumsByLibrary(libID, "moon", SearchResultLimit)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(albums) < 2 {
		t.Fatalf("expected both albums for \"moon\", got %v", albumTitles(albums))
	}
	if albums[0].Title != "Moon" {
		t.Fatalf("expected title match \"Moon\" ranked first, got %v", albumTitles(albums))
	}
}

// TestSearchTracks_ByArtistName checks tracks are also findable by artist.
func TestSearchTracks_ByArtistName(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	seedSearchLibrary(t, repo, libID)

	tracks, err := repo.SearchTracksByLibrary(libID, "bjork", SearchResultLimit)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(tracks) != 1 || tracks[0].Title != "Jóga" {
		t.Fatalf("expected Jóga track for \"bjork\", got %d tracks", len(tracks))
	}
}

// TestSearchArtists_Diacritics checks the artist index folds accents too.
func TestSearchArtists_Diacritics(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	seedSearchLibrary(t, repo, libID)

	artists, err := repo.SearchArtistsByLibrary(libID, "bjork", SearchResultLimit)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(artists) != 1 || artists[0].Name != "Björk" {
		t.Fatalf("expected Björk for \"bjork\", got %v", artists)
	}
}

// TestSearch_LibraryScoped checks results never leak across libraries.
func TestSearch_LibraryScoped(t *testing.T) {
	repo, db, libID := setupTestRepo(t)
	seedSearchLibrary(t, repo, libID)

	res, err := db.Exec("INSERT INTO library (name) VALUES ('other')")
	if err != nil {
		t.Fatalf("seed second library: %v", err)
	}
	otherLib, _ := res.LastInsertId()
	otherArtist, err := repo.GetOrCreateArtist(otherLib, "Pink Floyd")
	if err != nil {
		t.Fatalf("artist: %v", err)
	}
	if _, err := repo.GetOrCreateAlbum(otherLib, &otherArtist.ID, "Animals", "1977", "/o/animals"); err != nil {
		t.Fatalf("album: %v", err)
	}

	albums, err := repo.SearchAlbumsByLibrary(libID, "pink", SearchResultLimit)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if contains(albumTitles(albums), "Animals") {
		t.Fatalf("search leaked across libraries: %v", albumTitles(albums))
	}
}

// TestSearch_NoMatch checks a non-matching query returns no rows, not an error.
func TestSearch_NoMatch(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	seedSearchLibrary(t, repo, libID)

	albums, err := repo.SearchAlbumsByLibrary(libID, "zzzznomatch", SearchResultLimit)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(albums) != 0 {
		t.Fatalf("expected no albums, got %v", albumTitles(albums))
	}
}

// TestSearch_PunctuationOnlyQuery checks a query that tokenizes to nothing is
// handled gracefully (no SQL error, no results).
func TestSearch_PunctuationOnlyQuery(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	seedSearchLibrary(t, repo, libID)

	albums, err := repo.SearchAlbumsByLibrary(libID, "!!! ???", SearchResultLimit)
	if err != nil {
		t.Fatalf("punctuation-only query should not error: %v", err)
	}
	if len(albums) != 0 {
		t.Fatalf("expected no albums for punctuation-only query, got %v", albumTitles(albums))
	}
}

// TestSearchTracks_ReindexedOnUpdate checks the track FTS row tracks metadata
// rewrites: after a rescan changes the title, the new title is searchable and
// the old one is not.
func TestSearchTracks_ReindexedOnUpdate(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	artist, err := repo.GetOrCreateArtist(libID, "Aphex Twin")
	if err != nil {
		t.Fatalf("artist: %v", err)
	}
	album, err := repo.GetOrCreateAlbum(libID, &artist.ID, "Drukqs", "2001", "/m/aphex")
	if err != nil {
		t.Fatalf("album: %v", err)
	}
	tr := &Track{
		LibraryID: libID, AlbumID: &album.ID, ArtistID: &artist.ID,
		FilePath: "/m/aphex/01.flac", FileName: "01.flac", FileSize: 1,
		FileModified: time.Unix(0, 0), Title: "Avril Wrongnametypo",
		Codec: "flac", SampleRate: 44100, Channels: 2,
	}
	if _, err := repo.CreateOrUpdateTrack(tr); err != nil {
		t.Fatalf("create track: %v", err)
	}

	tr.Title = "Avril 14th"
	if _, err := repo.CreateOrUpdateTrack(tr); err != nil {
		t.Fatalf("update track: %v", err)
	}

	hit, err := repo.SearchTracksByLibrary(libID, "avril 14th", SearchResultLimit)
	if err != nil {
		t.Fatalf("search new title: %v", err)
	}
	if len(hit) != 1 {
		t.Fatalf("expected updated title to be searchable, got %d", len(hit))
	}
	stale, err := repo.SearchTracksByLibrary(libID, "wrongnametypo", SearchResultLimit)
	if err != nil {
		t.Fatalf("search old title: %v", err)
	}
	if len(stale) != 0 {
		t.Fatalf("old title should no longer match after reindex, got %d", len(stale))
	}
}

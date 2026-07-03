package library

import (
	"testing"
	"time"
)

func mustCreateAlbum(t *testing.T, repo Repository, libID int64) (*Artist, *Album) {
	t.Helper()
	artist, err := repo.GetOrCreateArtist(libID, "Track Test Artist")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	album, err := repo.GetOrCreateAlbum(libID, &artist.ID, "Track Test Album", "2020", "/m/track-test")
	if err != nil {
		t.Fatalf("create album: %v", err)
	}
	return artist, album
}

// TestCreateOrUpdateTrack_CreatesThenUpdatesInPlace checks the upsert
// behaviour: same file_path updates the existing row (same ID) rather than
// inserting a duplicate.
func TestCreateOrUpdateTrack_CreatesThenUpdatesInPlace(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	artist, album := mustCreateAlbum(t, repo, libID)

	tr := &Track{
		LibraryID: libID, AlbumID: &album.ID, ArtistID: &artist.ID,
		FilePath: "/m/track-test/01.flac", FileName: "01.flac", FileSize: 100,
		FileModified: time.Unix(1000, 0), Title: "Original Title",
		TrackNumber: 1, Codec: "flac", SampleRate: 44100, Channels: 2,
	}
	created, err := repo.CreateOrUpdateTrack(tr)
	if err != nil {
		t.Fatalf("create track: %v", err)
	}
	if created.ID == 0 {
		t.Fatalf("expected non-zero track ID")
	}

	tr.Title = "Updated Title"
	tr.FileSize = 200
	updated, err := repo.CreateOrUpdateTrack(tr)
	if err != nil {
		t.Fatalf("update track: %v", err)
	}
	if updated.ID != created.ID {
		t.Fatalf("expected same track ID on update, got %d vs %d", created.ID, updated.ID)
	}

	fetched, err := repo.GetTrackByID(created.ID)
	if err != nil {
		t.Fatalf("get track: %v", err)
	}
	if fetched.Title != "Updated Title" || fetched.FileSize != 200 {
		t.Fatalf("expected updated fields, got title=%q size=%d", fetched.Title, fetched.FileSize)
	}
}

// TestGetTrackByID_NotFound checks the sentinel error for a missing ID.
func TestGetTrackByID_NotFound(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	_, err := repo.GetTrackByID(99999)
	if err != ErrTrackNotFound {
		t.Fatalf("expected ErrTrackNotFound, got %v", err)
	}
}

// TestGetTrackByPath_NotFound checks the sentinel error for a missing path
// within a library that does have other tracks.
func TestGetTrackByPath_NotFound(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	artist, album := mustCreateAlbum(t, repo, libID)
	if _, err := repo.CreateOrUpdateTrack(&Track{
		LibraryID: libID, AlbumID: &album.ID, ArtistID: &artist.ID,
		FilePath: "/m/track-test/01.flac", FileName: "01.flac", FileSize: 1,
		Title: "T1", Codec: "flac", SampleRate: 44100, Channels: 2,
	}); err != nil {
		t.Fatalf("create track: %v", err)
	}

	_, err := repo.GetTrackByPath(libID, "/m/track-test/does-not-exist.flac")
	if err != ErrTrackNotFound {
		t.Fatalf("expected ErrTrackNotFound, got %v", err)
	}
}

// TestGetTrackByPath_ScopedToLibrary checks the same file_path in a different
// library doesn't collide.
func TestGetTrackByPath_ScopedToLibrary(t *testing.T) {
	repo, db, libID := setupTestRepo(t)
	artist, album := mustCreateAlbum(t, repo, libID)
	if _, err := repo.CreateOrUpdateTrack(&Track{
		LibraryID: libID, AlbumID: &album.ID, ArtistID: &artist.ID,
		FilePath: "/shared/path.flac", FileName: "path.flac", FileSize: 1,
		Title: "Lib One Track", Codec: "flac", SampleRate: 44100, Channels: 2,
	}); err != nil {
		t.Fatalf("create track: %v", err)
	}

	res, err := db.Exec("INSERT INTO library (name) VALUES ('other')")
	if err != nil {
		t.Fatalf("seed second library: %v", err)
	}
	otherLib, _ := res.LastInsertId()

	_, err = repo.GetTrackByPath(otherLib, "/shared/path.flac")
	if err != ErrTrackNotFound {
		t.Fatalf("expected ErrTrackNotFound in other library, got %v", err)
	}

	found, err := repo.GetTrackByPath(libID, "/shared/path.flac")
	if err != nil {
		t.Fatalf("expected track found in owning library: %v", err)
	}
	if found.Title != "Lib One Track" {
		t.Fatalf("expected Lib One Track, got %q", found.Title)
	}
}

// TestListTracksByAlbum_OrderedByDiscThenTrackNumber checks the ordering
// contract the UI relies on for tracklist display.
func TestListTracksByAlbum_OrderedByDiscThenTrackNumber(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	artist, album := mustCreateAlbum(t, repo, libID)

	mk := func(path string, disc, num int) {
		if _, err := repo.CreateOrUpdateTrack(&Track{
			LibraryID: libID, AlbumID: &album.ID, ArtistID: &artist.ID,
			FilePath: path, FileName: path, FileSize: 1,
			Title: path, DiscNumber: disc, TrackNumber: num,
			Codec: "flac", SampleRate: 44100, Channels: 2,
		}); err != nil {
			t.Fatalf("create track %q: %v", path, err)
		}
	}
	mk("/m/track-test/d2t1.flac", 2, 1)
	mk("/m/track-test/d1t2.flac", 1, 2)
	mk("/m/track-test/d1t1.flac", 1, 1)

	tracks, err := repo.ListTracksByAlbum(album.ID)
	if err != nil {
		t.Fatalf("list tracks: %v", err)
	}
	if len(tracks) != 3 {
		t.Fatalf("expected 3 tracks, got %d", len(tracks))
	}
	want := []string{"/m/track-test/d1t1.flac", "/m/track-test/d1t2.flac", "/m/track-test/d2t1.flac"}
	for i, w := range want {
		if tracks[i].FilePath != w {
			t.Fatalf("track order mismatch at %d: want %q, got %q", i, w, tracks[i].FilePath)
		}
	}
}

// TestListTracksByLibrary_OrderedByTitle checks the library-wide listing is
// alphabetical by title.
func TestListTracksByLibrary_OrderedByTitle(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	artist, album := mustCreateAlbum(t, repo, libID)

	mk := func(path, title string) {
		if _, err := repo.CreateOrUpdateTrack(&Track{
			LibraryID: libID, AlbumID: &album.ID, ArtistID: &artist.ID,
			FilePath: path, FileName: path, FileSize: 1,
			Title: title, Codec: "flac", SampleRate: 44100, Channels: 2,
		}); err != nil {
			t.Fatalf("create track %q: %v", path, err)
		}
	}
	mk("/m/track-test/z.flac", "Zebra Song")
	mk("/m/track-test/a.flac", "Aardvark Song")

	tracks, err := repo.ListTracksByLibrary(libID)
	if err != nil {
		t.Fatalf("list tracks: %v", err)
	}
	if len(tracks) != 2 || tracks[0].Title != "Aardvark Song" || tracks[1].Title != "Zebra Song" {
		t.Fatalf("expected alphabetical order, got %v", []string{tracks[0].Title, tracks[1].Title})
	}
}

// TestGetTrackPathsForLibrary_MapsPathToModTime is the behaviour spec for
// incremental scanning: the scanner diffs this map against the filesystem to
// skip unchanged files.
func TestGetTrackPathsForLibrary_MapsPathToModTime(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	artist, album := mustCreateAlbum(t, repo, libID)

	modTime := time.Unix(123456789, 0).UTC()
	if _, err := repo.CreateOrUpdateTrack(&Track{
		LibraryID: libID, AlbumID: &album.ID, ArtistID: &artist.ID,
		FilePath: "/m/track-test/01.flac", FileName: "01.flac", FileSize: 1,
		FileModified: modTime, Title: "T1", Codec: "flac", SampleRate: 44100, Channels: 2,
	}); err != nil {
		t.Fatalf("create track: %v", err)
	}

	paths, err := repo.GetTrackPathsForLibrary(libID)
	if err != nil {
		t.Fatalf("get track paths: %v", err)
	}
	got, ok := paths["/m/track-test/01.flac"]
	if !ok {
		t.Fatalf("expected path in map, got %v", paths)
	}
	if !got.Equal(modTime) {
		t.Fatalf("expected mod time %v, got %v", modTime, got)
	}
}

// TestAddTrackArtist_UpsertsPositionOnConflict checks the ON CONFLICT clause:
// re-adding the same (track, artist, role) updates position instead of
// erroring or duplicating.
func TestAddTrackArtist_UpsertsPositionOnConflict(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	artist, album := mustCreateAlbum(t, repo, libID)
	track, err := repo.CreateOrUpdateTrack(&Track{
		LibraryID: libID, AlbumID: &album.ID, ArtistID: &artist.ID,
		FilePath: "/m/track-test/01.flac", FileName: "01.flac", FileSize: 1,
		Title: "T1", Codec: "flac", SampleRate: 44100, Channels: 2,
	})
	if err != nil {
		t.Fatalf("create track: %v", err)
	}

	if err := repo.AddTrackArtist(track.ID, artist.ID, "primary", 0); err != nil {
		t.Fatalf("add track artist: %v", err)
	}
	if err := repo.AddTrackArtist(track.ID, artist.ID, "primary", 5); err != nil {
		t.Fatalf("re-add track artist with new position: %v", err)
	}

	artists, err := repo.GetTrackArtists(track.ID)
	if err != nil {
		t.Fatalf("get track artists: %v", err)
	}
	if len(artists) != 1 {
		t.Fatalf("expected exactly 1 track-artist row (upsert, not duplicate), got %d", len(artists))
	}
	if artists[0].Position != 5 {
		t.Fatalf("expected position updated to 5, got %d", artists[0].Position)
	}
}

// TestGetTrackArtists_OrderedByPosition checks featured/remixer credits come
// back in the intended display order.
func TestGetTrackArtists_OrderedByPosition(t *testing.T) {
	repo, _, libID := setupTestRepo(t)
	primary, album := mustCreateAlbum(t, repo, libID)
	featured, err := repo.GetOrCreateArtist(libID, "Featured Artist")
	if err != nil {
		t.Fatalf("create featured artist: %v", err)
	}
	track, err := repo.CreateOrUpdateTrack(&Track{
		LibraryID: libID, AlbumID: &album.ID, ArtistID: &primary.ID,
		FilePath: "/m/track-test/01.flac", FileName: "01.flac", FileSize: 1,
		Title: "T1", Codec: "flac", SampleRate: 44100, Channels: 2,
	})
	if err != nil {
		t.Fatalf("create track: %v", err)
	}

	if err := repo.AddTrackArtist(track.ID, featured.ID, "featured", 1); err != nil {
		t.Fatalf("add featured artist: %v", err)
	}
	if err := repo.AddTrackArtist(track.ID, primary.ID, "primary", 0); err != nil {
		t.Fatalf("add primary artist: %v", err)
	}

	artists, err := repo.GetTrackArtists(track.ID)
	if err != nil {
		t.Fatalf("get track artists: %v", err)
	}
	if len(artists) != 2 {
		t.Fatalf("expected 2 track-artist rows, got %d", len(artists))
	}
	if artists[0].ArtistID != primary.ID || artists[0].Role != "primary" {
		t.Fatalf("expected primary artist first (position 0), got %+v", artists[0])
	}
	if artists[1].ArtistID != featured.ID || artists[1].Role != "featured" {
		t.Fatalf("expected featured artist second (position 1), got %+v", artists[1])
	}
}

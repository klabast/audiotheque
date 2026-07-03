package library

import "testing"

// TestGetAlbumByID_NotFound checks the sentinel error for a missing ID.
func TestGetAlbumByID_NotFound(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	_, err := repo.GetAlbumByID(99999)
	if err != ErrAlbumNotFound {
		t.Fatalf("expected ErrAlbumNotFound, got %v", err)
	}
}

// TestGetOrCreateAlbum_CompilationHasNilArtistID checks that passing a nil
// artistID (compilation) round-trips as nil, not as some sentinel zero value.
func TestGetOrCreateAlbum_CompilationHasNilArtistID(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	album, err := repo.GetOrCreateAlbum(libID, nil, "Various Artists Vol. 1", "2000", "/m/various")
	if err != nil {
		t.Fatalf("create compilation album: %v", err)
	}
	if album.ArtistID != nil {
		t.Fatalf("expected nil ArtistID for compilation, got %v", *album.ArtistID)
	}

	fetched, err := repo.GetAlbumByID(album.ID)
	if err != nil {
		t.Fatalf("get album: %v", err)
	}
	if fetched.ArtistID != nil {
		t.Fatalf("expected nil ArtistID on re-fetch, got %v", *fetched.ArtistID)
	}
}

// TestUpdateAlbumCoverArt_PersistsAndReturnsNotFoundForMissingID mirrors the
// artist mbid test: happy path plus not-found branch of the same statement.
func TestUpdateAlbumCoverArt_PersistsAndReturnsNotFoundForMissingID(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	artist, err := repo.GetOrCreateArtist(libID, "Boards of Canada")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	album, err := repo.GetOrCreateAlbum(libID, &artist.ID, "Music Has the Right to Children", "1998", "/m/boc")
	if err != nil {
		t.Fatalf("create album: %v", err)
	}

	if err := repo.UpdateAlbumCoverArt(album.ID, "/covers/boc.jpg"); err != nil {
		t.Fatalf("update cover art: %v", err)
	}
	fetched, err := repo.GetAlbumByID(album.ID)
	if err != nil {
		t.Fatalf("get album: %v", err)
	}
	if fetched.CoverArtPath != "/covers/boc.jpg" {
		t.Fatalf("expected cover art path %q, got %q", "/covers/boc.jpg", fetched.CoverArtPath)
	}

	if err := repo.UpdateAlbumCoverArt(99999, "/covers/nope.jpg"); err != ErrAlbumNotFound {
		t.Fatalf("expected ErrAlbumNotFound for missing album, got %v", err)
	}
}

// TestGetAlbumByID_IsHiResReflectsTracks checks IsHiRes is computed from
// whether any of the album's tracks is hi-res, not a stored column.
func TestGetAlbumByID_IsHiResReflectsTracks(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	artist, err := repo.GetOrCreateArtist(libID, "Four Tet")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	album, err := repo.GetOrCreateAlbum(libID, &artist.ID, "Rounds", "2003", "/m/fourtet")
	if err != nil {
		t.Fatalf("create album: %v", err)
	}

	notYetHiRes, err := repo.GetAlbumByID(album.ID)
	if err != nil {
		t.Fatalf("get album: %v", err)
	}
	if notYetHiRes.IsHiRes {
		t.Fatalf("expected album not hi-res before any hi-res track")
	}

	if _, err := repo.CreateOrUpdateTrack(&Track{
		LibraryID: libID, AlbumID: &album.ID, ArtistID: &artist.ID,
		FilePath: "/m/fourtet/01.flac", FileName: "01.flac", FileSize: 1,
		Title: "Hangdog", Codec: "flac", SampleRate: 96000, BitDepth: 24,
		Channels: 2, IsHiRes: true,
	}); err != nil {
		t.Fatalf("create hi-res track: %v", err)
	}

	nowHiRes, err := repo.GetAlbumByID(album.ID)
	if err != nil {
		t.Fatalf("get album: %v", err)
	}
	if !nowHiRes.IsHiRes {
		t.Fatalf("expected album hi-res after adding a hi-res track")
	}
}

// TestListAlbumsByLibrary_HiResOnlyFiltersNonHiResAlbums checks the
// opts.HiResOnly filter excludes albums with no hi-res track.
func TestListAlbumsByLibrary_HiResOnlyFiltersNonHiResAlbums(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	artist, err := repo.GetOrCreateArtist(libID, "Artist")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	lossy, err := repo.GetOrCreateAlbum(libID, &artist.ID, "Lossy Album", "2010", "/m/lossy")
	if err != nil {
		t.Fatalf("create lossy album: %v", err)
	}
	hires, err := repo.GetOrCreateAlbum(libID, &artist.ID, "HiRes Album", "2011", "/m/hires")
	if err != nil {
		t.Fatalf("create hires album: %v", err)
	}
	if _, err := repo.CreateOrUpdateTrack(&Track{
		LibraryID: libID, AlbumID: &lossy.ID, ArtistID: &artist.ID,
		FilePath: "/m/lossy/01.mp3", FileName: "01.mp3", FileSize: 1,
		Title: "T1", Codec: "mp3", SampleRate: 44100, Channels: 2,
	}); err != nil {
		t.Fatalf("create track: %v", err)
	}
	if _, err := repo.CreateOrUpdateTrack(&Track{
		LibraryID: libID, AlbumID: &hires.ID, ArtistID: &artist.ID,
		FilePath: "/m/hires/01.flac", FileName: "01.flac", FileSize: 1,
		Title: "T2", Codec: "flac", SampleRate: 96000, BitDepth: 24,
		Channels: 2, IsHiRes: true,
	}); err != nil {
		t.Fatalf("create track: %v", err)
	}

	all, err := repo.ListAlbumsByLibrary(libID, ListAlbumsOptions{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 albums without filter, got %d", len(all))
	}

	hiResOnly, err := repo.ListAlbumsByLibrary(libID, ListAlbumsOptions{HiResOnly: true})
	if err != nil {
		t.Fatalf("list hi-res only: %v", err)
	}
	if len(hiResOnly) != 1 || hiResOnly[0].Title != "HiRes Album" {
		t.Fatalf("expected only HiRes Album, got %v", albumTitles(hiResOnly))
	}
}

// TestListAlbumsByLibrary_SortOrders is table-driven over the sort fields
// exposed to the UI: album-artist, album-title, year — each ascending and
// descending.
func TestListAlbumsByLibrary_SortOrders(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	artistA, err := repo.GetOrCreateArtist(libID, "Aardvark")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	artistZ, err := repo.GetOrCreateArtist(libID, "Zebra")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	if _, err := repo.GetOrCreateAlbum(libID, &artistZ.ID, "Alpha Title", "2020", "/m/za"); err != nil {
		t.Fatalf("create album: %v", err)
	}
	if _, err := repo.GetOrCreateAlbum(libID, &artistA.ID, "Zulu Title", "2010", "/m/az"); err != nil {
		t.Fatalf("create album: %v", err)
	}

	tests := []struct {
		name      string
		sortBy    []SortSpec
		wantFirst string
	}{
		{
			name:      "album-artist asc",
			sortBy:    []SortSpec{{Field: SortFieldAlbumArtist, Direction: SortAsc}},
			wantFirst: "Zulu Title", // credited to Aardvark, sorts first
		},
		{
			name:      "album-artist desc",
			sortBy:    []SortSpec{{Field: SortFieldAlbumArtist, Direction: SortDesc}},
			wantFirst: "Alpha Title", // credited to Zebra, sorts first descending
		},
		{
			name:      "album-title asc",
			sortBy:    []SortSpec{{Field: SortFieldAlbumTitle, Direction: SortAsc}},
			wantFirst: "Alpha Title",
		},
		{
			name:      "album-title desc",
			sortBy:    []SortSpec{{Field: SortFieldAlbumTitle, Direction: SortDesc}},
			wantFirst: "Zulu Title",
		},
		{
			name:      "year asc",
			sortBy:    []SortSpec{{Field: SortFieldYear, Direction: SortAsc}},
			wantFirst: "Zulu Title", // 2010
		},
		{
			name:      "year desc",
			sortBy:    []SortSpec{{Field: SortFieldYear, Direction: SortDesc}},
			wantFirst: "Alpha Title", // 2020
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			albums, err := repo.ListAlbumsByLibrary(libID, ListAlbumsOptions{SortBy: tc.sortBy})
			if err != nil {
				t.Fatalf("list albums: %v", err)
			}
			if len(albums) == 0 || albums[0].Title != tc.wantFirst {
				t.Fatalf("expected %q first, got %v", tc.wantFirst, albumTitles(albums))
			}
		})
	}
}

// TestListAlbumsByLibrary_DefaultSortWhenNoSpecGiven checks the safety-net
// default (album-artist asc) applies when the caller passes no SortBy.
func TestListAlbumsByLibrary_DefaultSortWhenNoSpecGiven(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	artistA, err := repo.GetOrCreateArtist(libID, "Aardvark")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	artistZ, err := repo.GetOrCreateArtist(libID, "Zebra")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	if _, err := repo.GetOrCreateAlbum(libID, &artistZ.ID, "Z Album", "2020", "/m/z"); err != nil {
		t.Fatalf("create album: %v", err)
	}
	if _, err := repo.GetOrCreateAlbum(libID, &artistA.ID, "A Album", "2010", "/m/a"); err != nil {
		t.Fatalf("create album: %v", err)
	}

	albums, err := repo.ListAlbumsByLibrary(libID, ListAlbumsOptions{})
	if err != nil {
		t.Fatalf("list albums: %v", err)
	}
	if len(albums) != 2 || albums[0].Title != "A Album" {
		t.Fatalf("expected default sort to put Aardvark's album first, got %v", albumTitles(albums))
	}
}

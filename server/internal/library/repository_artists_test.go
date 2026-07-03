package library

import "testing"

// TestGetOrCreateArtist_IsIdempotentByName checks a second call with the same
// name returns the same row instead of inserting a duplicate.
func TestGetOrCreateArtist_IsIdempotentByName(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	first, err := repo.GetOrCreateArtist(libID, "Radiohead")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	second, err := repo.GetOrCreateArtist(libID, "Radiohead")
	if err != nil {
		t.Fatalf("re-fetch artist: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected same artist row, got %d vs %d", first.ID, second.ID)
	}
}

// TestGetOrCreateArtist_ComputesSortName checks the sort_name is derived from
// the name at insert time (article-stripping) rather than left blank.
func TestGetOrCreateArtist_ComputesSortName(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	artist, err := repo.GetOrCreateArtist(libID, "The Beatles")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	if artist.SortName != computeSortName("The Beatles") {
		t.Fatalf("expected sort name %q, got %q", computeSortName("The Beatles"), artist.SortName)
	}
}

// TestGetArtistByID_NotFound checks the sentinel error for a missing ID.
func TestGetArtistByID_NotFound(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	_, err := repo.GetArtistByID(99999)
	if err != ErrArtistNotFound {
		t.Fatalf("expected ErrArtistNotFound, got %v", err)
	}
}

// TestUpdateArtistMusicBrainzID_PersistsAndReturnsNotFoundForMissingID covers
// both the happy path and the not-found path in one behaviour spec since
// they're two branches of the same statement.
func TestUpdateArtistMusicBrainzID_PersistsAndReturnsNotFoundForMissingID(t *testing.T) {
	repo, _, libID := setupTestRepo(t)

	artist, err := repo.GetOrCreateArtist(libID, "Aphex Twin")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}

	if err := repo.UpdateArtistMusicBrainzID(artist.ID, "mbid-123"); err != nil {
		t.Fatalf("update mbid: %v", err)
	}
	fetched, err := repo.GetArtistByID(artist.ID)
	if err != nil {
		t.Fatalf("get artist: %v", err)
	}
	if fetched.MusicBrainzID != "mbid-123" {
		t.Fatalf("expected mbid %q, got %q", "mbid-123", fetched.MusicBrainzID)
	}

	if err := repo.UpdateArtistMusicBrainzID(99999, "mbid-456"); err != ErrArtistNotFound {
		t.Fatalf("expected ErrArtistNotFound for missing artist, got %v", err)
	}
}

// TestBackfillArtistSortNames_FillsMissingButLeavesExisting checks the
// backfill only touches rows with a NULL/empty sort_name, and reports the
// count of rows it actually updated.
func TestBackfillArtistSortNames_FillsMissingButLeavesExisting(t *testing.T) {
	repo, db, libID := setupTestRepo(t)

	// Artist created normally already has a sort_name computed.
	withSortName, err := repo.GetOrCreateArtist(libID, "The Who")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}

	// Simulate a legacy row with no sort_name (e.g. imported before the
	// column existed).
	res, err := db.Exec(`INSERT INTO artist (library_id, name, sort_name) VALUES (?, ?, NULL)`,
		libID, "The Rolling Stones")
	if err != nil {
		t.Fatalf("seed legacy artist: %v", err)
	}
	legacyID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("legacy artist id: %v", err)
	}

	n, err := repo.BackfillArtistSortNames()
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 row backfilled, got %d", n)
	}

	legacy, err := repo.GetArtistByID(legacyID)
	if err != nil {
		t.Fatalf("get legacy artist: %v", err)
	}
	if legacy.SortName != computeSortName("The Rolling Stones") {
		t.Fatalf("expected backfilled sort name %q, got %q",
			computeSortName("The Rolling Stones"), legacy.SortName)
	}

	untouched, err := repo.GetArtistByID(withSortName.ID)
	if err != nil {
		t.Fatalf("get untouched artist: %v", err)
	}
	if untouched.SortName != withSortName.SortName {
		t.Fatalf("expected untouched artist's sort name unchanged, got %q vs %q",
			untouched.SortName, withSortName.SortName)
	}

	// Idempotent: running again finds nothing left to backfill.
	again, err := repo.BackfillArtistSortNames()
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if again != 0 {
		t.Fatalf("expected 0 rows on second backfill, got %d", again)
	}
}

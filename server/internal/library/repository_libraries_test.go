package library

import (
	"database/sql"
	"testing"
)

// seedUser inserts a minimal user row and returns its ID. library_access has
// a foreign key to user, so access-control tests need a real user row.
func seedUser(t *testing.T, db *sql.DB, username string) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO user (username, password_hash) VALUES (?, 'x')`, username)
	if err != nil {
		t.Fatalf("seed user %q: %v", username, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed user id: %v", err)
	}
	return id
}

// TestCreateLibrary_PersistsNameAndPaths checks the library and its paths are
// written and come back on a subsequent read.
func TestCreateLibrary_PersistsNameAndPaths(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	lib, err := repo.CreateLibrary("My Music", []string{"/music/a", "/music/b"})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	if lib.ID == 0 {
		t.Fatalf("expected non-zero library ID")
	}

	fetched, err := repo.GetLibraryByID(lib.ID)
	if err != nil {
		t.Fatalf("get library: %v", err)
	}
	if fetched.Name != "My Music" {
		t.Fatalf("expected name %q, got %q", "My Music", fetched.Name)
	}
	if len(fetched.Paths) != 2 || fetched.Paths[0] != "/music/a" || fetched.Paths[1] != "/music/b" {
		t.Fatalf("expected paths [/music/a /music/b], got %v", fetched.Paths)
	}
}

// TestGetLibraryByID_NotFound checks the sentinel error is returned for a
// missing ID rather than a raw sql.ErrNoRows.
func TestGetLibraryByID_NotFound(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	_, err := repo.GetLibraryByID(99999)
	if err != ErrLibraryNotFound {
		t.Fatalf("expected ErrLibraryNotFound, got %v", err)
	}
}

// TestUpdateLibrary_ReplacesNameAndPaths checks the old paths are fully
// replaced, not merged with the new ones.
func TestUpdateLibrary_ReplacesNameAndPaths(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	lib, err := repo.CreateLibrary("Original", []string{"/old"})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	updated, err := repo.UpdateLibrary(lib.ID, "Renamed", []string{"/new/one", "/new/two"})
	if err != nil {
		t.Fatalf("update library: %v", err)
	}
	if updated.Name != "Renamed" {
		t.Fatalf("expected renamed library, got %q", updated.Name)
	}

	fetched, err := repo.GetLibraryByID(lib.ID)
	if err != nil {
		t.Fatalf("get library: %v", err)
	}
	if len(fetched.Paths) != 2 || fetched.Paths[0] != "/new/one" || fetched.Paths[1] != "/new/two" {
		t.Fatalf("expected replaced paths [/new/one /new/two], got %v", fetched.Paths)
	}
}

// TestUpdateLibrary_NotFound checks updating a non-existent library returns
// the sentinel error instead of silently succeeding.
func TestRepository_UpdateLibrary_NotFound(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	_, err := repo.UpdateLibrary(99999, "Nope", []string{"/x"})
	if err != ErrLibraryNotFound {
		t.Fatalf("expected ErrLibraryNotFound, got %v", err)
	}
}

// TestDeleteLibrary_RemovesAccessAndPaths checks a delete cascades to
// library_access and library_path rows (deleted manually inside a tx, not via
// FK cascade), and that a second delete on the same ID reports not-found.
func TestDeleteLibrary_RemovesAccessAndPaths(t *testing.T) {
	repo, db, _ := setupTestRepo(t)
	userID := seedUser(t, db, "delete-lib-user")

	lib, err := repo.CreateLibrary("Temp", []string{"/temp"})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	if err := repo.GrantAccess(userID, lib.ID); err != nil {
		t.Fatalf("grant access: %v", err)
	}

	if err := repo.DeleteLibrary(lib.ID); err != nil {
		t.Fatalf("delete library: %v", err)
	}

	if _, err := repo.GetLibraryByID(lib.ID); err != ErrLibraryNotFound {
		t.Fatalf("expected library gone, got err=%v", err)
	}

	has, err := repo.UserHasLibraryAccess(userID, lib.ID)
	if err != nil {
		t.Fatalf("check access: %v", err)
	}
	if has {
		t.Fatalf("expected access revoked after library delete")
	}

	if err := repo.DeleteLibrary(lib.ID); err != ErrLibraryNotFound {
		t.Fatalf("expected ErrLibraryNotFound on second delete, got %v", err)
	}
}

// TestGrantAndRevokeAccess_ControlsUserHasLibraryAccess is the behaviour spec
// for the grant/revoke/check trio.
func TestGrantAndRevokeAccess_ControlsUserHasLibraryAccess(t *testing.T) {
	repo, db, libID := setupTestRepo(t)

	userID := seedUser(t, db, "grant-revoke-user")

	has, err := repo.UserHasLibraryAccess(userID, libID)
	if err != nil {
		t.Fatalf("check access: %v", err)
	}
	if has {
		t.Fatalf("expected no access before grant")
	}

	if err := repo.GrantAccess(userID, libID); err != nil {
		t.Fatalf("grant access: %v", err)
	}
	has, err = repo.UserHasLibraryAccess(userID, libID)
	if err != nil {
		t.Fatalf("check access: %v", err)
	}
	if !has {
		t.Fatalf("expected access after grant")
	}

	if err := repo.RevokeAccess(userID, libID); err != nil {
		t.Fatalf("revoke access: %v", err)
	}
	has, err = repo.UserHasLibraryAccess(userID, libID)
	if err != nil {
		t.Fatalf("check access: %v", err)
	}
	if has {
		t.Fatalf("expected no access after revoke")
	}
}

// TestListLibrariesForUser_ScopedToGrantedAccess checks the listing only
// returns libraries the user has been granted access to, with paths and
// counts attached.
func TestListLibrariesForUser_ScopedToGrantedAccess(t *testing.T) {
	repo, db, _ := setupTestRepo(t)

	userID := seedUser(t, db, "list-libs-user")

	granted, err := repo.CreateLibrary("Granted", []string{"/g1", "/g2"})
	if err != nil {
		t.Fatalf("create granted library: %v", err)
	}
	if err := repo.GrantAccess(userID, granted.ID); err != nil {
		t.Fatalf("grant access: %v", err)
	}

	if _, err := repo.CreateLibrary("NotGranted", []string{"/n1"}); err != nil {
		t.Fatalf("create ungranted library: %v", err)
	}

	artist, err := repo.GetOrCreateArtist(granted.ID, "Some Artist")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	album, err := repo.GetOrCreateAlbum(granted.ID, &artist.ID, "Some Album", "2020", "/g1/album")
	if err != nil {
		t.Fatalf("create album: %v", err)
	}
	if _, err := repo.CreateOrUpdateTrack(&Track{
		LibraryID: granted.ID, AlbumID: &album.ID, ArtistID: &artist.ID,
		FilePath: "/g1/album/01.flac", FileName: "01.flac", FileSize: 1,
		Title: "Track One", Codec: "flac", SampleRate: 44100, Channels: 2,
	}); err != nil {
		t.Fatalf("create track: %v", err)
	}

	libs, err := repo.ListLibrariesForUser(userID)
	if err != nil {
		t.Fatalf("list libraries: %v", err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected exactly 1 library visible to user, got %d", len(libs))
	}
	if libs[0].Name != "Granted" {
		t.Fatalf("expected the granted library, got %q", libs[0].Name)
	}
	if len(libs[0].Paths) != 2 {
		t.Fatalf("expected 2 paths, got %v", libs[0].Paths)
	}
	if libs[0].AlbumCount != 1 {
		t.Fatalf("expected album count 1, got %d", libs[0].AlbumCount)
	}
	if libs[0].TrackCount != 1 {
		t.Fatalf("expected track count 1, got %d", libs[0].TrackCount)
	}
}

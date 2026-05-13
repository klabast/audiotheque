package library

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/pressly/goose/v3"

	"audiod/internal/database"
)

func setupTestRepo(t *testing.T) (Repository, *sql.DB, int64) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable fk: %v", err)
	}

	goose.SetBaseFS(database.EmbedMigrations())
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	res, err := db.Exec("INSERT INTO library (name) VALUES ('test')")
	if err != nil {
		t.Fatalf("seed library: %v", err)
	}
	libID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("library id: %v", err)
	}

	return NewRepository(db), db, libID
}

// TestGetOrCreateAlbum_SeparatesVersionsByFolder is the behaviour spec for the
// multi-version album bug: when the same release exists in two folders (e.g.
// standard + 24-bit hi-res rip), each folder must produce its own album row
// so tracks don't get merged into a single duplicated tracklist.
func TestGetOrCreateAlbum_SeparatesVersionsByFolder(t *testing.T) {
	repo, _, libraryID := setupTestRepo(t)

	artist, err := repo.GetOrCreateArtist(libraryID, "Amy Winehouse")
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}

	standard, err := repo.GetOrCreateAlbum(libraryID, &artist.ID, "Back to Black", "2006",
		"/music/Amy Winehouse/Back to Black")
	if err != nil {
		t.Fatalf("create standard album: %v", err)
	}

	hires, err := repo.GetOrCreateAlbum(libraryID, &artist.ID, "Back to Black", "2006",
		"/music/Amy Winehouse/Back to Black (24bit Hi-Res)")
	if err != nil {
		t.Fatalf("create hi-res album: %v", err)
	}

	if standard.ID == hires.ID {
		t.Fatalf("expected separate album rows for different folder paths, got same ID %d", standard.ID)
	}

	// Idempotency: calling again with the same folder path returns the same row.
	again, err := repo.GetOrCreateAlbum(libraryID, &artist.ID, "Back to Black", "2006",
		"/music/Amy Winehouse/Back to Black")
	if err != nil {
		t.Fatalf("re-fetch standard album: %v", err)
	}
	if again.ID != standard.ID {
		t.Fatalf("expected same album row on re-fetch, got %d vs %d", again.ID, standard.ID)
	}
}

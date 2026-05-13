package playback

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/pressly/goose/v3"

	"audiod/internal/database"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	goose.SetBaseFS(database.EmbedMigrations())
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

func TestDBSessionRepository_RoundTrip(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDBSessionRepository(db)

	want := &Session{
		UserID: 42,
		State:  StatePlaying,
		Current: &CurrentTrack{
			TrackID:  7,
			Position: 123,
		},
		Source: Source{
			Type:      SourceTypeAlbum,
			ID:        100,
			Remaining: []int64{8, 9, 10},
		},
		Queue: []QueueItem{
			{TrackID: 11, AddedFromID: 200, AddedFrom: SourceTypePlaylist},
		},
		History:   []int64{1, 2, 3},
		DeviceID:  "mpd-living-room",
		DeviceVolumes: map[string]int{
			"":                70, // browser
			"mpd-living-room": 55,
		},
		IsPrivate: true,
	}

	if err := repo.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.GetByUserID(42)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if got == nil {
		t.Fatal("session not found")
	}

	if got.UserID != want.UserID {
		t.Errorf("UserID = %d, want %d", got.UserID, want.UserID)
	}
	if got.State != want.State {
		t.Errorf("State = %s, want %s", got.State, want.State)
	}
	if got.Current == nil || got.Current.TrackID != 7 || got.Current.Position != 123 {
		t.Errorf("Current = %+v, want {7,123}", got.Current)
	}
	if got.Source.Type != SourceTypeAlbum || got.Source.ID != 100 {
		t.Errorf("Source = %+v", got.Source)
	}
	if len(got.Source.Remaining) != 3 || got.Source.Remaining[0] != 8 {
		t.Errorf("Source.Remaining = %v", got.Source.Remaining)
	}
	if len(got.Queue) != 1 || got.Queue[0].TrackID != 11 {
		t.Errorf("Queue = %+v", got.Queue)
	}
	if len(got.History) != 3 || got.History[2] != 3 {
		t.Errorf("History = %v", got.History)
	}
	if got.DeviceID != "mpd-living-room" {
		t.Errorf("DeviceID = %q", got.DeviceID)
	}
	if got.DeviceVolumes["mpd-living-room"] != 55 {
		t.Errorf("DeviceVolumes[mpd] = %d", got.DeviceVolumes["mpd-living-room"])
	}
	if !got.IsPrivate {
		t.Error("IsPrivate lost")
	}
}

func TestDBSessionRepository_Upsert(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDBSessionRepository(db)

	first := &Session{
		UserID:  42,
		State:   StatePlaying,
		Current: &CurrentTrack{TrackID: 1, Position: 0},
	}
	if err := repo.Save(first); err != nil {
		t.Fatalf("Save first: %v", err)
	}

	second := &Session{
		UserID:  42,
		State:   StatePaused,
		Current: &CurrentTrack{TrackID: 5, Position: 30},
	}
	if err := repo.Save(second); err != nil {
		t.Fatalf("Save second: %v", err)
	}

	got, err := repo.GetByUserID(42)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if got.State != StatePaused {
		t.Errorf("State = %s, want %s", got.State, StatePaused)
	}
	if got.Current.TrackID != 5 {
		t.Errorf("TrackID = %d, want 5", got.Current.TrackID)
	}

	// Should still be exactly one row.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM playback_session`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("row count = %d, want 1", count)
	}
}

func TestDBSessionRepository_GetMissing(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDBSessionRepository(db)

	got, err := repo.GetByUserID(99)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing session, got %+v", got)
	}
}

// The whole point of persistence: state survives a server restart.
// We simulate "restart" by closing the repo's view of the DB and reopening.
func TestDBSessionRepository_SurvivesRestart(t *testing.T) {
	// Use a file-backed DB so we can close and reopen.
	dir := t.TempDir()
	dbPath := dir + "/audiod.db"

	db1, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db1: %v", err)
	}
	goose.SetBaseFS(database.EmbedMigrations())
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.Up(db1, "migrations"); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	repo1 := NewDBSessionRepository(db1)
	if err := repo1.Save(&Session{
		UserID:  7,
		State:   StatePlaying,
		Current: &CurrentTrack{TrackID: 99, Position: 60},
		Source:  Source{Type: SourceTypeAlbum, ID: 100, Remaining: []int64{}},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	db1.Close()

	// "Restart": reopen the same file.
	db2, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db2: %v", err)
	}
	defer db2.Close()
	repo2 := NewDBSessionRepository(db2)

	got, err := repo2.GetByUserID(7)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if got == nil {
		t.Fatal("session lost across restart")
	}
	if got.Current == nil || got.Current.Position != 60 {
		t.Errorf("position lost: %+v", got.Current)
	}
}

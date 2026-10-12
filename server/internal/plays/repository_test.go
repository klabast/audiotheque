package plays

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/pressly/goose/v3"

	"audiod/internal/database"
)

type fixture struct {
	repo    *Repository
	db      *sql.DB
	userID  int64
	trackID int64
}

func setup(t *testing.T) fixture {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
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

	f := fixture{repo: NewRepository(db), db: db}
	f.userID = insert(t, db, `INSERT INTO user (username, password_hash) VALUES ('alice', 'x')`)
	libraryID := insert(t, db, `INSERT INTO library (name) VALUES ('music')`)
	f.trackID = insert(t, db, `INSERT INTO track (library_id, file_path, file_name, file_size, file_modified, title, codec, sample_rate)
		VALUES (?, '/m/a.flac', 'a.flac', 1, CURRENT_TIMESTAMP, 'A', 'flac', 44100)`, libraryID)
	return f
}

func insert(t *testing.T, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	res, err := db.Exec(query, args...)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed id: %v", err)
	}
	return id
}

type storedPlay struct {
	userID, trackID int64
	playedSeconds   int
}

func storedPlays(t *testing.T, db *sql.DB) []storedPlay {
	t.Helper()
	rows, err := db.Query(`SELECT user_id, track_id, played_seconds FROM play ORDER BY id`)
	if err != nil {
		t.Fatalf("query plays: %v", err)
	}
	defer rows.Close()
	var plays []storedPlay
	for rows.Next() {
		var p storedPlay
		if err := rows.Scan(&p.userID, &p.trackID, &p.playedSeconds); err != nil {
			t.Fatalf("scan: %v", err)
		}
		plays = append(plays, p)
	}
	return plays
}

func TestRepository_RecordPlay(t *testing.T) {
	f := setup(t)

	if err := f.repo.RecordPlay(f.userID, f.trackID, 187); err != nil {
		t.Fatalf("RecordPlay: %v", err)
	}

	got := storedPlays(t, f.db)
	want := storedPlay{userID: f.userID, trackID: f.trackID, playedSeconds: 187}
	if len(got) != 1 || got[0] != want {
		t.Errorf("plays = %v, want [%v]", got, want)
	}
}

func TestRepository_PlaysGoWithTheirTrack(t *testing.T) {
	// Given a recorded play
	f := setup(t)
	if err := f.repo.RecordPlay(f.userID, f.trackID, 30); err != nil {
		t.Fatalf("RecordPlay: %v", err)
	}

	// When the track leaves the library
	if _, err := f.db.Exec(`DELETE FROM track WHERE id = ?`, f.trackID); err != nil {
		t.Fatalf("delete track: %v", err)
	}

	// Then its plays are gone too
	if got := storedPlays(t, f.db); len(got) != 0 {
		t.Errorf("plays = %v, want none", got)
	}
}

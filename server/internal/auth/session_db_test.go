package auth

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/pressly/goose/v3"

	"audiod/internal/database"
)

func setupSessionTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	goose.SetBaseFS(database.EmbedMigrations())
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("failed to set goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return db
}

func TestSessionRepository_SetExpiryForUser(t *testing.T) {
	db := setupSessionTestDB(t)
	repo := NewSessionRepository(db)

	now := time.Now().UTC().Truncate(time.Second)
	mine := &Session{ID: "sess-mine", UserID: 1, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(30 * 24 * time.Hour)}
	other := &Session{ID: "sess-other", UserID: 2, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(30 * 24 * time.Hour)}
	if err := repo.Create(mine); err != nil {
		t.Fatalf("create mine: %v", err)
	}
	if err := repo.Create(other); err != nil {
		t.Fatalf("create other: %v", err)
	}

	soon := now.Add(1 * time.Minute)
	if err := repo.SetExpiryForUser(1, soon); err != nil {
		t.Fatalf("SetExpiryForUser: %v", err)
	}

	got, err := repo.GetByID("sess-mine")
	if err != nil {
		t.Fatalf("GetByID mine: %v", err)
	}
	if !got.ExpiresAt.Equal(soon) {
		t.Errorf("expected user 1's session expiry to be bumped to %v, got %v", soon, got.ExpiresAt)
	}

	untouched, err := repo.GetByID("sess-other")
	if err != nil {
		t.Fatalf("GetByID other: %v", err)
	}
	if untouched.ExpiresAt.Equal(soon) {
		t.Errorf("expected user 2's session to be untouched, but it was bumped")
	}
}

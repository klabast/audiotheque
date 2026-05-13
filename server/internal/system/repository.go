package system

import (
	"database/sql"
	"fmt"
)

// Repository defines data access operations for system-level management.
type Repository interface {
	// ResetAll deletes all libraries and users, resetting the system to initial state.
	ResetAll() error
}

type sqliteRepository struct {
	db *sql.DB
}

// NewRepository creates a new system repository.
func NewRepository(db *sql.DB) Repository {
	return &sqliteRepository{db: db}
}

func (r *sqliteRepository) ResetAll() error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Order matters: child rows first, then parents. SQLite will refuse
	// to delete a row that's still referenced via FK, but we keep the
	// explicit order here so future schema additions don't break reset
	// silently. Migration metadata (goose_db_version) is intentionally
	// preserved — the schema itself isn't being reset, only data.
	tables := []string{
		"playback_session",
		"reset_code",
		"scan_queue",
		"track_artist",
		"track",
		"album",
		"artist",
		"library_access",
		"library_path",
		"library",
		"device",
		"setting",
		"user",
	}
	for _, t := range tables {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil {
			return fmt.Errorf("failed to delete %s: %w", t, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit reset: %w", err)
	}

	return nil
}

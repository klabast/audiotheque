package plays

import (
	"database/sql"
	"fmt"
)

// Repository stores the play log.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) RecordPlay(userID, trackID int64, playedSeconds int) error {
	//language=SQL
	const query = `INSERT INTO play (user_id, track_id, played_seconds) VALUES (?, ?, ?)`
	if _, err := r.db.Exec(query, userID, trackID, playedSeconds); err != nil {
		return fmt.Errorf("record play: %w", err)
	}
	return nil
}

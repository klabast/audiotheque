package playback

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// DBSessionRepository persists playback sessions in SQLite. Arrays/maps are
// stored as JSON because they only need to round-trip and are never queried
// by their contents.
type DBSessionRepository struct {
	db *sql.DB
}

// NewDBSessionRepository creates a SQLite-backed session repository.
func NewDBSessionRepository(db *sql.DB) *DBSessionRepository {
	return &DBSessionRepository{db: db}
}

// Save upserts the session. There is exactly one row per user (Spotify model).
func (r *DBSessionRepository) Save(session *Session) error {
	if session == nil {
		return errors.New("nil session")
	}

	remainingJSON, err := json.Marshal(safeInt64Slice(session.Source.Remaining))
	if err != nil {
		return fmt.Errorf("marshal remaining: %w", err)
	}
	queueJSON, err := json.Marshal(safeQueueSlice(session.Queue))
	if err != nil {
		return fmt.Errorf("marshal queue: %w", err)
	}
	historyJSON, err := json.Marshal(safeInt64Slice(session.History))
	if err != nil {
		return fmt.Errorf("marshal history: %w", err)
	}
	volumesJSON, err := json.Marshal(safeVolumesMap(session.DeviceVolumes))
	if err != nil {
		return fmt.Errorf("marshal volumes: %w", err)
	}

	var currentTrackID sql.NullInt64
	var currentPosition int
	if session.Current != nil {
		currentTrackID = sql.NullInt64{Int64: session.Current.TrackID, Valid: true}
		currentPosition = session.Current.Position
	}

	//language=SQL
	const query = `
		INSERT INTO playback_session (
			user_id, state, current_track_id, current_position,
			source_type, source_id, source_remaining,
			queue, history, device_id, device_volumes, is_private,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET
			state = excluded.state,
			current_track_id = excluded.current_track_id,
			current_position = excluded.current_position,
			source_type = excluded.source_type,
			source_id = excluded.source_id,
			source_remaining = excluded.source_remaining,
			queue = excluded.queue,
			history = excluded.history,
			device_id = excluded.device_id,
			device_volumes = excluded.device_volumes,
			is_private = excluded.is_private,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err = r.db.Exec(query,
		session.UserID,
		string(session.State),
		currentTrackID,
		currentPosition,
		string(session.Source.Type),
		session.Source.ID,
		string(remainingJSON),
		string(queueJSON),
		string(historyJSON),
		session.DeviceID,
		string(volumesJSON),
		session.IsPrivate,
	)
	if err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}
	return nil
}

// GetByUserID returns the session for a user, or (nil, nil) if none exists.
func (r *DBSessionRepository) GetByUserID(userID int64) (*Session, error) {
	//language=SQL
	const query = `
		SELECT state, current_track_id, current_position,
			source_type, source_id, source_remaining,
			queue, history, device_id, device_volumes, is_private
		FROM playback_session
		WHERE user_id = ?
	`

	var (
		state           string
		currentTrackID  sql.NullInt64
		currentPosition int
		sourceType      string
		sourceID        int64
		sourceRemaining string
		queueJSON       string
		historyJSON     string
		deviceID        string
		deviceVolumes   string
		isPrivate       bool
	)

	err := r.db.QueryRow(query, userID).Scan(
		&state, &currentTrackID, &currentPosition,
		&sourceType, &sourceID, &sourceRemaining,
		&queueJSON, &historyJSON, &deviceID, &deviceVolumes, &isPrivate,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}

	session := &Session{
		UserID:    userID,
		State:     State(state),
		DeviceID:  deviceID,
		IsPrivate: isPrivate,
		Source: Source{
			Type: SourceType(sourceType),
			ID:   sourceID,
		},
	}

	if currentTrackID.Valid {
		session.Current = &CurrentTrack{
			TrackID:  currentTrackID.Int64,
			Position: currentPosition,
		}
	}

	if err := json.Unmarshal([]byte(sourceRemaining), &session.Source.Remaining); err != nil {
		return nil, fmt.Errorf("unmarshal remaining: %w", err)
	}
	if err := json.Unmarshal([]byte(queueJSON), &session.Queue); err != nil {
		return nil, fmt.Errorf("unmarshal queue: %w", err)
	}
	if err := json.Unmarshal([]byte(historyJSON), &session.History); err != nil {
		return nil, fmt.Errorf("unmarshal history: %w", err)
	}
	if err := json.Unmarshal([]byte(deviceVolumes), &session.DeviceVolumes); err != nil {
		return nil, fmt.Errorf("unmarshal volumes: %w", err)
	}

	// Normalize nil slices/maps to empty so callers don't have to nil-check.
	if session.Source.Remaining == nil {
		session.Source.Remaining = []int64{}
	}
	if session.Queue == nil {
		session.Queue = []QueueItem{}
	}
	if session.History == nil {
		session.History = []int64{}
	}

	return session, nil
}

// Delete removes a user's session row. No-op if the row doesn't exist.
func (r *DBSessionRepository) Delete(userID int64) error {
	//language=SQL
	const query = `DELETE FROM playback_session WHERE user_id = ?`
	if _, err := r.db.Exec(query, userID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteWithoutDevice removes any persisted session whose device_id is empty.
// Such rows pre-date the unified-device invariant — they have no addressable
// owner and there is no safe way to resume playback from them, so the
// startup migration sweeps them out. Returns the number of rows removed.
func (r *DBSessionRepository) DeleteWithoutDevice() (int64, error) {
	//language=SQL
	const query = `DELETE FROM playback_session WHERE device_id = ''`
	res, err := r.db.Exec(query)
	if err != nil {
		return 0, fmt.Errorf("delete sessions without device: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}

// JSON marshals nil slices as "null"; we want "[]" so the column round-trips
// through Go without surprise. Same for maps.
func safeInt64Slice(s []int64) []int64 {
	if s == nil {
		return []int64{}
	}
	return s
}

func safeQueueSlice(s []QueueItem) []QueueItem {
	if s == nil {
		return []QueueItem{}
	}
	return s
}

func safeVolumesMap(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{}
	}
	return m
}

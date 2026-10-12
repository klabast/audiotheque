package plays

import (
	"database/sql"
	"fmt"
	"slices"
	"time"
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

// playsSince is every play by the user since a point in time, limited to
// tracks in libraries the user can still read. Parameters: user id, since.
//
// language=SQL
const playsSince = `
	SELECT p.played_seconds, p.ended_at, t.id AS track_id, t.album_id, t.artist_id, t.duration
	FROM play p
	JOIN track t ON t.id = p.track_id
	JOIN library_access la ON la.library_id = t.library_id AND la.user_id = p.user_id
	WHERE p.user_id = ? AND p.ended_at >= ?`

// listensSince narrows playsSince to listens. Extra parameter: the listen
// threshold in seconds.
//
// language=SQL
const listensSince = `
	SELECT played_seconds, ended_at, track_id, album_id, artist_id, duration FROM (` + playsSince + `)
	WHERE played_seconds * 1000 >= MIN(duration / 2, ? * 1000)`

// Stats counts the user's listens that ended at or after since, in libraries
// they can still read, with the top `limit` tracks, albums and artists.
func (r *Repository) Stats(userID int64, since time.Time, limit int) (*Stats, error) {
	params := []any{userID, since.UTC().Format(time.DateTime)}
	listenParams := append(slices.Clone(params), ListenThresholdSeconds)
	stats := &Stats{}

	//language=SQL
	totals := `SELECT
		(SELECT COUNT(*) FROM (` + listensSince + `)),
		(SELECT COALESCE(SUM(played_seconds), 0) FROM (` + playsSince + `))`
	if err := r.db.QueryRow(totals, append(slices.Clone(listenParams), params...)...).
		Scan(&stats.Listens, &stats.ListeningSeconds); err != nil {
		return nil, fmt.Errorf("play totals: %w", err)
	}

	//language=SQL
	topTracks := `
		SELECT l.track_id, t.title, COALESCE(ar.name, ''), COALESCE(t.album_id, 0), COALESCE(al.title, ''), COUNT(*) AS plays
		FROM (` + listensSince + `) l
		JOIN track t ON t.id = l.track_id
		LEFT JOIN artist ar ON ar.id = t.artist_id
		LEFT JOIN album al ON al.id = t.album_id
		GROUP BY l.track_id
		ORDER BY plays DESC, MAX(l.ended_at) DESC
		LIMIT ?`
	err := r.queryEach(topTracks, append(slices.Clone(listenParams), limit), func(rows *sql.Rows) error {
		var tc TrackCount
		if err := rows.Scan(&tc.TrackID, &tc.Title, &tc.ArtistName, &tc.AlbumID, &tc.AlbumTitle, &tc.Plays); err != nil {
			return err
		}
		stats.TopTracks = append(stats.TopTracks, tc)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("top tracks: %w", err)
	}

	//language=SQL
	topAlbums := `
		SELECT al.id, al.title, COALESCE(ar.name, ''), COUNT(*) AS plays
		FROM (` + listensSince + `) l
		JOIN album al ON al.id = l.album_id
		LEFT JOIN artist ar ON ar.id = al.artist_id
		GROUP BY al.id
		ORDER BY plays DESC, al.title
		LIMIT ?`
	err = r.queryEach(topAlbums, append(slices.Clone(listenParams), limit), func(rows *sql.Rows) error {
		var ac AlbumCount
		if err := rows.Scan(&ac.AlbumID, &ac.Title, &ac.ArtistName, &ac.Plays); err != nil {
			return err
		}
		stats.TopAlbums = append(stats.TopAlbums, ac)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("top albums: %w", err)
	}

	//language=SQL
	topArtists := `
		SELECT ar.id, ar.name, COUNT(*) AS plays
		FROM (` + listensSince + `) l
		JOIN artist ar ON ar.id = l.artist_id
		GROUP BY ar.id
		ORDER BY plays DESC, ar.name
		LIMIT ?`
	err = r.queryEach(topArtists, append(slices.Clone(listenParams), limit), func(rows *sql.Rows) error {
		var ac ArtistCount
		if err := rows.Scan(&ac.ArtistID, &ac.Name, &ac.Plays); err != nil {
			return err
		}
		stats.TopArtists = append(stats.TopArtists, ac)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("top artists: %w", err)
	}

	return stats, nil
}

func (r *Repository) queryEach(query string, args []any, scan func(*sql.Rows) error) error {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

package library

import (
	"database/sql"
	"time"
)

// =============================================================================
// Track methods
// =============================================================================

// CreateOrUpdateTrack creates a new track or updates an existing one
func (r *repository) CreateOrUpdateTrack(track *Track) (*Track, error) {
	// Check if track exists
	existing, err := r.GetTrackByPath(track.LibraryID, track.FilePath)
	if err != nil && err != ErrTrackNotFound {
		return nil, err
	}

	if existing != nil {
		// Update existing track
		//language=SQL
		query := `
UPDATE track SET
    album_id = ?, artist_id = ?, file_name = ?, file_size = ?, file_modified = ?,
    title = ?, sort_title = ?, track_number = ?, disc_number = ?, duration = ?,
    year = ?, genre = ?, musicbrainz_id = ?, codec = ?, bitrate = ?,
    sample_rate = ?, bit_depth = ?, channels = ?, is_lossless = ?, is_hires = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?`

		_, err := r.db.Exec(query,
			track.AlbumID, track.ArtistID, track.FileName, track.FileSize, track.FileModified,
			track.Title, track.SortTitle, track.TrackNumber, track.DiscNumber, track.Duration,
			track.Year, track.Genre, track.MusicBrainzID, track.Codec, track.Bitrate,
			track.SampleRate, track.BitDepth, track.Channels, track.IsLossless, track.IsHiRes,
			existing.ID,
		)
		if err != nil {
			return nil, err
		}

		track.ID = existing.ID
		return track, nil
	}

	// Create new track
	//language=SQL
	query := `
INSERT INTO track (
    library_id, album_id, artist_id, file_path, file_name, file_size, file_modified,
    title, sort_title, track_number, disc_number, duration, year, genre, musicbrainz_id,
    codec, bitrate, sample_rate, bit_depth, channels, is_lossless, is_hires
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := r.db.Exec(query,
		track.LibraryID, track.AlbumID, track.ArtistID, track.FilePath, track.FileName,
		track.FileSize, track.FileModified, track.Title, track.SortTitle, track.TrackNumber,
		track.DiscNumber, track.Duration, track.Year, track.Genre, track.MusicBrainzID,
		track.Codec, track.Bitrate, track.SampleRate, track.BitDepth, track.Channels,
		track.IsLossless, track.IsHiRes,
	)
	if err != nil {
		return nil, err
	}

	trackID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	track.ID = trackID
	return track, nil
}

// GetTrackByID retrieves a track by its ID
func (r *repository) GetTrackByID(trackID int64) (*Track, error) {
	//language=SQL
	query := `
SELECT id, library_id, album_id, artist_id, file_path, file_name, file_size, file_modified,
       title, COALESCE(sort_title, ''), track_number, disc_number, duration, year, COALESCE(genre, ''), COALESCE(musicbrainz_id, ''),
       COALESCE(codec, ''), bitrate, sample_rate, bit_depth, channels, is_lossless, is_hires,
       created_at, updated_at
FROM track
WHERE id = ?`

	track := &Track{}
	var albumID, artistID, bitDepth sql.NullInt64
	err := r.db.QueryRow(query, trackID).Scan(
		&track.ID, &track.LibraryID, &albumID, &artistID, &track.FilePath, &track.FileName,
		&track.FileSize, &track.FileModified, &track.Title, &track.SortTitle, &track.TrackNumber,
		&track.DiscNumber, &track.Duration, &track.Year, &track.Genre, &track.MusicBrainzID,
		&track.Codec, &track.Bitrate, &track.SampleRate, &bitDepth, &track.Channels,
		&track.IsLossless, &track.IsHiRes, &track.CreatedAt, &track.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrTrackNotFound
	}
	if err != nil {
		return nil, err
	}

	if albumID.Valid {
		track.AlbumID = &albumID.Int64
	}
	if artistID.Valid {
		track.ArtistID = &artistID.Int64
	}
	if bitDepth.Valid {
		track.BitDepth = int(bitDepth.Int64)
	}

	return track, nil
}

// GetTrackByPath retrieves a track by its file path
func (r *repository) GetTrackByPath(libraryID int64, filePath string) (*Track, error) {
	//language=SQL
	query := `
SELECT id, library_id, album_id, artist_id, file_path, file_name, file_size, file_modified,
       title, COALESCE(sort_title, ''), track_number, disc_number, duration, year, COALESCE(genre, ''), COALESCE(musicbrainz_id, ''),
       COALESCE(codec, ''), bitrate, sample_rate, bit_depth, channels, is_lossless, is_hires,
       created_at, updated_at
FROM track
WHERE library_id = ? AND file_path = ?`

	track := &Track{}
	var albumID, artistID, bitDepth sql.NullInt64
	err := r.db.QueryRow(query, libraryID, filePath).Scan(
		&track.ID, &track.LibraryID, &albumID, &artistID, &track.FilePath, &track.FileName,
		&track.FileSize, &track.FileModified, &track.Title, &track.SortTitle, &track.TrackNumber,
		&track.DiscNumber, &track.Duration, &track.Year, &track.Genre, &track.MusicBrainzID,
		&track.Codec, &track.Bitrate, &track.SampleRate, &bitDepth, &track.Channels,
		&track.IsLossless, &track.IsHiRes, &track.CreatedAt, &track.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrTrackNotFound
	}
	if err != nil {
		return nil, err
	}

	if albumID.Valid {
		track.AlbumID = &albumID.Int64
	}
	if artistID.Valid {
		track.ArtistID = &artistID.Int64
	}
	if bitDepth.Valid {
		track.BitDepth = int(bitDepth.Int64)
	}

	return track, nil
}

// ListTracksByAlbum retrieves all tracks in an album
func (r *repository) ListTracksByAlbum(albumID int64) ([]*Track, error) {
	//language=SQL
	query := `
SELECT id, library_id, album_id, artist_id, file_path, file_name, file_size, file_modified,
       title, COALESCE(sort_title, ''), track_number, disc_number, duration, year, COALESCE(genre, ''), COALESCE(musicbrainz_id, ''),
       COALESCE(codec, ''), bitrate, sample_rate, bit_depth, channels, is_lossless, is_hires,
       created_at, updated_at
FROM track
WHERE album_id = ?
ORDER BY disc_number, track_number`

	return r.queryTracks(query, albumID)
}

// ListTracksByLibrary retrieves all tracks in a library
func (r *repository) ListTracksByLibrary(libraryID int64) ([]*Track, error) {
	//language=SQL
	query := `
SELECT id, library_id, album_id, artist_id, file_path, file_name, file_size, file_modified,
       title, COALESCE(sort_title, ''), track_number, disc_number, duration, year, COALESCE(genre, ''), COALESCE(musicbrainz_id, ''),
       COALESCE(codec, ''), bitrate, sample_rate, bit_depth, channels, is_lossless, is_hires,
       created_at, updated_at
FROM track
WHERE library_id = ?
ORDER BY title`

	return r.queryTracks(query, libraryID)
}

// queryTracks is a helper to scan multiple tracks
func (r *repository) queryTracks(query string, args ...interface{}) ([]*Track, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []*Track
	for rows.Next() {
		track := &Track{}
		var albumID, artistID, bitDepth sql.NullInt64
		err := rows.Scan(
			&track.ID, &track.LibraryID, &albumID, &artistID, &track.FilePath, &track.FileName,
			&track.FileSize, &track.FileModified, &track.Title, &track.SortTitle, &track.TrackNumber,
			&track.DiscNumber, &track.Duration, &track.Year, &track.Genre, &track.MusicBrainzID,
			&track.Codec, &track.Bitrate, &track.SampleRate, &bitDepth, &track.Channels,
			&track.IsLossless, &track.IsHiRes, &track.CreatedAt, &track.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if albumID.Valid {
			track.AlbumID = &albumID.Int64
		}
		if artistID.Valid {
			track.ArtistID = &artistID.Int64
		}
		if bitDepth.Valid {
			track.BitDepth = int(bitDepth.Int64)
		}
		tracks = append(tracks, track)
	}

	return tracks, rows.Err()
}

// GetTrackPathsForLibrary returns a map of file paths to modification times for incremental scanning
func (r *repository) GetTrackPathsForLibrary(libraryID int64) (map[string]time.Time, error) {
	//language=SQL
	query := `SELECT file_path, file_modified FROM track WHERE library_id = ?`

	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]time.Time)
	for rows.Next() {
		var path string
		var modTime time.Time
		if err := rows.Scan(&path, &modTime); err != nil {
			return nil, err
		}
		result[path] = modTime
	}

	return result, rows.Err()
}

// deleteTracksBatch is how many paths go into one DELETE. SQLite's default
// bound-parameter limit is 999, so a library that lost thousands of files still
// has to be removed in chunks.
const deleteTracksBatch = 500

// DeleteTracksByPaths removes tracks by absolute file path. The FTS index is
// maintained by the track_ad trigger, so no separate cleanup is needed.
func (r *repository) DeleteTracksByPaths(libraryID int64, paths []string) (int64, error) {
	var deleted int64
	for start := 0; start < len(paths); start += deleteTracksBatch {
		end := min(start+deleteTracksBatch, len(paths))
		chunk := paths[start:end]

		args := make([]interface{}, 0, len(chunk)+1)
		args = append(args, libraryID)
		for _, p := range chunk {
			args = append(args, p)
		}

		//language=SQL
		query := `DELETE FROM track WHERE library_id = ? AND file_path IN (` + placeholders(len(chunk)) + `)`
		result, err := r.db.Exec(query, args...)
		if err != nil {
			return deleted, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return deleted, err
		}
		deleted += n
	}
	return deleted, nil
}

// =============================================================================
// Track-Artist relationship methods
// =============================================================================

// AddTrackArtist adds an artist to a track with a specific role
func (r *repository) AddTrackArtist(trackID, artistID int64, role string, position int) error {
	//language=SQL
	query := `
INSERT INTO track_artist (track_id, artist_id, role, position)
VALUES (?, ?, ?, ?)
ON CONFLICT (track_id, artist_id, role) DO UPDATE SET position = excluded.position`

	_, err := r.db.Exec(query, trackID, artistID, role, position)
	return err
}

// GetTrackArtists retrieves all artists for a track
func (r *repository) GetTrackArtists(trackID int64) ([]*TrackArtist, error) {
	//language=SQL
	query := `
SELECT track_id, artist_id, role, position
FROM track_artist
WHERE track_id = ?
ORDER BY position`

	rows, err := r.db.Query(query, trackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trackArtists []*TrackArtist
	for rows.Next() {
		ta := &TrackArtist{}
		if err := rows.Scan(&ta.TrackID, &ta.ArtistID, &ta.Role, &ta.Position); err != nil {
			return nil, err
		}
		trackArtists = append(trackArtists, ta)
	}

	return trackArtists, rows.Err()
}

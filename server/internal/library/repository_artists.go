package library

import "database/sql"

// =============================================================================
// Artist methods
// =============================================================================

// GetOrCreateArtist retrieves an artist by name or creates a new one
func (r *repository) GetOrCreateArtist(libraryID int64, name string) (*Artist, error) {
	// Try to find existing artist
	// Use COALESCE to handle NULL values for optional string fields
	//language=SQL
	selectQuery := `
SELECT id, library_id, name, COALESCE(sort_name, ''), COALESCE(musicbrainz_id, ''), created_at, updated_at
FROM artist
WHERE library_id = ? AND name = ?`

	artist := &Artist{}
	err := r.db.QueryRow(selectQuery, libraryID, name).Scan(
		&artist.ID, &artist.LibraryID, &artist.Name, &artist.SortName,
		&artist.MusicBrainzID, &artist.CreatedAt, &artist.UpdatedAt,
	)
	if err == nil {
		return artist, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create new artist
	//language=SQL
	insertQuery := `INSERT INTO artist (library_id, name, sort_name) VALUES (?, ?, ?)`
	result, err := r.db.Exec(insertQuery, libraryID, name, computeSortName(name))
	if err != nil {
		return nil, err
	}

	artistID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	// Fetch the created artist to get timestamps
	return r.GetArtistByID(artistID)
}

// GetArtistByID retrieves an artist by ID
func (r *repository) GetArtistByID(id int64) (*Artist, error) {
	// Use COALESCE to handle NULL values for optional string fields
	//language=SQL
	query := `
SELECT id, library_id, name, COALESCE(sort_name, ''), COALESCE(musicbrainz_id, ''), created_at, updated_at
FROM artist
WHERE id = ?`

	artist := &Artist{}
	err := r.db.QueryRow(query, id).Scan(
		&artist.ID, &artist.LibraryID, &artist.Name, &artist.SortName,
		&artist.MusicBrainzID, &artist.CreatedAt, &artist.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrArtistNotFound
	}
	return artist, err
}

// UpdateArtistMusicBrainzID updates an artist's MusicBrainz ID
func (r *repository) UpdateArtistMusicBrainzID(id int64, mbid string) error {
	//language=SQL
	query := `UPDATE artist SET musicbrainz_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	result, err := r.db.Exec(query, mbid, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrArtistNotFound
	}
	return nil
}

// BackfillArtistSortNames re-applies computeSortName to every artist whose
// sort_name is NULL or empty. Reads in one query, applies in Go (so we use the
// same logic as inserts), writes back as needed.
func (r *repository) BackfillArtistSortNames() (int64, error) {
	//language=SQL
	rows, err := r.db.Query(`SELECT id, name FROM artist WHERE sort_name IS NULL OR sort_name = ''`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type pending struct {
		id   int64
		sort string
	}
	var updates []pending
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return 0, err
		}
		updates = append(updates, pending{id: id, sort: computeSortName(name)})
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	//language=SQL
	const updateQuery = `UPDATE artist SET sort_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	var n int64
	for _, u := range updates {
		res, err := r.db.Exec(updateQuery, u.sort, u.id)
		if err != nil {
			return n, err
		}
		if affected, _ := res.RowsAffected(); affected > 0 {
			n += affected
		}
	}
	return n, nil
}

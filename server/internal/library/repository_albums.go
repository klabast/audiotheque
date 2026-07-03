package library

import (
	"database/sql"
	"strings"
)

// =============================================================================
// Album methods
// =============================================================================

// GetOrCreateAlbum retrieves an album by (library, artist, title, folder_path)
// or creates a new one. Including folder_path in the dedup key is what keeps
// different physical releases (e.g. standard + 24-bit hi-res) as separate
// album rows even when they share title and artist.
func (r *repository) GetOrCreateAlbum(libraryID int64, artistID *int64, title string, releaseDate string, folderPath string) (*Album, error) {
	// Try to find existing album
	//language=SQL
	var selectQuery string
	var args []interface{}

	if artistID != nil {
		selectQuery = `
SELECT id, library_id, artist_id, title, COALESCE(sort_title, ''), COALESCE(musicbrainz_id, ''), COALESCE(release_date, ''),
       COALESCE(genre, ''), COALESCE(total_tracks, 0), COALESCE(total_discs, 0), COALESCE(cover_art_path, ''), COALESCE(is_compilation, 0),
       COALESCE(folder_path, ''), COALESCE(release_type, 'original'), created_at, updated_at
FROM album
WHERE library_id = ? AND artist_id = ? AND title = ? AND folder_path = ?`
		args = []interface{}{libraryID, *artistID, title, folderPath}
	} else {
		selectQuery = `
SELECT id, library_id, artist_id, title, COALESCE(sort_title, ''), COALESCE(musicbrainz_id, ''), COALESCE(release_date, ''),
       COALESCE(genre, ''), COALESCE(total_tracks, 0), COALESCE(total_discs, 0), COALESCE(cover_art_path, ''), COALESCE(is_compilation, 0),
       COALESCE(folder_path, ''), COALESCE(release_type, 'original'), created_at, updated_at
FROM album
WHERE library_id = ? AND artist_id IS NULL AND title = ? AND folder_path = ?`
		args = []interface{}{libraryID, title, folderPath}
	}

	album := &Album{}
	var artistIDScan sql.NullInt64
	err := r.db.QueryRow(selectQuery, args...).Scan(
		&album.ID, &album.LibraryID, &artistIDScan, &album.Title, &album.SortTitle,
		&album.MusicBrainzID, &album.ReleaseDate, &album.Genre, &album.TotalTracks,
		&album.TotalDiscs, &album.CoverArtPath, &album.IsCompilation,
		&album.FolderPath, &album.ReleaseType,
		&album.CreatedAt, &album.UpdatedAt,
	)
	if err == nil {
		if artistIDScan.Valid {
			album.ArtistID = &artistIDScan.Int64
		}
		return album, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create new album
	releaseType := detectReleaseType(folderPath)
	//language=SQL
	insertQuery := `INSERT INTO album (library_id, artist_id, title, release_date, folder_path, release_type) VALUES (?, ?, ?, ?, ?, ?)`
	result, err := r.db.Exec(insertQuery, libraryID, artistID, title, releaseDate, folderPath, releaseType)
	if err != nil {
		return nil, err
	}

	albumID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	// Fetch the created album
	return r.GetAlbumByID(albumID)
}

// GetAlbumByID retrieves an album by ID
func (r *repository) GetAlbumByID(id int64) (*Album, error) {
	//language=SQL
	query := `
SELECT a.id, a.library_id, a.artist_id, a.title, COALESCE(a.sort_title, ''), COALESCE(a.musicbrainz_id, ''),
       COALESCE(a.release_date, ''), COALESCE(a.genre, ''), COALESCE(a.total_tracks, 0), COALESCE(a.total_discs, 0),
       COALESCE(a.cover_art_path, ''), COALESCE(a.is_compilation, 0),
       EXISTS(SELECT 1 FROM track t WHERE t.album_id = a.id AND t.is_hires = 1) AS is_hires,
       COALESCE(a.folder_path, ''), COALESCE(a.release_type, 'original'),
       a.created_at, a.updated_at
FROM album a
WHERE a.id = ?`

	album := &Album{}
	var artistID sql.NullInt64
	err := r.db.QueryRow(query, id).Scan(
		&album.ID, &album.LibraryID, &artistID, &album.Title, &album.SortTitle,
		&album.MusicBrainzID, &album.ReleaseDate, &album.Genre, &album.TotalTracks,
		&album.TotalDiscs, &album.CoverArtPath, &album.IsCompilation, &album.IsHiRes,
		&album.FolderPath, &album.ReleaseType,
		&album.CreatedAt, &album.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrAlbumNotFound
	}
	if err != nil {
		return nil, err
	}
	if artistID.Valid {
		album.ArtistID = &artistID.Int64
	}
	return album, nil
}

// UpdateAlbumCoverArt updates an album's cover art path
func (r *repository) UpdateAlbumCoverArt(id int64, coverPath string) error {
	//language=SQL
	query := `UPDATE album SET cover_art_path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	result, err := r.db.Exec(query, coverPath, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrAlbumNotFound
	}
	return nil
}

// ListAlbumsByLibrary retrieves all albums in a library.
// IsHiRes is computed via a track-existence subquery — an album is hi-res when
// any of its tracks is hi-res. When opts.HiResOnly is set, albums without any
// hi-res track are filtered out.
func (r *repository) ListAlbumsByLibrary(libraryID int64, opts ListAlbumsOptions) ([]*Album, error) {
	//language=SQL
	query := `
SELECT a.id, a.library_id, a.artist_id, a.title, COALESCE(a.sort_title, ''), COALESCE(a.musicbrainz_id, ''),
       COALESCE(a.release_date, ''), COALESCE(a.genre, ''), COALESCE(a.total_tracks, 0), COALESCE(a.total_discs, 0),
       COALESCE(a.cover_art_path, ''), COALESCE(a.is_compilation, 0),
       EXISTS(SELECT 1 FROM track t WHERE t.album_id = a.id AND t.is_hires = 1) AS is_hires,
       COALESCE(a.folder_path, ''), COALESCE(a.release_type, 'original'),
       a.created_at, a.updated_at
FROM album a
LEFT JOIN artist ar ON ar.id = a.artist_id
WHERE a.library_id = ?`
	if opts.HiResOnly {
		query += `
  AND EXISTS(SELECT 1 FROM track t WHERE t.album_id = a.id AND t.is_hires = 1)`
	}
	query += "\nORDER BY " + buildAlbumOrderBy(opts.SortBy)

	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []*Album
	for rows.Next() {
		album := &Album{}
		var artistID sql.NullInt64
		err := rows.Scan(
			&album.ID, &album.LibraryID, &artistID, &album.Title, &album.SortTitle,
			&album.MusicBrainzID, &album.ReleaseDate, &album.Genre, &album.TotalTracks,
			&album.TotalDiscs, &album.CoverArtPath, &album.IsCompilation, &album.IsHiRes,
			&album.FolderPath, &album.ReleaseType,
			&album.CreatedAt, &album.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if artistID.Valid {
			album.ArtistID = &artistID.Int64
		}
		albums = append(albums, album)
	}

	return albums, rows.Err()
}

// albumSortColumn maps a SortField to its SQL expression. Unknown fields
// fall back to album-artist so the query never injects unvetted input.
func albumSortColumn(f SortField) string {
	switch f {
	case SortFieldAlbumArtist, SortFieldArtist:
		// Album-artist and artist both resolve to the album's credited artist
		// in the current schema (track-artist majority isn't pre-computed).
		// Compilations have no artist_id; use COALESCE so they sort consistently.
		return "COALESCE(ar.sort_name, ar.name, '')"
	case SortFieldAlbumTitle:
		return "COALESCE(NULLIF(a.sort_title, ''), a.title)"
	case SortFieldYear:
		// album.release_date is often empty; fall back to MIN(track.year) so
		// users get the album's earliest track year. Both sources zero-pad
		// 4-digit years, so lexical ordering matches numeric ordering.
		return `COALESCE(NULLIF(substr(COALESCE(a.release_date, ''), 1, 4), ''),
		         CAST((SELECT MIN(t.year) FROM track t WHERE t.album_id = a.id AND t.year > 0) AS TEXT),
		         '')`
	default:
		return "COALESCE(ar.sort_name, ar.name, '')"
	}
}

func sortDirectionSQL(d SortDirection) string {
	if d == SortDesc {
		return "DESC"
	}
	return "ASC"
}

func buildAlbumOrderBy(specs []SortSpec) string {
	if len(specs) == 0 {
		// Service applies a default; this is a safety net for direct repo callers.
		specs = []SortSpec{{Field: SortFieldAlbumArtist, Direction: SortAsc}}
	}
	parts := make([]string, 0, len(specs)+1)
	for _, s := range specs {
		parts = append(parts, albumSortColumn(s.Field)+" "+sortDirectionSQL(s.Direction))
	}
	// Stable tiebreaker by id so pagination/scroll-restoration is deterministic.
	parts = append(parts, "a.id ASC")
	return strings.Join(parts, ", ")
}

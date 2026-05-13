package library

import (
	"database/sql"
	"strings"
	"time"
)

type repository struct {
	db *sql.DB
}

// NewRepository creates a new library repository
func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

// ListLibrariesForUser retrieves all libraries accessible to a user with their paths and counts
func (r *repository) ListLibrariesForUser(userID int64) ([]*Library, error) {
	// Get libraries with track and album counts
	//language=SQL
	librariesQuery := `
SELECT l.id,
       l.name,
       COALESCE((SELECT COUNT(*) FROM track t WHERE t.library_id = l.id), 0) AS track_count,
       COALESCE((SELECT COUNT(*) FROM album a WHERE a.library_id = l.id), 0) AS album_count
FROM library l
         INNER JOIN library_access la ON l.id = la.library_id
WHERE la.user_id = ?
ORDER BY l.name`

	rows, err := r.db.Query(librariesQuery, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var libraries []*Library
	libraryMap := make(map[int64]*Library)

	for rows.Next() {
		lib := &Library{Paths: []string{}}
		if err := rows.Scan(&lib.ID, &lib.Name, &lib.TrackCount, &lib.AlbumCount); err != nil {
			return nil, err
		}
		libraries = append(libraries, lib)
		libraryMap[lib.ID] = lib
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Get paths for all libraries
	if len(libraries) > 0 {
		//language=SQL
		pathsQuery := `
SELECT library_id, path
FROM library_path
WHERE library_id IN (` + placeholders(len(libraries)) + `)
ORDER BY library_id, path`

		libraryIDs := make([]interface{}, len(libraries))
		for i, lib := range libraries {
			libraryIDs[i] = lib.ID
		}

		pathRows, err := r.db.Query(pathsQuery, libraryIDs...)
		if err != nil {
			return nil, err
		}
		defer pathRows.Close()

		for pathRows.Next() {
			var libraryID int64
			var path string
			if err := pathRows.Scan(&libraryID, &path); err != nil {
				return nil, err
			}
			if lib, ok := libraryMap[libraryID]; ok {
				lib.Paths = append(lib.Paths, path)
			}
		}

		if err := pathRows.Err(); err != nil {
			return nil, err
		}
	}

	return libraries, nil
}

// placeholders generates SQL placeholders like "?, ?, ?"
func placeholders(n int) string {
	if n == 0 {
		return ""
	}
	result := "?"
	for i := 1; i < n; i++ {
		result += ", ?"
	}
	return result
}

// CreateLibrary creates a new library with multiple paths
func (r *repository) CreateLibrary(name string, paths []string) (*Library, error) {
	// Start transaction
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Insert library
	//language=SQL
	libraryQuery := `INSERT INTO library (name) VALUES (?)`
	result, err := tx.Exec(libraryQuery, name)
	if err != nil {
		return nil, err
	}

	libraryID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	// Insert paths
	//language=SQL
	pathQuery := `INSERT INTO library_path (library_id, path) VALUES (?, ?)`
	for _, path := range paths {
		_, err := tx.Exec(pathQuery, libraryID, path)
		if err != nil {
			return nil, err
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &Library{
		ID:    libraryID,
		Name:  name,
		Paths: paths,
	}, nil
}

// GrantAccess grants a user access to a library
func (r *repository) GrantAccess(userID, libraryID int64) error {
	//language=SQL
	query := `
INSERT INTO library_access (user_id, library_id)
VALUES (?, ?)`

	_, err := r.db.Exec(query, userID, libraryID)
	return err
}

// RevokeAccess removes a user's access to a library
func (r *repository) RevokeAccess(userID, libraryID int64) error {
	//language=SQL
	query := `DELETE FROM library_access WHERE user_id = ? AND library_id = ?`

	_, err := r.db.Exec(query, userID, libraryID)
	return err
}

// GetLibraryByID retrieves a library by its ID with all paths
func (r *repository) GetLibraryByID(libraryID int64) (*Library, error) {
	//language=SQL
	libraryQuery := `
SELECT id, name
FROM library
WHERE id = ?`

	library := &Library{Paths: []string{}}
	err := r.db.QueryRow(libraryQuery, libraryID).Scan(&library.ID, &library.Name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrLibraryNotFound
		}
		return nil, err
	}

	// Get paths for the library
	//language=SQL
	pathsQuery := `
SELECT path
FROM library_path
WHERE library_id = ?
ORDER BY path`

	rows, err := r.db.Query(pathsQuery, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		library.Paths = append(library.Paths, path)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return library, nil
}

// DeleteLibrary deletes a library and all associated data
func (r *repository) DeleteLibrary(libraryID int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete library access records
	//language=SQL
	_, err = tx.Exec(`DELETE FROM library_access WHERE library_id = ?`, libraryID)
	if err != nil {
		return err
	}

	// Delete library paths
	//language=SQL
	_, err = tx.Exec(`DELETE FROM library_path WHERE library_id = ?`, libraryID)
	if err != nil {
		return err
	}

	// Delete the library itself
	//language=SQL
	result, err := tx.Exec(`DELETE FROM library WHERE id = ?`, libraryID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrLibraryNotFound
	}

	return tx.Commit()
}

// UpdateLibrary updates a library's name and paths
func (r *repository) UpdateLibrary(libraryID int64, name string, paths []string) (*Library, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Update library name
	//language=SQL
	result, err := tx.Exec(`UPDATE library SET name = ? WHERE id = ?`, name, libraryID)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, ErrLibraryNotFound
	}

	// Delete existing paths
	//language=SQL
	_, err = tx.Exec(`DELETE FROM library_path WHERE library_id = ?`, libraryID)
	if err != nil {
		return nil, err
	}

	// Insert new paths
	//language=SQL
	pathQuery := `INSERT INTO library_path (library_id, path) VALUES (?, ?)`
	for _, path := range paths {
		_, err := tx.Exec(pathQuery, libraryID, path)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &Library{
		ID:    libraryID,
		Name:  name,
		Paths: paths,
	}, nil
}

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

// SearchAlbumsByLibrary returns albums with a title matching the LIKE pattern.
// LIKE in SQLite is case-insensitive for ASCII by default; the wrapper escapes
// %, _, and \ so user input can't expand the pattern.
func (r *repository) SearchAlbumsByLibrary(libraryID int64, query string, limit int) ([]*Album, error) {
	pattern := "%" + escapeLikePattern(query) + "%"

	//language=SQL
	q := `
SELECT a.id, a.library_id, a.artist_id, a.title, COALESCE(a.sort_title, ''), COALESCE(a.musicbrainz_id, ''),
       COALESCE(a.release_date, ''), COALESCE(a.genre, ''), COALESCE(a.total_tracks, 0), COALESCE(a.total_discs, 0),
       COALESCE(a.cover_art_path, ''), COALESCE(a.is_compilation, 0),
       EXISTS(SELECT 1 FROM track t WHERE t.album_id = a.id AND t.is_hires = 1) AS is_hires,
       COALESCE(a.folder_path, ''), COALESCE(a.release_type, 'original'),
       a.created_at, a.updated_at
FROM album a
WHERE a.library_id = ? AND a.title LIKE ? ESCAPE '\'
ORDER BY a.title
LIMIT ?`

	rows, err := r.db.Query(q, libraryID, pattern, limit)
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

// SearchArtistsByLibrary returns artists with a name matching the LIKE pattern.
func (r *repository) SearchArtistsByLibrary(libraryID int64, query string, limit int) ([]*Artist, error) {
	pattern := "%" + escapeLikePattern(query) + "%"

	//language=SQL
	q := `
SELECT id, library_id, name, COALESCE(sort_name, ''), COALESCE(musicbrainz_id, ''), created_at, updated_at
FROM artist
WHERE library_id = ? AND name LIKE ? ESCAPE '\'
ORDER BY name
LIMIT ?`

	rows, err := r.db.Query(q, libraryID, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artists []*Artist
	for rows.Next() {
		artist := &Artist{}
		err := rows.Scan(
			&artist.ID, &artist.LibraryID, &artist.Name, &artist.SortName,
			&artist.MusicBrainzID, &artist.CreatedAt, &artist.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		artists = append(artists, artist)
	}
	return artists, rows.Err()
}

// SearchTracksByLibrary returns tracks with a title matching the LIKE pattern.
func (r *repository) SearchTracksByLibrary(libraryID int64, query string, limit int) ([]*Track, error) {
	pattern := "%" + escapeLikePattern(query) + "%"

	//language=SQL
	q := `
SELECT id, library_id, album_id, artist_id, file_path, file_name, file_size, file_modified,
       title, COALESCE(sort_title, ''), track_number, disc_number, duration, year, COALESCE(genre, ''), COALESCE(musicbrainz_id, ''),
       COALESCE(codec, ''), bitrate, sample_rate, bit_depth, channels, is_lossless, is_hires,
       created_at, updated_at
FROM track
WHERE library_id = ? AND title LIKE ? ESCAPE '\'
ORDER BY title
LIMIT ?`

	rows, err := r.db.Query(q, libraryID, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []*Track
	for rows.Next() {
		track := &Track{}
		var albumID, artistID sql.NullInt64
		var bitDepth sql.NullInt64
		err := rows.Scan(
			&track.ID, &track.LibraryID, &albumID, &artistID, &track.FilePath, &track.FileName,
			&track.FileSize, &track.FileModified, &track.Title, &track.SortTitle,
			&track.TrackNumber, &track.DiscNumber, &track.Duration, &track.Year,
			&track.Genre, &track.MusicBrainzID, &track.Codec, &track.Bitrate,
			&track.SampleRate, &bitDepth, &track.Channels, &track.IsLossless, &track.IsHiRes,
			&track.CreatedAt, &track.UpdatedAt,
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

// escapeLikePattern escapes %, _, and \ so user input is treated literally
// inside a LIKE pattern (paired with `ESCAPE '\'` in the query).
func escapeLikePattern(s string) string {
	r := s
	r = strings.ReplaceAll(r, `\`, `\\`)
	r = strings.ReplaceAll(r, `%`, `\%`)
	r = strings.ReplaceAll(r, `_`, `\_`)
	return r
}

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

// UserHasLibraryAccess checks if a user has access to a library
func (r *repository) UserHasLibraryAccess(userID, libraryID int64) (bool, error) {
	//language=SQL
	query := `SELECT EXISTS(SELECT 1 FROM library_access WHERE user_id = ? AND library_id = ?)`

	var exists bool
	err := r.db.QueryRow(query, userID, libraryID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
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

// =============================================================================
// Scan queue methods
// =============================================================================

// QueueScan adds a scan job to the queue for a library
func (r *repository) QueueScan(libraryID int64) (*ScanJob, error) {
	//language=SQL
	query := `INSERT INTO scan_queue (library_id, status) VALUES (?, 'pending')`
	result, err := r.db.Exec(query, libraryID)
	if err != nil {
		return nil, err
	}

	jobID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.getScanJobByID(jobID)
}

// GetPendingScan retrieves the oldest pending scan job (FIFO)
func (r *repository) GetPendingScan() (*ScanJob, error) {
	//language=SQL
	query := `
SELECT id, library_id, requested_at, started_at, updated_at, status,
       total_files, processed_files, tracks_added, tracks_updated, errors, current_file
FROM scan_queue
WHERE status = 'pending'
ORDER BY requested_at ASC
LIMIT 1`

	job := &ScanJob{}
	var startedAt sql.NullTime
	err := r.db.QueryRow(query).Scan(
		&job.ID, &job.LibraryID, &job.RequestedAt, &startedAt, &job.UpdatedAt, &job.Status,
		&job.TotalFiles, &job.ProcessedFiles, &job.TracksAdded, &job.TracksUpdated,
		&job.Errors, &job.CurrentFile,
	)
	if err == sql.ErrNoRows {
		return nil, nil // No pending jobs
	}
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	return job, nil
}

// GetScanJobByLibrary retrieves active (pending/running) scan job for a library
func (r *repository) GetScanJobByLibrary(libraryID int64) (*ScanJob, error) {
	//language=SQL
	query := `
SELECT id, library_id, requested_at, started_at, updated_at, status,
       total_files, processed_files, tracks_added, tracks_updated, errors, current_file
FROM scan_queue
WHERE library_id = ? AND status IN ('pending', 'running')
ORDER BY requested_at DESC
LIMIT 1`

	job := &ScanJob{}
	var startedAt sql.NullTime
	err := r.db.QueryRow(query, libraryID).Scan(
		&job.ID, &job.LibraryID, &job.RequestedAt, &startedAt, &job.UpdatedAt, &job.Status,
		&job.TotalFiles, &job.ProcessedFiles, &job.TracksAdded, &job.TracksUpdated,
		&job.Errors, &job.CurrentFile,
	)
	if err == sql.ErrNoRows {
		return nil, nil // No active job for this library
	}
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	return job, nil
}

// UpdateScanJob updates a scan job's progress (also updates heartbeat timestamp)
func (r *repository) UpdateScanJob(job *ScanJob) error {
	//language=SQL
	query := `
UPDATE scan_queue SET
    started_at = ?, status = ?,
    total_files = ?, processed_files = ?,
    tracks_added = ?, tracks_updated = ?,
    errors = ?, current_file = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?`

	_, err := r.db.Exec(query,
		job.StartedAt, job.Status,
		job.TotalFiles, job.ProcessedFiles,
		job.TracksAdded, job.TracksUpdated,
		job.Errors, job.CurrentFile,
		job.ID,
	)
	return err
}

// DeleteScanJob removes a scan job from the queue
func (r *repository) DeleteScanJob(jobID int64) error {
	//language=SQL
	query := `DELETE FROM scan_queue WHERE id = ?`
	_, err := r.db.Exec(query, jobID)
	return err
}

// getScanJobByID is a helper to retrieve a scan job by ID
func (r *repository) getScanJobByID(id int64) (*ScanJob, error) {
	//language=SQL
	query := `
SELECT id, library_id, requested_at, started_at, updated_at, status,
       total_files, processed_files, tracks_added, tracks_updated, errors, current_file
FROM scan_queue
WHERE id = ?`

	job := &ScanJob{}
	var startedAt sql.NullTime
	err := r.db.QueryRow(query, id).Scan(
		&job.ID, &job.LibraryID, &job.RequestedAt, &startedAt, &job.UpdatedAt, &job.Status,
		&job.TotalFiles, &job.ProcessedFiles, &job.TracksAdded, &job.TracksUpdated,
		&job.Errors, &job.CurrentFile,
	)
	if err == sql.ErrNoRows {
		return nil, ErrScanJobNotFound
	}
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	return job, nil
}

// ResetOrphanedJobs resets "running" jobs that haven't been updated within the timeout
// (orphaned due to crash/restart). Returns number of jobs reset.
func (r *repository) ResetOrphanedJobs(timeout time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-timeout)
	//language=SQL
	query := `
UPDATE scan_queue
SET status = 'pending', started_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE status = 'running' AND updated_at < ?`

	result, err := r.db.Exec(query, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ResetAllRunningJobs resets every row with status='running' to 'pending',
// regardless of heartbeat freshness. This is the boot-time reset: a fresh
// worker process implies no scan is actually running, so all 'running' rows
// are by definition stale (the previous worker is dead).
func (r *repository) ResetAllRunningJobs() (int64, error) {
	//language=SQL
	query := `
UPDATE scan_queue
SET status = 'pending', started_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE status = 'running'`

	result, err := r.db.Exec(query)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

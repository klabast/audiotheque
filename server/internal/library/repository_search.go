package library

import (
	"database/sql"
	"strings"
)

// SearchAlbumsByLibrary returns albums with a title matching the LIKE pattern.
// LIKE in SQLite is case-insensitive for ASCII by default; the wrapper escapes
// %, _, and \ so user input can't expand the pattern.
func (r *repository) SearchAlbumsByLibrary(libraryID int64, query string, limit int) ([]*Album, error) {
	match := buildFTSMatch(query)
	if match == "" {
		return nil, nil
	}

	// title is weighted highest, then artist; genre/year are weak signals.
	//language=SQL
	q := `
SELECT a.id, a.library_id, a.artist_id, a.title, COALESCE(a.sort_title, ''), COALESCE(a.musicbrainz_id, ''),
       COALESCE(a.release_date, ''), COALESCE(a.genre, ''), COALESCE(a.total_tracks, 0), COALESCE(a.total_discs, 0),
       COALESCE(a.cover_art_path, ''), COALESCE(a.is_compilation, 0),
       EXISTS(SELECT 1 FROM track t WHERE t.album_id = a.id AND t.is_hires = 1) AS is_hires,
       COALESCE(a.folder_path, ''), COALESCE(a.release_type, 'original'),
       a.created_at, a.updated_at
FROM album_fts
JOIN album a ON a.id = album_fts.rowid
WHERE album_fts MATCH ? AND album_fts.library_id = ?
ORDER BY bm25(album_fts, 10.0, 5.0, 1.0, 1.0)
LIMIT ?`

	rows, err := r.db.Query(q, match, libraryID, limit)
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
	match := buildFTSMatch(query)
	if match == "" {
		return nil, nil
	}

	//language=SQL
	q := `
SELECT ar.id, ar.library_id, ar.name, COALESCE(ar.sort_name, ''), COALESCE(ar.musicbrainz_id, ''), ar.created_at, ar.updated_at
FROM artist_fts
JOIN artist ar ON ar.id = artist_fts.rowid
WHERE artist_fts MATCH ? AND artist_fts.library_id = ?
ORDER BY bm25(artist_fts)
LIMIT ?`

	rows, err := r.db.Query(q, match, libraryID, limit)
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
	match := buildFTSMatch(query)
	if match == "" {
		return nil, nil
	}

	//language=SQL
	q := `
SELECT t.id, t.library_id, t.album_id, t.artist_id, t.file_path, t.file_name, t.file_size, t.file_modified,
       t.title, COALESCE(t.sort_title, ''), t.track_number, t.disc_number, t.duration, t.year, COALESCE(t.genre, ''), COALESCE(t.musicbrainz_id, ''),
       COALESCE(t.codec, ''), t.bitrate, t.sample_rate, t.bit_depth, t.channels, t.is_lossless, t.is_hires,
       t.created_at, t.updated_at
FROM track_fts
JOIN track t ON t.id = track_fts.rowid
WHERE track_fts MATCH ? AND track_fts.library_id = ?
ORDER BY bm25(track_fts, 10.0, 5.0, 1.0, 1.0)
LIMIT ?`

	rows, err := r.db.Query(q, match, libraryID, limit)
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

// buildFTSMatch turns free-text user input into a safe FTS5 MATCH expression.
// Each whitespace-separated token is wrapped in double quotes (so punctuation
// can't be parsed as FTS syntax and inner quotes are escaped) with a trailing
// `*` for prefix / type-ahead matching, and the tokens are ANDed together so
// every word must match somewhere (title, artist, genre or year). Returns ""
// when the input has no usable tokens, letting callers skip the query.
func buildFTSMatch(query string) string {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return ""
	}
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		escaped := strings.ReplaceAll(f, `"`, `""`)
		parts = append(parts, `"`+escaped+`"*`)
	}
	return strings.Join(parts, " AND ")
}

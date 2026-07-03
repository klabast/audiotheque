package library

import (
	"database/sql"
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

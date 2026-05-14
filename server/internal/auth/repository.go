package auth

import (
	"database/sql"
	"errors"
	"time"
)

type repository struct {
	db *sql.DB
}

// NewRepository creates a new auth repository
func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

// GetByUsername retrieves a user by username
func (r *repository) GetByUsername(username string) (*User, error) {
	//language=SQL
	query := `
SELECT id,
       username,
       password_hash,
       is_admin,
       created_at,
       updated_at
FROM user
WHERE username = ?`

	user := &User{}
	err := r.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

// GetByID retrieves a user by ID
func (r *repository) GetByID(id int64) (*User, error) {
	//language=SQL
	query := `
SELECT id,
       username,
       password_hash,
       is_admin,
       created_at,
       updated_at
FROM user
WHERE id = ?`

	user := &User{}
	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

// GetUserCount returns the total number of users
func (r *repository) GetUserCount() (int, error) {
	//language=SQL
	query := `SELECT COUNT(*) FROM user`

	var count int
	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetAdminCount returns the number of admin users
func (r *repository) GetAdminCount() (int, error) {
	//language=SQL
	query := `SELECT COUNT(*) FROM user WHERE is_admin = 1`

	var count int
	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetFirstAdmin returns the lowest-id admin row. Used by auth-disabled mode
// to resolve every request to a deterministic "system" admin so downstream
// code (audit logs, ownership) still sees a real user pointer. Returns
// ErrUserNotFound when no admin exists (which in practice means the system
// hasn't been initialized).
func (r *repository) GetFirstAdmin() (*User, error) {
	//language=SQL
	query := `
SELECT id,
       username,
       password_hash,
       is_admin,
       created_at,
       updated_at
FROM user
WHERE is_admin = 1
ORDER BY id ASC
LIMIT 1`

	user := &User{}
	err := r.db.QueryRow(query).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// ListUsers returns every user row in id order. Feeds the admin Users
// settings panel and CLI listings.
func (r *repository) ListUsers() ([]*User, error) {
	//language=SQL
	query := `
SELECT id,
       username,
       password_hash,
       is_admin,
       created_at,
       updated_at
FROM user
ORDER BY id ASC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(
			&u.ID, &u.Username, &u.PasswordHash, &u.IsAdmin,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

// Delete removes a user row. ON DELETE CASCADE on the dependent tables
// (session, reset_code, library_access) handles the rest.
func (r *repository) Delete(userID int64) error {
	//language=SQL
	query := `DELETE FROM user WHERE id = ?`
	result, err := r.db.Exec(query, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

// Create creates a new user
func (r *repository) Create(username, passwordHash string, isAdmin bool) (*User, error) {
	//language=SQL
	query := `
INSERT INTO user (username, password_hash, is_admin)
VALUES (?, ?, ?)`

	_, err := r.db.Exec(query, username, passwordHash, isAdmin)
	if err != nil {
		return nil, err
	}

	// Fetch the created user
	return r.GetByUsername(username)
}

// UpdatePassword updates a user's password
func (r *repository) UpdatePassword(userID int64, newPasswordHash string) error {
	//language=SQL
	query := `
UPDATE user
SET password_hash = ?,
    updated_at    = CURRENT_TIMESTAMP
WHERE id = ?`

	result, err := r.db.Exec(query, newPasswordHash, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

// StoreResetCode stores a password reset code
func (r *repository) StoreResetCode(code string, userID int64, expiresAt time.Time) error {
	//language=SQL
	query := `
INSERT INTO reset_code (code, user_id, expires_at)
VALUES (?, ?, ?)`

	_, err := r.db.Exec(query, code, userID, expiresAt)
	return err
}

// GetResetCode retrieves a reset code
func (r *repository) GetResetCode(code string) (*ResetCode, error) {
	//language=SQL
	query := `
SELECT code, user_id, expires_at, created_at
FROM reset_code
WHERE code = ?`

	resetCode := &ResetCode{}
	err := r.db.QueryRow(query, code).Scan(
		&resetCode.Code,
		&resetCode.UserID,
		&resetCode.ExpiresAt,
		&resetCode.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidResetCode
		}
		return nil, err
	}

	return resetCode, nil
}

// DeleteResetCode deletes a specific reset code
func (r *repository) DeleteResetCode(code string) error {
	//language=SQL
	query := `DELETE FROM reset_code WHERE code = ?`

	_, err := r.db.Exec(query, code)
	return err
}

// DeleteResetCodesByUserID deletes all reset codes for a user
func (r *repository) DeleteResetCodesByUserID(userID int64) error {
	//language=SQL
	query := `DELETE FROM reset_code WHERE user_id = ?`

	_, err := r.db.Exec(query, userID)
	return err
}

// DeleteExpiredResetCodes deletes all expired reset codes
func (r *repository) DeleteExpiredResetCodes() error {
	//language=SQL
	query := `DELETE FROM reset_code WHERE expires_at < CURRENT_TIMESTAMP`

	_, err := r.db.Exec(query)
	return err
}

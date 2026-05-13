package settings

import (
	"database/sql"
	"time"
)

type SQLiteRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) CreateDevice(device *Device) error {
	//language=SQL
	query := `INSERT INTO device (id, name, type, address) VALUES (?, ?, ?, ?)`
	_, err := r.db.Exec(query, device.ID, device.Name, device.Type, device.Address)
	return err
}

func (r *SQLiteRepository) GetDevice(id string) (*Device, error) {
	//language=SQL
	query := `SELECT id, name, type, address, created_at, updated_at FROM device WHERE id = ?`
	row := r.db.QueryRow(query, id)

	var d Device
	var createdAt, updatedAt string
	err := row.Scan(&d.ID, &d.Name, &d.Type, &d.Address, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrDeviceNotFound
	}
	if err != nil {
		return nil, err
	}
	d.CreatedAt = parseTime(createdAt)
	d.UpdatedAt = parseTime(updatedAt)
	return &d, nil
}

func (r *SQLiteRepository) ListDevices() ([]Device, error) {
	//language=SQL
	query := `SELECT id, name, type, address, created_at, updated_at FROM device ORDER BY created_at`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		var createdAt, updatedAt string
		if err := rows.Scan(&d.ID, &d.Name, &d.Type, &d.Address, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		d.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		d.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		devices = append(devices, d)
	}
	if devices == nil {
		devices = []Device{}
	}
	return devices, rows.Err()
}

func (r *SQLiteRepository) UpdateDevice(device *Device) error {
	//language=SQL
	query := `UPDATE device SET name = ?, type = ?, address = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	result, err := r.db.Exec(query, device.Name, device.Type, device.Address, device.ID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrDeviceNotFound
	}
	return nil
}

func (r *SQLiteRepository) DeleteDevice(id string) error {
	//language=SQL
	query := `DELETE FROM device WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *SQLiteRepository) GetSetting(key string) (string, error) {
	//language=SQL
	query := `SELECT value FROM setting WHERE key = ?`
	var value string
	err := r.db.QueryRow(query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", ErrSettingNotFound
	}
	return value, err
}

// parseTime tries common SQLite timestamp formats.
func parseTime(s string) time.Time {
	for _, format := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	} {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (r *SQLiteRepository) SetSetting(key, value string) error {
	//language=SQL
	query := `INSERT INTO setting (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`
	_, err := r.db.Exec(query, key, value)
	return err
}

package settings

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/pressly/goose/v3"

	"audiod/internal/database"
)

func setupTestDB(t *testing.T) *SQLiteRepository {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Run migrations
	goose.SetBaseFS(database.EmbedMigrations())
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("failed to set goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return NewRepository(db)
}

func TestCreateAndGetDevice(t *testing.T) {
	repo := setupTestDB(t)

	device := &Device{
		ID:      "test-device-1",
		Name:    "Living Room Speaker",
		Type:    "mpd",
		Address: "192.168.1.10:6600",
	}

	err := repo.CreateDevice(device)
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	got, err := repo.GetDevice("test-device-1")
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}

	if got.ID != device.ID {
		t.Errorf("ID = %q, want %q", got.ID, device.ID)
	}
	if got.Name != device.Name {
		t.Errorf("Name = %q, want %q", got.Name, device.Name)
	}
	if got.Type != device.Type {
		t.Errorf("Type = %q, want %q", got.Type, device.Type)
	}
	if got.Address != device.Address {
		t.Errorf("Address = %q, want %q", got.Address, device.Address)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestListDevices(t *testing.T) {
	repo := setupTestDB(t)

	// Empty list initially
	devices, err := repo.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}
	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}

	// Add two devices
	_ = repo.CreateDevice(&Device{ID: "d1", Name: "Device 1", Type: "mpd", Address: "10.0.0.1:6600"})
	_ = repo.CreateDevice(&Device{ID: "d2", Name: "Device 2", Type: "mpd", Address: "10.0.0.2:6600"})

	devices, err = repo.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}
	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}
}

func TestUpdateDevice(t *testing.T) {
	repo := setupTestDB(t)

	_ = repo.CreateDevice(&Device{ID: "d1", Name: "Old Name", Type: "mpd", Address: "10.0.0.1:6600"})

	time.Sleep(10 * time.Millisecond)

	err := repo.UpdateDevice(&Device{ID: "d1", Name: "New Name", Type: "mpd", Address: "10.0.0.2:6600"})
	if err != nil {
		t.Fatalf("UpdateDevice failed: %v", err)
	}

	got, err := repo.GetDevice("d1")
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}
	if got.Name != "New Name" {
		t.Errorf("Name = %q, want %q", got.Name, "New Name")
	}
	if got.Address != "10.0.0.2:6600" {
		t.Errorf("Address = %q, want %q", got.Address, "10.0.0.2:6600")
	}
}

func TestDeleteDevice(t *testing.T) {
	repo := setupTestDB(t)

	_ = repo.CreateDevice(&Device{ID: "d1", Name: "Device", Type: "mpd", Address: "10.0.0.1:6600"})

	err := repo.DeleteDevice("d1")
	if err != nil {
		t.Fatalf("DeleteDevice failed: %v", err)
	}

	_, err = repo.GetDevice("d1")
	if err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestGetSetSetting(t *testing.T) {
	repo := setupTestDB(t)

	err := repo.SetSetting("streaming_hostname", "192.168.1.5:8080")
	if err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	val, err := repo.GetSetting("streaming_hostname")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if val != "192.168.1.5:8080" {
		t.Errorf("value = %q, want %q", val, "192.168.1.5:8080")
	}

	// Update existing setting
	err = repo.SetSetting("streaming_hostname", "10.0.0.1:9090")
	if err != nil {
		t.Fatalf("SetSetting (update) failed: %v", err)
	}

	val, err = repo.GetSetting("streaming_hostname")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if val != "10.0.0.1:9090" {
		t.Errorf("value = %q, want %q", val, "10.0.0.1:9090")
	}
}

func TestGetNonExistentSetting(t *testing.T) {
	repo := setupTestDB(t)

	_, err := repo.GetSetting("nonexistent")
	if err != ErrSettingNotFound {
		t.Errorf("expected ErrSettingNotFound, got %v", err)
	}
}

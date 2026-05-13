package playback

import (
	"testing"
)

// TDD: User can register a device and retrieve it
func TestDeviceRegistry_RegisterAndGet(t *testing.T) {
	registry := NewInMemoryDeviceRegistry()

	device := Device{
		ID:      "mpd-living-room",
		Name:    "Living Room",
		Type:    DeviceTypeMPD,
		Address: "192.168.1.10:6600",
		UserID:  1,
	}

	err := registry.Register(device)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := registry.Get("mpd-living-room")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "Living Room" {
		t.Errorf("Expected name 'Living Room', got '%s'", got.Name)
	}
	if got.Type != DeviceTypeMPD {
		t.Errorf("Expected type 'mpd', got '%s'", got.Type)
	}
}

// TDD: User can list all devices (devices are global by design)
func TestDeviceRegistry_ListByUser(t *testing.T) {
	registry := NewInMemoryDeviceRegistry()

	registry.Register(Device{ID: "mpd-1", Name: "Living Room", Type: DeviceTypeMPD, UserID: 1})
	registry.Register(Device{ID: "mpd-2", Name: "Bedroom", Type: DeviceTypeMPD, UserID: 1})
	registry.Register(Device{ID: "mpd-3", Name: "Other User", Type: DeviceTypeMPD, UserID: 2})

	devices, err := registry.ListByUser(1)
	if err != nil {
		t.Fatalf("ListByUser failed: %v", err)
	}

	if len(devices) != 3 {
		t.Fatalf("Expected 3 devices (all devices are global), got %d", len(devices))
	}
}

// TDD: Getting a non-existent device returns error
func TestDeviceRegistry_GetNotFound(t *testing.T) {
	registry := NewInMemoryDeviceRegistry()

	_, err := registry.Get("nonexistent")
	if err != ErrDeviceNotFound {
		t.Errorf("Expected ErrDeviceNotFound, got %v", err)
	}
}

// TDD: User can remove a device
func TestDeviceRegistry_Remove(t *testing.T) {
	registry := NewInMemoryDeviceRegistry()

	registry.Register(Device{ID: "mpd-1", Name: "Living Room", Type: DeviceTypeMPD, UserID: 1})

	err := registry.Remove("mpd-1")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	_, err = registry.Get("mpd-1")
	if err != ErrDeviceNotFound {
		t.Errorf("Expected ErrDeviceNotFound after remove, got %v", err)
	}
}

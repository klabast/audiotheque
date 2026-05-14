package settings

import (
	"testing"
)

// mockRepo implements Repository for testing the service layer.
type mockRepo struct {
	devices  map[string]*Device
	settings map[string]string
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		devices:  make(map[string]*Device),
		settings: make(map[string]string),
	}
}

func (m *mockRepo) CreateDevice(device *Device) error {
	m.devices[device.ID] = device
	return nil
}

func (m *mockRepo) GetDevice(id string) (*Device, error) {
	d, ok := m.devices[id]
	if !ok {
		return nil, ErrDeviceNotFound
	}
	return d, nil
}

func (m *mockRepo) ListDevices() ([]Device, error) {
	var result []Device
	for _, d := range m.devices {
		result = append(result, *d)
	}
	return result, nil
}

func (m *mockRepo) UpdateDevice(device *Device) error {
	if _, ok := m.devices[device.ID]; !ok {
		return ErrDeviceNotFound
	}
	m.devices[device.ID] = device
	return nil
}

func (m *mockRepo) DeleteDevice(id string) error {
	delete(m.devices, id)
	return nil
}

func (m *mockRepo) GetSetting(key string) (string, error) {
	v, ok := m.settings[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return v, nil
}

func (m *mockRepo) SetSetting(key, value string) error {
	m.settings[key] = value
	return nil
}

func TestServiceCreateDevice(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	device, err := svc.CreateDevice("Living Room", "mpd", "192.168.1.10:6600")
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	if device.ID == "" {
		t.Error("expected generated ID")
	}
	if device.Name != "Living Room" {
		t.Errorf("Name = %q, want %q", device.Name, "Living Room")
	}
	if device.Address != "192.168.1.10:6600" {
		t.Errorf("Address = %q, want %q", device.Address, "192.168.1.10:6600")
	}
}

func TestServiceListDevices(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	_, _ = svc.CreateDevice("Device 1", "mpd", "10.0.0.1:6600")
	_, _ = svc.CreateDevice("Device 2", "mpd", "10.0.0.2:6600")

	devices, err := svc.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}
	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}
}

func TestServiceUpdateDevice(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	device, _ := svc.CreateDevice("Old Name", "mpd", "10.0.0.1:6600")

	updated, err := svc.UpdateDevice(device.ID, "New Name", "10.0.0.2:6600")
	if err != nil {
		t.Fatalf("UpdateDevice failed: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("Name = %q, want %q", updated.Name, "New Name")
	}
	if updated.Address != "10.0.0.2:6600" {
		t.Errorf("Address = %q, want %q", updated.Address, "10.0.0.2:6600")
	}
}

func TestServiceDeleteDevice(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	device, _ := svc.CreateDevice("Device", "mpd", "10.0.0.1:6600")

	err := svc.DeleteDevice(device.ID)
	if err != nil {
		t.Fatalf("DeleteDevice failed: %v", err)
	}

	devices, _ := svc.ListDevices()
	if len(devices) != 0 {
		t.Errorf("expected 0 devices after delete, got %d", len(devices))
	}
}

func TestServiceStreamingHostnameDefault(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	hostname, err := svc.GetStreamingHostname()
	if err != nil {
		t.Fatalf("GetStreamingHostname failed: %v", err)
	}
	if hostname != "localhost:8080" {
		t.Errorf("hostname = %q, want %q", hostname, "localhost:8080")
	}
}

func TestServiceStreamingHostnameSetGet(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	err := svc.SetStreamingHostname("192.168.1.5:8080")
	if err != nil {
		t.Fatalf("SetStreamingHostname failed: %v", err)
	}

	hostname, err := svc.GetStreamingHostname()
	if err != nil {
		t.Fatalf("GetStreamingHostname failed: %v", err)
	}
	if hostname != "192.168.1.5:8080" {
		t.Errorf("hostname = %q, want %q", hostname, "192.168.1.5:8080")
	}
}

func TestServiceAuthEnabledDefaultsTrue(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	enabled, err := svc.IsAuthEnabled()
	if err != nil {
		t.Fatalf("IsAuthEnabled failed: %v", err)
	}
	if !enabled {
		t.Error("IsAuthEnabled() = false, want true (default when row missing)")
	}
}

func TestServiceAuthEnabledRoundTrip(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	if err := svc.SetAuthEnabled(false); err != nil {
		t.Fatalf("SetAuthEnabled(false) failed: %v", err)
	}
	enabled, err := svc.IsAuthEnabled()
	if err != nil {
		t.Fatalf("IsAuthEnabled failed: %v", err)
	}
	if enabled {
		t.Error("after SetAuthEnabled(false), IsAuthEnabled() = true, want false")
	}

	if err := svc.SetAuthEnabled(true); err != nil {
		t.Fatalf("SetAuthEnabled(true) failed: %v", err)
	}
	enabled, err = svc.IsAuthEnabled()
	if err != nil {
		t.Fatalf("IsAuthEnabled failed: %v", err)
	}
	if !enabled {
		t.Error("after SetAuthEnabled(true), IsAuthEnabled() = false, want true")
	}
}

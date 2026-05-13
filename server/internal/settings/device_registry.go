package settings

import (
	"audiod/internal/playback"
)

// DBDeviceRegistry adapts settings.Repository to playback.DeviceRegistry.
// Devices from settings are global (not per-user), so ListByUser returns all devices.
type DBDeviceRegistry struct {
	repo Repository
}

func NewDBDeviceRegistry(repo Repository) *DBDeviceRegistry {
	return &DBDeviceRegistry{repo: repo}
}

func (r *DBDeviceRegistry) Register(device playback.Device) error {
	d := &Device{
		ID:      device.ID,
		Name:    device.Name,
		Type:    string(device.Type),
		Address: device.Address,
	}
	return r.repo.CreateDevice(d)
}

func (r *DBDeviceRegistry) Get(id string) (playback.Device, error) {
	d, err := r.repo.GetDevice(id)
	if err != nil {
		if err == ErrDeviceNotFound {
			return playback.Device{}, playback.ErrDeviceNotFound
		}
		return playback.Device{}, err
	}
	return playback.Device{
		ID:      d.ID,
		Name:    d.Name,
		Type:    playback.DeviceType(d.Type),
		Address: d.Address,
	}, nil
}

func (r *DBDeviceRegistry) ListByUser(userID int64) ([]playback.Device, error) {
	devices, err := r.repo.ListDevices()
	if err != nil {
		return nil, err
	}
	result := make([]playback.Device, len(devices))
	for i, d := range devices {
		result[i] = playback.Device{
			ID:      d.ID,
			Name:    d.Name,
			Type:    playback.DeviceType(d.Type),
			Address: d.Address,
			UserID:  userID,
		}
	}
	return result, nil
}

func (r *DBDeviceRegistry) Remove(id string) error {
	return r.repo.DeleteDevice(id)
}

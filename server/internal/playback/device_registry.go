package playback

import "sync"

// DeviceRegistry manages playback devices
type DeviceRegistry interface {
	Register(device Device) error
	Get(id string) (Device, error)
	ListByUser(userID int64) ([]Device, error)
	Remove(id string) error
}

// InMemoryDeviceRegistry stores devices in memory
type InMemoryDeviceRegistry struct {
	devices map[string]Device // id -> device
	mu      sync.RWMutex
}

// NewInMemoryDeviceRegistry creates a new in-memory device registry
func NewInMemoryDeviceRegistry() *InMemoryDeviceRegistry {
	return &InMemoryDeviceRegistry{
		devices: make(map[string]Device),
	}
}

func (r *InMemoryDeviceRegistry) Register(device Device) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.devices[device.ID] = device
	return nil
}

func (r *InMemoryDeviceRegistry) Get(id string) (Device, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.devices[id]
	if !ok {
		return Device{}, ErrDeviceNotFound
	}
	return d, nil
}

func (r *InMemoryDeviceRegistry) ListByUser(userID int64) ([]Device, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Device
	for _, d := range r.devices {
		result = append(result, d)
	}
	return result, nil
}

func (r *InMemoryDeviceRegistry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.devices, id)
	return nil
}

package playback

import (
	"sync"
	"time"
)

// BrowserDevice describes an in-memory browser tab that can act as a playback
// target. Lifetime is the WebSocket connection: registered on register,
// removed on disconnect.
type BrowserDevice struct {
	ClientID    string
	Name        string
	UserID      int64
	ConnectedAt time.Time
}

// BrowserDeviceRegistry tracks live browser tabs. Tabs are referenced by their
// hub-assigned client ID. Concurrent reads are common (every device-list call
// reads a snapshot), writes are bursty during reconnects — RWMutex is fine.
type BrowserDeviceRegistry struct {
	mu      sync.RWMutex
	devices map[string]BrowserDevice // keyed by ClientID
}

// NewBrowserDeviceRegistry returns an empty registry.
func NewBrowserDeviceRegistry() *BrowserDeviceRegistry {
	return &BrowserDeviceRegistry{
		devices: make(map[string]BrowserDevice),
	}
}

// Register adds (or replaces) the device for clientID. UserID == 0 means
// "not authenticated"; we still track it so a reconnect with the same client
// can be replaced cleanly, but list operations skip userID=0 entries.
func (r *BrowserDeviceRegistry) Register(clientID string, userID int64, name string) {
	if clientID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.devices[clientID] = BrowserDevice{
		ClientID:    clientID,
		Name:        name,
		UserID:      userID,
		ConnectedAt: time.Now(),
	}
}

// Unregister removes the device for clientID. No-op if the ID isn't tracked.
func (r *BrowserDeviceRegistry) Unregister(clientID string) {
	if clientID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.devices, clientID)
}

// Get returns the device for clientID and a boolean indicating presence.
func (r *BrowserDeviceRegistry) Get(clientID string) (BrowserDevice, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.devices[clientID]
	return d, ok
}

// ListByUser returns the snapshot of all browser devices for the user.
// Anonymous (userID=0) devices are always excluded — they have no user to
// broadcast playback to anyway.
func (r *BrowserDeviceRegistry) ListByUser(userID int64) []BrowserDevice {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]BrowserDevice, 0, len(r.devices))
	for _, d := range r.devices {
		if d.UserID == 0 || d.UserID != userID {
			continue
		}
		out = append(out, d)
	}
	return out
}

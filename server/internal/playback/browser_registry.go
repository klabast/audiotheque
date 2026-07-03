package playback

import (
	"sync"
	"time"
)

// disconnectGracePeriod is how long a browser device stays resolvable after
// its WebSocket connection drops. A LAN→WLAN roam (or any reconnect) briefly
// leaves the device unregistered while the new connection is still being
// established; without a grace window GetSession would see the deviceID as
// unresolvable and delete the session out from under the reconnecting tab.
// It's a package var rather than a constant so tests can shrink it instead of
// sleeping a full minute; writes only happen from test setup, never
// concurrently with production traffic.
var disconnectGracePeriod = 60 * time.Second

// BrowserDevice describes an in-memory browser tab that can act as a playback
// target. Lifetime is the WebSocket connection: registered on register,
// removed on disconnect (after disconnectGracePeriod, unless re-registered).
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
	mu             sync.RWMutex
	devices        map[string]BrowserDevice // keyed by ClientID
	pendingRemoval map[string]*time.Timer   // keyed by ClientID; armed by Unregister, disarmed by Register
}

// NewBrowserDeviceRegistry returns an empty registry.
func NewBrowserDeviceRegistry() *BrowserDeviceRegistry {
	return &BrowserDeviceRegistry{
		devices:        make(map[string]BrowserDevice),
		pendingRemoval: make(map[string]*time.Timer),
	}
}

// Register adds (or replaces) the device for clientID. UserID == 0 means
// "not authenticated"; we still track it so a reconnect with the same client
// can be replaced cleanly, but list operations skip userID=0 entries.
//
// Re-registering a clientID that has a pending grace-period removal (i.e. it
// disconnected and reconnected before the timer fired) cancels that timer —
// the device never disappears from the caller's point of view.
func (r *BrowserDeviceRegistry) Register(clientID string, userID int64, name string) {
	if clientID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if timer, ok := r.pendingRemoval[clientID]; ok {
		timer.Stop()
		delete(r.pendingRemoval, clientID)
	}
	r.devices[clientID] = BrowserDevice{
		ClientID:    clientID,
		Name:        name,
		UserID:      userID,
		ConnectedAt: time.Now(),
	}
}

// Unregister marks the device for clientID as disconnected. The entry is not
// removed immediately — it survives for disconnectGracePeriod so a fast
// reconnect (same clientID re-registering) never observes a gap. If the grace
// period elapses without a re-register, the device is actually deleted.
// No-op if the ID isn't tracked.
func (r *BrowserDeviceRegistry) Unregister(clientID string) {
	if clientID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.devices[clientID]; !ok {
		return
	}
	if timer, ok := r.pendingRemoval[clientID]; ok {
		timer.Stop()
	}
	r.pendingRemoval[clientID] = time.AfterFunc(disconnectGracePeriod, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.devices, clientID)
		delete(r.pendingRemoval, clientID)
	})
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

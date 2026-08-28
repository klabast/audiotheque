package playback

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"audiod/internal/mpd"
)

// mpdDialTimeout bounds how long a connect attempt may take. gompd dials with
// no timeout of its own, so an unreachable host would otherwise hold the
// device's slot for the OS TCP connect timeout.
const mpdDialTimeout = 5 * time.Second

// RegistryDeviceResolver resolves device IDs to PlaybackDevices by looking up
// devices in the registry, dialing MPD, and caching connections.
//
// Browser tabs are not in the persisted DeviceRegistry — they live in a
// separate ephemeral BrowserDeviceRegistry attached via SetBrowserRegistry.
// When a deviceID matches a browser tab the resolver returns a no-op
// BrowserPlaybackDevice; the actual audio is driven by the target tab itself
// (driven by the broadcast `playback-session` message it receives over WS).
//
// r.mu guards only the maps and is never held across network I/O. Connecting
// and health-checking one device happens under that device's own slot lock, so
// an MPD box that stops answering can't wedge playback for every other device
// and every other user.
type RegistryDeviceResolver struct {
	registry        DeviceRegistry
	browserRegistry *BrowserDeviceRegistry
	slots           map[string]*deviceSlot
	mu              sync.Mutex
	logger          *slog.Logger
	// dial opens a connection to an MPD address. Injectable so tests can
	// observe connection lifecycle without a live daemon.
	dial func(addr string) (mpd.Client, error)
}

// deviceSlot owns the cached connection for one device ID. It stays in the map
// for the process's lifetime; only the connection inside comes and goes.
type deviceSlot struct {
	mu     sync.Mutex
	device *MPDPlaybackDevice
}

// NewRegistryDeviceResolver creates a resolver backed by a device registry.
func NewRegistryDeviceResolver(registry DeviceRegistry, logger *slog.Logger) *RegistryDeviceResolver {
	return &RegistryDeviceResolver{
		registry: registry,
		slots:    make(map[string]*deviceSlot),
		logger:   logger,
		dial:     func(addr string) (mpd.Client, error) { return mpd.Dial(addr) },
	}
}

// SetBrowserRegistry wires the ephemeral browser-tab registry. When set, the
// resolver recognises browser-tab client IDs and returns a no-op device for
// them so the service layer's transfer/play/pause flow stays uniform.
func (r *RegistryDeviceResolver) SetBrowserRegistry(reg *BrowserDeviceRegistry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.browserRegistry = reg
}

// IsBrowserDevice reports whether the ID names a live browser tab. Pure map
// lookup, no I/O — the service calls it on every session persist to keep
// browser sessions out of the MPD poller.
func (r *RegistryDeviceResolver) IsBrowserDevice(deviceID string) bool {
	r.mu.Lock()
	reg := r.browserRegistry
	r.mu.Unlock()
	if reg == nil {
		return false
	}
	_, ok := reg.Get(deviceID)
	return ok
}

// ResolveDevice returns a PlaybackDevice for the given device ID. Cached
// connections are reused; unhealthy ones are closed and re-established.
//
// Errors are sentinel-wrapped so callers can tell a device that is gone
// (ErrDeviceNotFound — the session bound to it is an orphan) from one that is
// merely unreachable right now (ErrDeviceUnreachable — leave the session be).
func (r *RegistryDeviceResolver) ResolveDevice(deviceID string) (PlaybackDevice, error) {
	r.mu.Lock()
	browserRegistry := r.browserRegistry
	r.mu.Unlock()

	// Browser tab? No connection to dial — return the no-op device. Checked
	// before claiming a slot so short-lived tab IDs don't accumulate one each.
	if browserRegistry != nil {
		if _, ok := browserRegistry.Get(deviceID); ok {
			r.logger.Debug("resolved as browser device", "deviceID", deviceID)
			return &BrowserPlaybackDevice{}, nil
		}
	}

	r.mu.Lock()
	slot, ok := r.slots[deviceID]
	if !ok {
		slot = &deviceSlot{}
		r.slots[deviceID] = slot
	}
	r.mu.Unlock()

	slot.mu.Lock()
	defer slot.mu.Unlock()

	// Reuse the cached connection if it still answers.
	if cached := slot.device; cached != nil {
		if _, err := cached.Status(); err == nil {
			r.logger.Debug("using cached device connection", "deviceID", deviceID)
			return cached, nil
		}
		r.logger.Debug("cached connection stale, reconnecting", "deviceID", deviceID)
		slot.device = nil
		if err := cached.Close(); err != nil {
			r.logger.Debug("close stale connection failed", "deviceID", deviceID, "error", err)
		}
	}

	device, err := r.registry.Get(deviceID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDeviceNotFound, deviceID)
	}
	if device.Type != DeviceTypeMPD {
		return nil, fmt.Errorf("%w: unsupported device type %s", ErrDeviceNotFound, device.Type)
	}

	r.logger.Debug("dialing MPD", "deviceID", deviceID, "address", device.Address)
	client, err := r.dialWithTimeout(device.Address)
	if err != nil {
		return nil, fmt.Errorf("%w: connect to MPD at %s: %v", ErrDeviceUnreachable, device.Address, err)
	}

	mpdDevice := NewMPDPlaybackDevice(client)
	slot.device = mpdDevice
	r.logger.Info("connected to MPD device", "deviceID", deviceID, "address", device.Address)

	return mpdDevice, nil
}

// dialWithTimeout bounds the connect attempt. A dial that lands after the
// deadline is closed by its own goroutine — nothing else holds a reference to
// that client, so abandoning it is safe.
func (r *RegistryDeviceResolver) dialWithTimeout(addr string) (mpd.Client, error) {
	type dialResult struct {
		client mpd.Client
		err    error
	}
	done := make(chan dialResult, 1)
	go func() {
		client, err := r.dial(addr)
		done <- dialResult{client, err}
	}()

	timer := time.NewTimer(mpdDialTimeout)
	defer timer.Stop()
	select {
	case res := <-done:
		return res.client, res.err
	case <-timer.C:
		go func() {
			if res := <-done; res.client != nil {
				_ = res.client.Close()
			}
		}()
		return nil, fmt.Errorf("dial timed out after %s", mpdDialTimeout)
	}
}

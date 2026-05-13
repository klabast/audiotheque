package playback

import (
	"fmt"
	"log/slog"
	"sync"

	"audiod/internal/mpd"
)

// RegistryDeviceResolver resolves device IDs to PlaybackDevices by looking up
// devices in the registry, dialing MPD, and caching connections.
//
// Browser tabs are not in the persisted DeviceRegistry — they live in a
// separate ephemeral BrowserDeviceRegistry attached via SetBrowserRegistry.
// When a deviceID matches a browser tab the resolver returns a no-op
// BrowserPlaybackDevice; the actual audio is driven by the target tab itself
// (driven by the broadcast `playback-session` message it receives over WS).
type RegistryDeviceResolver struct {
	registry        DeviceRegistry
	browserRegistry *BrowserDeviceRegistry
	connections     map[string]PlaybackDevice
	mu              sync.Mutex
	logger          *slog.Logger
}

// NewRegistryDeviceResolver creates a resolver backed by a device registry.
func NewRegistryDeviceResolver(registry DeviceRegistry, logger *slog.Logger) *RegistryDeviceResolver {
	return &RegistryDeviceResolver{
		registry:    registry,
		connections: make(map[string]PlaybackDevice),
		logger:      logger,
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

// ResolveDevice returns a PlaybackDevice for the given device ID.
// Cached connections are reused; unhealthy connections are re-established.
func (r *RegistryDeviceResolver) ResolveDevice(deviceID string) (PlaybackDevice, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Browser tab? No connection to dial — return the no-op device.
	if r.browserRegistry != nil {
		if _, ok := r.browserRegistry.Get(deviceID); ok {
			r.logger.Debug("resolved as browser device", "deviceID", deviceID)
			return &BrowserPlaybackDevice{}, nil
		}
	}

	// Check cache first — verify the connection is still healthy
	if cached, ok := r.connections[deviceID]; ok {
		if _, err := cached.Status(); err == nil {
			r.logger.Debug("using cached device connection", "deviceID", deviceID)
			return cached, nil
		}
		// Stale connection — remove and reconnect
		r.logger.Debug("cached connection stale, reconnecting", "deviceID", deviceID)
		delete(r.connections, deviceID)
	}

	// Look up device in registry
	device, err := r.registry.Get(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found in registry: %w", err)
	}

	if device.Type != DeviceTypeMPD {
		return nil, fmt.Errorf("unsupported device type: %s", device.Type)
	}

	// Dial MPD
	r.logger.Debug("dialing MPD", "deviceID", deviceID, "address", device.Address)
	client, err := mpd.Dial(device.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MPD at %s: %w", device.Address, err)
	}

	mpdDevice := NewMPDPlaybackDevice(client)
	r.connections[deviceID] = mpdDevice
	r.logger.Info("connected to MPD device", "deviceID", deviceID, "address", device.Address)

	return mpdDevice, nil
}

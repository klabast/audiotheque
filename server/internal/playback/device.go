package playback

import (
	"errors"
	"fmt"
	"strings"

	"audiod/internal/mpd"
)

// ErrVolumeNotSupported is returned by PlaybackDevice.SetVolume when the
// device physically cannot change its volume (e.g. an MPD configured with
// `mixer_type "none"`). Service callers should treat this as benign:
// persist the user's volume intent in the session and report success at
// the API layer. Use errors.Is(err, ErrVolumeNotSupported) to detect.
var ErrVolumeNotSupported = errors.New("device does not support volume control")

// DeviceStatus represents the current state reported by a playback device
type DeviceStatus struct {
	State   string // "play", "pause", "stop", "browser"
	Elapsed int
	Volume  int
	// SongID is MPD's per-Add identifier for the song currently in slot 0.
	// Treated as opaque — used by the position poller to detect track
	// transitions independent of state-machine timing. Empty for browser
	// devices and when MPD has nothing loaded.
	SongID string
}

// PlaybackDevice abstracts where audio actually plays
type PlaybackDevice interface {
	Play(trackURL string, position int) error
	Pause() error
	Resume() error
	Stop() error
	Seek(position int) error
	SetVolume(volume int) error
	Status() (DeviceStatus, error)
	// SupportsVolume reports whether SetVolume can change device-side volume.
	// Surfaced on SessionResponse.deviceCapabilities so the UI can grey out
	// the slider on mixerless MPD configurations.
	SupportsVolume() bool
}

// BrowserPlaybackDevice is a no-op on the server side.
// The browser handles its own audio via HTMLAudioElement.
type BrowserPlaybackDevice struct{}

func (d *BrowserPlaybackDevice) Play(trackURL string, position int) error {
	return nil
}

func (d *BrowserPlaybackDevice) Pause() error {
	return nil
}

func (d *BrowserPlaybackDevice) Resume() error {
	return nil
}

func (d *BrowserPlaybackDevice) Stop() error {
	return nil
}

func (d *BrowserPlaybackDevice) Seek(position int) error {
	return nil
}

func (d *BrowserPlaybackDevice) SetVolume(volume int) error {
	return nil
}

// SupportsVolume returns true: HTMLAudioElement always exposes a volume
// property, so the UI's volume slider is meaningful for browser playback.
func (d *BrowserPlaybackDevice) SupportsVolume() bool {
	return true
}

func (d *BrowserPlaybackDevice) Status() (DeviceStatus, error) {
	// The server doesn't know the browser tab's audio state — the tab pushes
	// its own position over WS. Returning an error here keeps callers like
	// TransferPlayback from clobbering session.Current.Position with 0 when
	// the source device is a browser tab.
	return DeviceStatus{State: "browser"}, fmt.Errorf("browser device status not available server-side")
}

// MPDPlaybackDevice routes commands to an MPD server
type MPDPlaybackDevice struct {
	client mpd.Client
	// supportsVolume starts true and flips false the first time SetVolume
	// returns ErrVolumeNotSupported. Cached so SessionResponse can populate
	// deviceCapabilities.volume without re-probing MPD on every status read.
	supportsVolume bool
}

// NewMPDPlaybackDevice creates a new MPD playback device
func NewMPDPlaybackDevice(client mpd.Client) *MPDPlaybackDevice {
	return &MPDPlaybackDevice{client: client, supportsVolume: true}
}

// SupportsVolume reports whether this device has accepted a setvol command
// since connection. Defaults true; flips false after a single
// ErrVolumeNotSupported observation. The flag is connection-scoped: when
// the resolver dials a fresh connection, the new MPDPlaybackDevice instance
// starts optimistic again.
func (d *MPDPlaybackDevice) SupportsVolume() bool {
	return d.supportsVolume
}

func (d *MPDPlaybackDevice) Play(trackURL string, position int) error {
	if err := d.client.LoadURL(trackURL); err != nil {
		return fmt.Errorf("mpd load: %w", err)
	}
	if err := d.client.Play(); err != nil {
		return fmt.Errorf("mpd play: %w", err)
	}
	if position > 0 {
		if err := d.client.Seek(position); err != nil {
			return fmt.Errorf("mpd seek: %w", err)
		}
	}
	// The previous implementation busy-polled Status() up to 10×30ms after
	// Seek to keep the position poller from observing a pre-seek elapsed
	// value and writing position=0. That guard is no longer needed: every
	// caller of Play() persists the authoritative position via
	// persistAndBroadcast immediately after we return, so any racy poll
	// write is overwritten on the very next save.
	return nil
}

func (d *MPDPlaybackDevice) Pause() error {
	return d.client.Pause()
}

func (d *MPDPlaybackDevice) Resume() error {
	return d.client.Play()
}

func (d *MPDPlaybackDevice) Stop() error {
	return d.client.Stop()
}

func (d *MPDPlaybackDevice) Seek(position int) error {
	return d.client.Seek(position)
}

func (d *MPDPlaybackDevice) SetVolume(volume int) error {
	err := d.client.SetVolume(volume)
	if err == nil {
		return nil
	}
	// Real MPD returns "ACK [52@0] {setvol} problems setting volume: No mixer"
	// when the audio output has `mixer_type "none"` (HiFiBerry's default).
	// Match either substring so we tolerate minor wording drift across MPD
	// versions while still failing closed on unexpected errors.
	msg := err.Error()
	if strings.Contains(msg, "No mixer") || strings.Contains(msg, "problems setting volume") {
		d.supportsVolume = false
		return fmt.Errorf("%w: %v", ErrVolumeNotSupported, err)
	}
	return err
}

func (d *MPDPlaybackDevice) Status() (DeviceStatus, error) {
	status, err := d.client.Status()
	if err != nil {
		return DeviceStatus{}, fmt.Errorf("mpd status: %w", err)
	}
	return DeviceStatus{
		State:   status.State,
		Elapsed: status.Elapsed,
		Volume:  status.Volume,
		SongID:  status.SongID,
	}, nil
}

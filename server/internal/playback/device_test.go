package playback

import (
	"errors"
	"testing"

	"audiod/internal/mpd"
	"audiod/internal/mpd/testserver"
)

// TestPlaybackDevice_InterfaceExposesSupportsVolume pins SupportsVolume onto
// the PlaybackDevice interface so callers can query the capability directly
// instead of type-asserting at the call site (handler.capabilitiesFor used to
// do `device.(interface{ SupportsVolume() bool })`).
func TestPlaybackDevice_InterfaceExposesSupportsVolume(t *testing.T) {
	var d PlaybackDevice = &BrowserPlaybackDevice{}
	if !d.SupportsVolume() {
		t.Error("BrowserPlaybackDevice should report SupportsVolume=true (HTMLAudioElement.volume)")
	}
	d = NewMPDPlaybackDevice(nil)
	if !d.SupportsVolume() {
		t.Error("MPDPlaybackDevice should default SupportsVolume=true before any setvol attempt")
	}
}

// TDD: BrowserPlaybackDevice is a no-op on the server side
func TestBrowserPlaybackDevice_NoOp(t *testing.T) {
	device := &BrowserPlaybackDevice{}

	// All operations should succeed silently
	if err := device.Play("http://example.com/track.mp3", 0); err != nil {
		t.Errorf("Play should be no-op, got error: %v", err)
	}
	if err := device.Pause(); err != nil {
		t.Errorf("Pause should be no-op, got error: %v", err)
	}
	if err := device.Resume(); err != nil {
		t.Errorf("Resume should be no-op, got error: %v", err)
	}
	if err := device.Stop(); err != nil {
		t.Errorf("Stop should be no-op, got error: %v", err)
	}
	if err := device.Seek(30); err != nil {
		t.Errorf("Seek should be no-op, got error: %v", err)
	}
	if err := device.SetVolume(50); err != nil {
		t.Errorf("SetVolume should be no-op, got error: %v", err)
	}

	// Status() returns an error: the server doesn't know the tab's audio
	// state. The "browser" state is still reported in the struct so callers
	// that ignore the error can still distinguish browser from MPD.
	status, err := device.Status()
	if err == nil {
		t.Errorf("Status should return an error for browser devices (server doesn't know tab state)")
	}
	if status.State != "browser" {
		t.Errorf("Expected state 'browser', got '%s'", status.State)
	}
}

// TDD: Play(url, position) issues exactly one Status() call (or zero — the
// MPD device shouldn't need to read state to write state). The previous
// implementation busy-polled Status() up to 10 times after Seek to guard
// against a race that's now closed by persisting the authoritative position
// in service.persistAndBroadcast immediately after Play returns.
func TestMPDPlaybackDevice_Play_DoesNotPollStatus(t *testing.T) {
	c := &countingMPDClient{}
	device := NewMPDPlaybackDevice(c)

	if err := device.Play("http://x/stream", 60); err != nil {
		t.Fatalf("Play failed: %v", err)
	}

	if c.statusCalls > 0 {
		t.Errorf("Play polled Status %d times — expected 0 (no busy-wait after seek)", c.statusCalls)
	}
	if !c.loadCalled {
		t.Error("Play should LoadURL")
	}
	if !c.playCalled {
		t.Error("Play should Play")
	}
	if c.seekPosition != 60 {
		t.Errorf("Play should Seek to 60, got %d", c.seekPosition)
	}
}

// countingMPDClient is a minimal mpd.Client that records calls so we can
// assert the device's command pattern without timing-dependent assertions.
type countingMPDClient struct {
	loadCalled   bool
	playCalled   bool
	seekPosition int
	statusCalls  int
}

func (c *countingMPDClient) Play() error                      { c.playCalled = true; return nil }
func (c *countingMPDClient) Pause() error                     { return nil }
func (c *countingMPDClient) Stop() error                      { return nil }
func (c *countingMPDClient) SetVolume(int) error              { return nil }
func (c *countingMPDClient) Seek(p int) error                 { c.seekPosition = p; return nil }
func (c *countingMPDClient) LoadURL(string) error             { c.loadCalled = true; return nil }
func (c *countingMPDClient) Close() error                     { return nil }
func (c *countingMPDClient) Status() (mpd.Status, error) {
	c.statusCalls++
	return mpd.Status{State: "play", Elapsed: c.seekPosition}, nil
}
func (c *countingMPDClient) CurrentSong() (mpd.CurrentSong, error) {
	return mpd.CurrentSong{}, nil
}

// TDD: MPDPlaybackDevice routes commands to an MPD server
func TestMPDPlaybackDevice_Play(t *testing.T) {
	// Start fake MPD
	srv := testserver.New()
	defer srv.Close()

	// Connect
	client, err := mpd.Dial(srv.Addr())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	device := NewMPDPlaybackDevice(client)

	// Play a track
	err = device.Play("http://localhost:8080/api/tracks/1/stream", 0)
	if err != nil {
		t.Fatalf("Play failed: %v", err)
	}

	// Verify MPD received the command
	state := srv.State()
	if state.PlayState != "play" {
		t.Errorf("Expected MPD state 'play', got '%s'", state.PlayState)
	}
	if state.CurrentFile != "http://localhost:8080/api/tracks/1/stream" {
		t.Errorf("Expected track URL in MPD, got '%s'", state.CurrentFile)
	}
}

// TDD: MPDPlaybackDevice can pause and resume
func TestMPDPlaybackDevice_PauseResume(t *testing.T) {
	srv := testserver.New()
	defer srv.Close()

	client, err := mpd.Dial(srv.Addr())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	device := NewMPDPlaybackDevice(client)
	device.Play("http://example.com/track.mp3", 0)

	// Pause
	err = device.Pause()
	if err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	state := srv.State()
	if state.PlayState != "pause" {
		t.Errorf("Expected 'pause', got '%s'", state.PlayState)
	}

	// Resume
	err = device.Resume()
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	state = srv.State()
	if state.PlayState != "play" {
		t.Errorf("Expected 'play', got '%s'", state.PlayState)
	}
}

// MPD with `mixer_type "none"` (e.g. HiFiBerry default) returns an ACK on
// setvol. The device layer must surface this as a sentinel that callers can
// recognize and tolerate via errors.Is, without breaking the volume API for
// the user.
func TestMPDPlaybackDevice_SetVolume_NoMixer_ReturnsErrVolumeNotSupported(t *testing.T) {
	srv := testserver.New()
	defer srv.Close()
	srv.SetMixerless(true)

	client, err := mpd.Dial(srv.Addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	device := NewMPDPlaybackDevice(client)
	err = device.SetVolume(50)
	if err == nil {
		t.Fatal("expected SetVolume to error on mixerless device")
	}
	if !errors.Is(err, ErrVolumeNotSupported) {
		t.Errorf("expected errors.Is(err, ErrVolumeNotSupported); got: %v", err)
	}
}

// After observing ErrVolumeNotSupported once, the MPD device should expose
// SupportsVolume() == false so SessionResponse can surface the capability
// hint to the UI without re-probing on every request.
func TestMPDPlaybackDevice_SupportsVolume_FalseAfterNoMixerObservation(t *testing.T) {
	srv := testserver.New()
	defer srv.Close()
	srv.SetMixerless(true)

	client, err := mpd.Dial(srv.Addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	device := NewMPDPlaybackDevice(client)
	if !device.SupportsVolume() {
		t.Fatal("SupportsVolume should default to true before any setvol attempt")
	}
	_ = device.SetVolume(50)
	if device.SupportsVolume() {
		t.Error("SupportsVolume should be false after observing ErrVolumeNotSupported")
	}
}

// TDD: MPDPlaybackDevice reports status from MPD
func TestMPDPlaybackDevice_Status(t *testing.T) {
	srv := testserver.New()
	defer srv.Close()

	client, err := mpd.Dial(srv.Addr())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	device := NewMPDPlaybackDevice(client)
	device.Play("http://example.com/track.mp3", 0)
	device.SetVolume(75)

	status, err := device.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.State != "play" {
		t.Errorf("Expected state 'play', got '%s'", status.State)
	}
	if status.Volume != 75 {
		t.Errorf("Expected volume 75, got %d", status.Volume)
	}
}

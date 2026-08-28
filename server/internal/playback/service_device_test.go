package playback

import (
	"errors"
	"testing"
)

// mockPlaybackDevice tracks calls for test assertions
type mockPlaybackDevice struct {
	playCalled   bool
	playTrackURL string
	playPosition int
	pauseCalled  bool
	resumeCalled bool
	stopCalled   bool
	seekPosition int
	volume       int
	state        string
	songID       string

	// Error injection for resilience tests. nil means succeed.
	setVolumeErr error
	playErr      error
	statusErr    error
}

func (m *mockPlaybackDevice) Play(trackURL string, position int) error {
	if m.playErr != nil {
		return m.playErr
	}
	m.playCalled = true
	m.playTrackURL = trackURL
	m.playPosition = position
	m.state = "play"
	return nil
}

func (m *mockPlaybackDevice) Pause() error {
	m.pauseCalled = true
	m.state = "pause"
	return nil
}

func (m *mockPlaybackDevice) Resume() error {
	m.resumeCalled = true
	m.state = "play"
	return nil
}

func (m *mockPlaybackDevice) Stop() error {
	m.stopCalled = true
	m.state = "stop"
	return nil
}

func (m *mockPlaybackDevice) Seek(position int) error {
	m.seekPosition = position
	return nil
}

func (m *mockPlaybackDevice) SetVolume(volume int) error {
	if m.setVolumeErr != nil {
		return m.setVolumeErr
	}
	m.volume = volume
	return nil
}

func (m *mockPlaybackDevice) Status() (DeviceStatus, error) {
	if m.statusErr != nil {
		return DeviceStatus{}, m.statusErr
	}
	return DeviceStatus{State: m.state, Elapsed: m.seekPosition, Volume: m.volume, SongID: m.songID}, nil
}

func (m *mockPlaybackDevice) SupportsVolume() bool {
	return m.setVolumeErr == nil || !errors.Is(m.setVolumeErr, ErrVolumeNotSupported)
}

// mockDeviceResolver maps device IDs to mock devices for testing. errs lets a
// test make a known device fail to resolve (e.g. ErrDeviceUnreachable for an
// MPD box that is rebooting); browsers marks IDs the resolver considers
// browser tabs, mirroring RegistryDeviceResolver.IsBrowserDevice.
type mockDeviceResolver struct {
	devices  map[string]PlaybackDevice
	errs     map[string]error
	browsers map[string]bool
}

func (m *mockDeviceResolver) ResolveDevice(deviceID string) (PlaybackDevice, error) {
	if err, ok := m.errs[deviceID]; ok {
		return nil, err
	}
	if d, ok := m.devices[deviceID]; ok {
		return d, nil
	}
	return nil, ErrDeviceNotFound
}

func (m *mockDeviceResolver) IsBrowserDevice(deviceID string) bool {
	return m.browsers[deviceID]
}

// TDD: Playing an album with a device ID sends play command to that device
func TestService_PlayAlbum_WithDevice(t *testing.T) {
	trackProvider := &mockTrackProvider{
		tracks: []Track{
			{ID: 1, Title: "Track 1", AlbumID: 100, Duration: 180},
			{ID: 2, Title: "Track 2", AlbumID: 100, Duration: 200},
		},
	}
	sessionRepo := &mockSessionRepository{}
	mpdDevice := &mockPlaybackDevice{}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{
			"mpd-living-room": mpdDevice,
		},
	}

	service := NewService(trackProvider, sessionRepo)
	service.SetDeviceResolver(resolver)
	service.SetStreamURLBuilder(func(trackID, _ int64) string {
		return "http://localhost:8080/api/tracks/1/stream"
	})

	session, err := service.PlayAlbumOnDevice(1, 100, 0, "mpd-living-room")
	if err != nil {
		t.Fatalf("PlayAlbumOnDevice failed: %v", err)
	}

	// Session should have device ID set
	if session.DeviceID != "mpd-living-room" {
		t.Errorf("Expected DeviceID 'mpd-living-room', got '%s'", session.DeviceID)
	}

	// Device should have received play command
	if !mpdDevice.playCalled {
		t.Error("Expected play to be called on MPD device")
	}
	if mpdDevice.playTrackURL != "http://localhost:8080/api/tracks/1/stream" {
		t.Errorf("Expected stream URL, got '%s'", mpdDevice.playTrackURL)
	}
}

// TDD: Pause with device forwards to the device
func TestService_Pause_WithDevice(t *testing.T) {
	mpdDevice := &mockPlaybackDevice{state: "play"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd-1": mpdDevice},
	}

	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 0},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	service := NewService(nil, sessionRepo)
	service.SetDeviceResolver(resolver)

	_, err := service.Pause(1, 45)
	if err != nil {
		t.Fatalf("Pause failed: %v", err)
	}

	if !mpdDevice.pauseCalled {
		t.Error("Expected pause to be called on MPD device")
	}
}

// TDD: Resume with device forwards to the device
func TestService_Resume_WithDevice(t *testing.T) {
	mpdDevice := &mockPlaybackDevice{state: "pause"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd-1": mpdDevice},
	}

	existingSession := &Session{
		UserID:   1,
		State:    StatePaused,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 45},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	service := NewService(nil, sessionRepo)
	service.SetDeviceResolver(resolver)

	_, err := service.Resume(1)
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}

	if !mpdDevice.resumeCalled {
		t.Error("Expected resume to be called on MPD device")
	}
}

// TDD: Next with device plays the next track on the device
func TestService_Next_WithDevice(t *testing.T) {
	mpdDevice := &mockPlaybackDevice{state: "play"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd-1": mpdDevice},
	}

	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 1, Position: 100},
		Source: Source{
			Type:      SourceTypeAlbum,
			ID:        100,
			Remaining: []int64{2, 3},
		},
		Queue:   []QueueItem{},
		History: []int64{},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	service := NewService(nil, sessionRepo)
	service.SetDeviceResolver(resolver)
	service.SetStreamURLBuilder(func(trackID, _ int64) string {
		return "http://localhost:8080/api/tracks/2/stream"
	})

	session, err := service.Next(1)
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}

	// Should have advanced to track 2
	if session.Current.TrackID != 2 {
		t.Errorf("Expected track 2, got %d", session.Current.TrackID)
	}

	// Device should have received play for the new track
	if !mpdDevice.playCalled {
		t.Error("Expected play to be called on MPD device for next track")
	}
}

// TDD: Seek on device forwards to the device
func TestService_Seek_WithDevice(t *testing.T) {
	mpdDevice := &mockPlaybackDevice{state: "play"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd-1": mpdDevice},
	}

	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 0},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	service := NewService(nil, sessionRepo)
	service.SetDeviceResolver(resolver)

	_, err := service.SeekTrack(1, 90)
	if err != nil {
		t.Fatalf("SeekTrack failed: %v", err)
	}

	if mpdDevice.seekPosition != 90 {
		t.Errorf("Expected seek position 90, got %d", mpdDevice.seekPosition)
	}
}

// TDD: SetVolume on device forwards to the device
func TestService_SetVolume_WithDevice(t *testing.T) {
	mpdDevice := &mockPlaybackDevice{state: "play"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd-1": mpdDevice},
	}

	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 0},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	service := NewService(nil, sessionRepo)
	service.SetDeviceResolver(resolver)

	_, err := service.SetVolume(1, 75)
	if err != nil {
		t.Fatalf("SetVolume failed: %v", err)
	}

	if mpdDevice.volume != 75 {
		t.Errorf("Expected volume 75, got %d", mpdDevice.volume)
	}
}

// TDD: Transfer reads elapsed position from old device
func TestService_TransferPlayback_ReadsPositionFromDevice(t *testing.T) {
	mpdDevice := &mockPlaybackDevice{state: "play", seekPosition: 75}
	browserDevice := &mockPlaybackDevice{}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{
			"mpd-1":     mpdDevice,
			"c-browser": browserDevice,
		},
	}

	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 0},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	service := NewService(nil, sessionRepo)
	service.SetDeviceResolver(resolver)
	service.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	session, err := service.TransferPlayback(1, "c-browser")
	if err != nil {
		t.Fatalf("TransferPlayback failed: %v", err)
	}

	// Position should come from the MPD device's elapsed (75), not the stale session (0)
	if session.Current.Position != 75 {
		t.Errorf("Expected position 75 from device, got %d", session.Current.Position)
	}

	// Old device should be stopped
	if !mpdDevice.stopCalled {
		t.Error("Expected old device to be stopped")
	}
}

// TDD: Transfer saves volume for old device and restores volume for new device
func TestService_TransferPlayback_SavesAndRestoresVolume(t *testing.T) {
	mpdDevice := &mockPlaybackDevice{state: "play", volume: 40, seekPosition: 10}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{
			"mpd-1":       mpdDevice,
			"c-browser-1": &BrowserPlaybackDevice{},
		},
	}

	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "c-browser-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 10},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	service := NewService(nil, sessionRepo)
	service.SetDeviceResolver(resolver)
	service.SetStreamURLBuilder(func(trackID, _ int64) string {
		return "http://localhost/stream"
	})

	// Transfer to MPD — should save browser volume, restore MPD volume (if any)
	session, err := service.TransferPlayback(1, "mpd-1")
	if err != nil {
		t.Fatalf("TransferPlayback to MPD failed: %v", err)
	}

	// Session should track per-device volumes
	if session.DeviceVolumes == nil {
		t.Fatal("Expected DeviceVolumes to be initialized")
	}
}

// TDD: Round-trip transfer Browser→MPD→Browser preserves per-device volumes
func TestService_TransferPlayback_RoundTrip_PerDeviceVolume(t *testing.T) {
	mpdDevice := &mockPlaybackDevice{state: "stop", volume: 100}
	browserDevice := &mockPlaybackDevice{volume: 80}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{
			"mpd-1":       mpdDevice,
			"c-browser-1": browserDevice,
		},
	}

	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "c-browser-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 0},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	service := NewService(nil, sessionRepo)
	service.SetDeviceResolver(resolver)
	service.SetStreamURLBuilder(func(trackID, _ int64) string {
		return "http://localhost/stream"
	})

	// Step 1: Set browser volume to 80
	service.SetVolume(1, 80)

	// Step 2: Transfer Browser → MPD
	session, _ := service.TransferPlayback(1, "mpd-1")

	// Browser volume (80) should be saved
	if vol, ok := session.DeviceVolumes["c-browser-1"]; !ok || vol != 80 {
		t.Errorf("Expected browser volume 80 saved, got %v (ok=%v)", vol, ok)
	}

	// Step 3: Set MPD volume to 40
	service.SetVolume(1, 40)
	if mpdDevice.volume != 40 {
		t.Errorf("Expected MPD device volume 40, got %d", mpdDevice.volume)
	}

	// Step 4: Transfer MPD → Browser
	session, _ = service.TransferPlayback(1, "c-browser-1")

	// MPD volume (40) should be saved
	if vol, ok := session.DeviceVolumes["mpd-1"]; !ok || vol != 40 {
		t.Errorf("Expected MPD volume 40 saved, got %v (ok=%v)", vol, ok)
	}
	// Browser volume (80) should be in the map still
	if vol, ok := session.DeviceVolumes["c-browser-1"]; !ok || vol != 80 {
		t.Errorf("Expected browser volume 80 still in map, got %v (ok=%v)", vol, ok)
	}

	// Step 5: Transfer Browser → MPD again — MPD should restore volume 40
	service.SetVolume(1, 80) // browser is at 80 again
	session, _ = service.TransferPlayback(1, "mpd-1")

	// MPD device should have volume restored to 40
	if mpdDevice.volume != 40 {
		t.Errorf("Expected MPD volume restored to 40, got %d", mpdDevice.volume)
	}
}

// TDD: Multi-hop transfer Browser→MPD→Browser preserves seek position each time
func TestService_TransferPlayback_MultiHop_PreservesPosition(t *testing.T) {
	mpdDevice := &mockPlaybackDevice{state: "stop", volume: 100}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{
			"mpd-1":       mpdDevice,
			"c-browser-1": &BrowserPlaybackDevice{},
		},
	}

	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "c-browser-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 30},
		Source:   Source{Type: SourceTypeAlbum, ID: 100, Remaining: []int64{6}},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	service := NewService(nil, sessionRepo)
	service.SetDeviceResolver(resolver)
	service.SetStreamURLBuilder(func(trackID, _ int64) string {
		return "http://localhost/stream"
	})

	// Transfer Browser → MPD at position 30
	session, _ := service.TransferPlayback(1, "mpd-1")
	if mpdDevice.playPosition != 30 {
		t.Errorf("Expected MPD play at position 30, got %d", mpdDevice.playPosition)
	}

	// Seek to 60 on MPD
	_, _ = service.SeekTrack(1, 60)
	mpdDevice.seekPosition = 60 // mock: device now reports 60

	// Transfer MPD → Browser
	session, _ = service.TransferPlayback(1, "c-browser-1")
	if session.Current.Position != 60 {
		t.Errorf("Expected position 60 from MPD, got %d", session.Current.Position)
	}

	// Seek to 90 on browser (position is stored in session)
	_, _ = service.SeekTrack(1, 90)

	// Transfer Browser → MPD again
	session, _ = service.TransferPlayback(1, "mpd-1")
	if mpdDevice.playPosition != 90 {
		t.Errorf("Expected MPD play at position 90, got %d", mpdDevice.playPosition)
	}
}

// TDD: SetVolume stores volume in session's DeviceVolumes
func TestService_SetVolume_StoresInDeviceVolumes(t *testing.T) {
	mpdDevice := &mockPlaybackDevice{state: "play"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd-1": mpdDevice},
	}

	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 0},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	service := NewService(nil, sessionRepo)
	service.SetDeviceResolver(resolver)

	session, err := service.SetVolume(1, 65)
	if err != nil {
		t.Fatalf("SetVolume failed: %v", err)
	}

	// Volume should be stored per-device
	if session.DeviceVolumes == nil {
		t.Fatal("Expected DeviceVolumes to be initialized")
	}
	if session.DeviceVolumes["mpd-1"] != 65 {
		t.Errorf("Expected volume 65 for mpd-1, got %d", session.DeviceVolumes["mpd-1"])
	}
}

// TDD: When the device rejects a Pause command, the service must return that
// error instead of silently persisting state=paused. Otherwise the UI shows
// "paused" while the device keeps playing — and the user's only feedback is a
// log line on the server.
func TestService_Pause_DeviceFailureReturnsError(t *testing.T) {
	failingDevice := &failingMockDevice{err: errors.New("mpd disconnected")}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd-1": failingDevice},
	}

	existing := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 0},
	}
	repo := &mockSessionRepository{session: existing}

	svc := NewService(nil, repo)
	svc.SetDeviceResolver(resolver)

	if _, err := svc.Pause(1, 30); err == nil {
		t.Error("expected Pause to return an error when device fails")
	}

	// State must NOT have been persisted as paused.
	saved, _ := repo.GetByUserID(1)
	if saved.State != StatePlaying {
		t.Errorf("expected session to remain Playing after device failure, got %s", saved.State)
	}
}

// failingMockDevice rejects every command with the configured error so we can
// verify that the service surfaces device failures.
type failingMockDevice struct{ err error }

func (f *failingMockDevice) Play(string, int) error { return f.err }
func (f *failingMockDevice) Pause() error           { return f.err }
func (f *failingMockDevice) Resume() error          { return f.err }
func (f *failingMockDevice) Stop() error            { return f.err }
func (f *failingMockDevice) Seek(int) error         { return f.err }
func (f *failingMockDevice) SetVolume(int) error    { return f.err }
func (f *failingMockDevice) SupportsVolume() bool   { return true }
func (f *failingMockDevice) Status() (DeviceStatus, error) {
	return DeviceStatus{State: "play"}, nil
}

// TDD: A persisted session whose DeviceID points at a now-defunct browser-tab
// clientID (e.g. after the tab refreshed and got a new clientID, or after the
// server restarted and lost the in-memory browser registry) must be deleted
// entirely. Under the unified-session invariant a session without an
// addressable device has no owner; returning it with DeviceID cleared would
// just resurrect the legacy "empty means here" semantics on the client.
// Deletion lets the caller create a fresh session bound to a live device.
func TestService_GetSession_DeletesOrphanedSession(t *testing.T) {
	resolver := &mockDeviceResolver{
		// No devices registered — old clientID won't resolve.
		devices: map[string]PlaybackDevice{},
	}

	persisted := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "c-old", // stale clientID from a previous WS connection
		Current:  &CurrentTrack{TrackID: 5, Position: 12},
	}
	repo := &mockSessionRepository{session: persisted}

	svc := NewService(nil, repo)
	svc.SetDeviceResolver(resolver)

	got, err := svc.GetSession(1)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil session (orphaned, deleted), got %+v", got)
	}
	if !repo.deleteCalled {
		t.Error("expected the orphaned session to be deleted from the repo")
	}
}

// TDD: A session whose DeviceID points to a known device (MPD or live tab)
// is returned unchanged.
func TestService_GetSession_KeepsLiveDeviceID(t *testing.T) {
	mpd := &mockPlaybackDevice{state: "play"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd-1": mpd},
	}

	persisted := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 12},
	}
	repo := &mockSessionRepository{session: persisted}

	svc := NewService(nil, repo)
	svc.SetDeviceResolver(resolver)

	got, err := svc.GetSession(1)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if got.DeviceID != "mpd-1" {
		t.Errorf("expected DeviceID preserved, got %q", got.DeviceID)
	}
}

// TDD: Playing a new album on an explicitly-named active device routes the
// new playback to that device — never silently jumping somewhere else. Under
// the unified-session invariant the service no longer "inherits" the
// previous session's device when none is specified; instead the frontend is
// expected to pass the current session's deviceId when the user hits play on
// the active device. This test exercises the explicit form.
func TestService_PlayAlbumOnDevice_RoutesToNamedDevice(t *testing.T) {
	mpdDevice := &mockPlaybackDevice{state: "play"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd-1": mpdDevice},
	}

	trackProvider := &mockTrackProvider{
		tracks: []Track{
			{ID: 10, Title: "Old Track", AlbumID: 100, Duration: 180},
			{ID: 20, Title: "New Track 1", AlbumID: 200, Duration: 180},
			{ID: 21, Title: "New Track 2", AlbumID: 200, Duration: 180},
		},
	}

	// User is currently playing album 100 on MPD.
	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 10, Position: 42},
		Source:   Source{Type: SourceTypeAlbum, ID: 100, Remaining: []int64{}},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	service := NewService(trackProvider, sessionRepo)
	service.SetDeviceResolver(resolver)
	service.SetStreamURLBuilder(func(trackID, _ int64) string {
		return "http://localhost/stream/" + string(rune(trackID))
	})

	// Frontend passes "mpd-1" explicitly (it knew about the current device).
	session, err := service.PlayAlbumOnDevice(1, 200, 0, "mpd-1")
	if err != nil {
		t.Fatalf("PlayAlbumOnDevice failed: %v", err)
	}

	if session.DeviceID != "mpd-1" {
		t.Errorf("Expected DeviceID 'mpd-1', got '%s'", session.DeviceID)
	}
	if !mpdDevice.playCalled {
		t.Error("Expected play to be forwarded to the MPD device")
	}
	if session.Current == nil || session.Current.TrackID != 20 {
		t.Errorf("Expected Current.TrackID = 20, got %v", session.Current)
	}
}

// TDD: Transfer playback to a different device
func TestService_TransferPlayback(t *testing.T) {
	// Use the real BrowserPlaybackDevice for the browser slot — its Status()
	// returns an error, which matches production semantics and prevents the
	// transfer from clobbering session.Current.Position with the mock's
	// default elapsed=0. Tests that need to inspect browser-side calls use
	// a mockPlaybackDevice with an injected statusErr instead.
	mpdDevice := &mockPlaybackDevice{}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{
			"c-browser-1": &BrowserPlaybackDevice{},
			"mpd-1":       mpdDevice,
		},
	}

	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "c-browser-1", // currently on browser
		Current:  &CurrentTrack{TrackID: 5, Position: 30},
		Source:   Source{Type: SourceTypeAlbum, ID: 100, Remaining: []int64{6}},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	service := NewService(nil, sessionRepo)
	service.SetDeviceResolver(resolver)
	service.SetStreamURLBuilder(func(trackID, _ int64) string {
		return "http://localhost:8080/api/tracks/5/stream"
	})

	session, err := service.TransferPlayback(1, "mpd-1")
	if err != nil {
		t.Fatalf("TransferPlayback failed: %v", err)
	}

	// Session should now point to MPD device
	if session.DeviceID != "mpd-1" {
		t.Errorf("Expected DeviceID 'mpd-1', got '%s'", session.DeviceID)
	}

	// MPD device should have received play at the current position
	if !mpdDevice.playCalled {
		t.Error("Expected play to be called on MPD device")
	}
	if mpdDevice.playPosition != 30 {
		t.Errorf("Expected position 30, got %d", mpdDevice.playPosition)
	}
}

// A mixerless MPD device (e.g. HiFiBerry default with `mixer_type "none"`)
// returns ErrVolumeNotSupported on SetVolume. The user-facing API must still
// succeed: the user moved a slider, we promised to remember the value, and
// transfers to a future device that DOES have volume should honor it.
func TestService_SetVolume_ToleratesVolumeNotSupported(t *testing.T) {
	mpdDevice := &mockPlaybackDevice{
		state:        "play",
		setVolumeErr: ErrVolumeNotSupported,
	}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd-1": mpdDevice},
	}
	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 30},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	svc := NewService(nil, sessionRepo)
	svc.SetDeviceResolver(resolver)

	session, err := svc.SetVolume(1, 75)
	if err != nil {
		t.Fatalf("SetVolume should tolerate ErrVolumeNotSupported, got: %v", err)
	}
	if session.DeviceVolumes["mpd-1"] != 75 {
		t.Errorf("expected DeviceVolumes[mpd-1]=75, got %d", session.DeviceVolumes["mpd-1"])
	}
}

// A wrapped ErrVolumeNotSupported (as produced by MPDPlaybackDevice via
// fmt.Errorf("%w: ...")) must still match via errors.Is so tolerance kicks in.
func TestService_SetVolume_ToleratesWrappedVolumeNotSupported(t *testing.T) {
	wrapped := errors.New("setvol: ack: " + ErrVolumeNotSupported.Error())
	// Build a wrapper that errors.Is reports as ErrVolumeNotSupported.
	wrapped = wrappedSentinelError{inner: wrapped}

	mpdDevice := &mockPlaybackDevice{
		state:        "play",
		setVolumeErr: wrapped,
	}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd-1": mpdDevice},
	}
	sessionRepo := &mockSessionRepository{session: &Session{
		UserID: 1, State: StatePlaying, DeviceID: "mpd-1",
		Current: &CurrentTrack{TrackID: 5, Position: 30},
	}}

	svc := NewService(nil, sessionRepo)
	svc.SetDeviceResolver(resolver)

	if _, err := svc.SetVolume(1, 60); err != nil {
		t.Fatalf("expected success on wrapped sentinel, got: %v", err)
	}
}

// wrappedSentinelError mirrors the wrapping MPDPlaybackDevice does
// (fmt.Errorf("%w: ...", ErrVolumeNotSupported)).
type wrappedSentinelError struct{ inner error }

func (e wrappedSentinelError) Error() string { return e.inner.Error() }
func (e wrappedSentinelError) Unwrap() error { return ErrVolumeNotSupported }

// Transfer to a device whose SetVolume rejects with ErrVolumeNotSupported
// must still succeed end-to-end. Volume restore is best-effort; failing it
// must not block the user from moving playback to that device.
func TestService_TransferPlayback_RestoreVolumeNotSupportedDoesNotError(t *testing.T) {
	browserDevice := &mockPlaybackDevice{state: "play"}
	mpdDevice := &mockPlaybackDevice{
		setVolumeErr: ErrVolumeNotSupported,
	}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{
			"c-browser-1": browserDevice,
			"mpd-1":       mpdDevice,
		},
	}
	existingSession := &Session{
		UserID:        1,
		State:         StatePlaying,
		DeviceID:      "c-browser-1",
		Current:       &CurrentTrack{TrackID: 5, Position: 30},
		Source:        Source{Type: SourceTypeAlbum, ID: 100, Remaining: []int64{6}},
		DeviceVolumes: map[string]int{"mpd-1": 65}, // saved volume to restore
	}
	sessionRepo := &mockSessionRepository{session: existingSession}

	svc := NewService(nil, sessionRepo)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(trackID, _ int64) string {
		return "http://localhost:8080/api/tracks/5/stream"
	})

	session, err := svc.TransferPlayback(1, "mpd-1")
	if err != nil {
		t.Fatalf("Transfer should succeed despite mixerless device, got: %v", err)
	}
	if session.DeviceID != "mpd-1" {
		t.Errorf("Expected transfer to mpd-1, got DeviceID=%q", session.DeviceID)
	}
	if !mpdDevice.playCalled {
		t.Error("expected play on target despite volume restore failure")
	}
}

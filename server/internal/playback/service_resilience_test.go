package playback

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// copyingSessionRepository mimics the DB repository's value semantics: Get
// hands out a copy, Save stores a copy. The pointer-sharing mock repo hides
// lost updates because every caller mutates the same struct.
type copyingSessionRepository struct {
	mu           sync.Mutex
	session      *Session
	deleteCalled bool
}

func (r *copyingSessionRepository) Save(session *Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.session = cloneSession(session)
	return nil
}

func (r *copyingSessionRepository) GetByUserID(int64) (*Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneSession(r.session), nil
}

func (r *copyingSessionRepository) Delete(int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleteCalled = true
	r.session = nil
	return nil
}

func (r *copyingSessionRepository) DeleteWithoutDevice() (int64, error) { return 0, nil }

func cloneSession(s *Session) *Session {
	if s == nil {
		return nil
	}
	out := *s
	if s.Current != nil {
		cur := *s.Current
		out.Current = &cur
	}
	out.Queue = append([]QueueItem(nil), s.Queue...)
	out.History = append([]int64(nil), s.History...)
	out.Source.Remaining = append([]int64(nil), s.Source.Remaining...)
	if s.DeviceVolumes != nil {
		out.DeviceVolumes = make(map[string]int, len(s.DeviceVolumes))
		for k, v := range s.DeviceVolumes {
			out.DeviceVolumes[k] = v
		}
	}
	return &out
}

// blockingVolumeDevice parks inside SetVolume until released, so a test can
// interleave another writer while the volume round trip is in flight.
type blockingVolumeDevice struct {
	mockPlaybackDevice
	entered chan struct{}
	release chan struct{}
}

func (d *blockingVolumeDevice) SetVolume(volume int) error {
	close(d.entered)
	<-d.release
	d.volume = volume
	return nil
}

// P0-1: an MPD box that rebooted is unreachable, not gone. Destroying the
// session on a refused dial loses the user's queue, position and history for
// good — the next page load is enough to trigger it.
func TestService_GetSession_KeepsSessionWhenDeviceUnreachable(t *testing.T) {
	resolver := &mockDeviceResolver{
		errs: map[string]error{"mpd-1": ErrDeviceUnreachable},
	}
	persisted := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 12},
		History:  []int64{1, 2, 3},
	}
	repo := &mockSessionRepository{session: persisted}

	svc := NewService(nil, repo)
	svc.SetDeviceResolver(resolver)

	got, err := svc.GetSession(1)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if repo.deleteCalled {
		t.Error("a transiently unreachable device must not delete the session")
	}
	if got == nil {
		t.Fatal("expected the session to survive a transient device error")
	}
	if len(got.History) != 3 || got.Current.Position != 12 {
		t.Errorf("session state was not preserved: %+v", got)
	}
}

// P1-10: deleting an orphaned session must also drop its poller entry,
// otherwise a vanished device keeps a 1 Hz resolve+Status running forever.
func TestService_GetSession_OrphanDeleteClearsPollerEntry(t *testing.T) {
	device := &mockPlaybackDevice{state: "play"}
	resolver := &mockDeviceResolver{devices: map[string]PlaybackDevice{"mpd-1": device}}
	repo := &mockSessionRepository{}

	svc := NewService(&mockTrackProvider{tracks: []Track{{ID: 1, AlbumID: 100}}}, repo)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "mpd-1"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}
	if _, ok := svc.mpdUsers.get(1); !ok {
		t.Fatal("precondition: expected a poller entry after play")
	}

	// The device disappears from the registry entirely.
	delete(resolver.devices, "mpd-1")

	if _, err := svc.GetSession(1); err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if _, ok := svc.mpdUsers.get(1); ok {
		t.Error("orphan cleanup left a poller entry behind; the poller will keep resolving a dead device")
	}
}

// P1-10: browser sessions are pushed from the tab over WS, never polled. A
// poller entry for one costs a resolver round trip per second to learn nothing.
func TestService_TrackMPDState_SkipsBrowserDevices(t *testing.T) {
	resolver := &mockDeviceResolver{
		devices:  map[string]PlaybackDevice{"c-tab-1": &BrowserPlaybackDevice{}},
		browsers: map[string]bool{"c-tab-1": true},
	}
	repo := &mockSessionRepository{}

	svc := NewService(&mockTrackProvider{tracks: []Track{{ID: 1, AlbumID: 100}}}, repo)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "c-tab-1"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}

	if _, ok := svc.mpdUsers.get(1); ok {
		t.Error("a browser-tab session must not get an MPD poller entry")
	}
}

// P0-2: when the auto-advance play fails, Next used to return before
// persisting anything. The session stayed on the old track with state=playing
// while the poller had already cleared observedPlay — every later tick then
// read "stop without prior play" and refused to advance. Playback wedged with
// only a Debug log to show for it.
func TestService_Next_FailedDeviceAdvanceLeavesRecoverableState(t *testing.T) {
	device := &mockPlaybackDevice{state: "play"}
	resolver := &mockDeviceResolver{devices: map[string]PlaybackDevice{"mpd-1": device}}

	existing := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 1, Position: 100},
		Source:   Source{Type: SourceTypeAlbum, ID: 100, Remaining: []int64{2, 3}},
		Queue:    []QueueItem{},
		History:  []int64{},
	}
	repo := &mockSessionRepository{session: existing}

	svc := NewService(nil, repo)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	device.playErr = errors.New("mpd: connection refused")

	if _, err := svc.Next(1); err == nil {
		t.Fatal("expected Next to surface the device failure")
	}

	saved, _ := repo.GetByUserID(1)
	if saved == nil {
		t.Fatal("expected the session to be persisted despite the device failure")
	}
	if saved.State == StatePlaying {
		t.Errorf("session still claims to be playing after a failed advance: %+v", saved)
	}
	if saved.Current == nil || saved.Current.TrackID != 2 {
		t.Errorf("expected the session to hold the track we tried to advance to, got %+v", saved.Current)
	}
	if _, ok := svc.mpdUsers.get(1); ok {
		t.Error("a non-playing session must not keep a poller entry")
	}
}

// P0-3: pressing play on a second device left the first one playing, and
// because the session no longer names it nothing can ever stop it again.
func TestService_PlayAlbumOnDevice_StopsPreviousDevice(t *testing.T) {
	oldDevice := &mockPlaybackDevice{state: "play"}
	newDevice := &mockPlaybackDevice{state: "stop"}
	resolver := &mockDeviceResolver{devices: map[string]PlaybackDevice{
		"mpd-1": oldDevice,
		"mpd-2": newDevice,
	}}

	tracks := &mockTrackProvider{tracks: []Track{
		{ID: 10, AlbumID: 100},
		{ID: 20, AlbumID: 200},
	}}
	existing := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 10, Position: 42},
		Source:   Source{Type: SourceTypeAlbum, ID: 100},
	}
	repo := &mockSessionRepository{session: existing}

	svc := NewService(tracks, repo)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	session, err := svc.PlayAlbumOnDevice(1, 200, 0, "mpd-2")
	if err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}
	if session.DeviceID != "mpd-2" {
		t.Fatalf("expected the session to move to mpd-2, got %q", session.DeviceID)
	}
	if !newDevice.playCalled {
		t.Error("expected play on the new device")
	}
	if !oldDevice.stopCalled {
		t.Error("the previously playing device was never stopped — two devices now play at once")
	}
}

// P0-3: playing again on the SAME device must not stop it first; that would
// interrupt the very device we're about to start.
func TestService_PlayAlbumOnDevice_SameDeviceIsNotStopped(t *testing.T) {
	device := &mockPlaybackDevice{state: "play"}
	resolver := &mockDeviceResolver{devices: map[string]PlaybackDevice{"mpd-1": device}}

	tracks := &mockTrackProvider{tracks: []Track{
		{ID: 10, AlbumID: 100},
		{ID: 20, AlbumID: 200},
	}}
	repo := &mockSessionRepository{session: &Session{
		UserID: 1, State: StatePlaying, DeviceID: "mpd-1",
		Current: &CurrentTrack{TrackID: 10, Position: 42},
	}}

	svc := NewService(tracks, repo)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 200, 0, "mpd-1"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}
	if device.stopCalled {
		t.Error("playing on the current device must not stop it")
	}
}

// P1-4: SetVolume reads the session, then blocks on the MPD round trip. A
// position update landing in that window used to read the pre-volume session
// and save it afterwards — the volume was lost AND the stale value broadcast
// back, so the slider snapped to its old spot. The same shape drops a pause
// or a seek.
func TestService_SetVolume_SurvivesConcurrentPositionUpdate(t *testing.T) {
	device := &blockingVolumeDevice{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	device.state = "play"
	resolver := &mockDeviceResolver{devices: map[string]PlaybackDevice{"mpd-1": device}}

	repo := &copyingSessionRepository{session: &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 0},
	}}

	svc := NewService(nil, repo)
	svc.SetDeviceResolver(resolver)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := svc.SetVolume(1, 55); err != nil {
			t.Errorf("SetVolume: %v", err)
		}
	}()

	<-device.entered // SetVolume has read the session and is inside device I/O

	wg.Add(1)
	go func() {
		defer wg.Done()
		svc.UpdatePosition(1, 10)
	}()

	// Give the racing writer time to complete its full read-modify-write.
	time.Sleep(50 * time.Millisecond)
	close(device.release)
	wg.Wait()

	saved, _ := repo.GetByUserID(1)
	if saved.DeviceVolumes["mpd-1"] != 55 {
		t.Errorf("volume lost to a concurrent position update: got %v", saved.DeviceVolumes)
	}
	if saved.Current.Position != 10 {
		t.Errorf("position lost to a concurrent volume change: got %d", saved.Current.Position)
	}
}

// P2-11: with state != playing the play branch is skipped entirely, so the
// target device ID was persisted without anyone ever checking it exists.
// Transferring a paused session to an offline box reported success.
func TestService_TransferPlayback_PausedValidatesTarget(t *testing.T) {
	oldDevice := &mockPlaybackDevice{state: "pause"}
	resolver := &mockDeviceResolver{devices: map[string]PlaybackDevice{"mpd-1": oldDevice}}

	repo := &mockSessionRepository{session: &Session{
		UserID:   1,
		State:    StatePaused,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 30},
	}}

	svc := NewService(nil, repo)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	if _, err := svc.TransferPlayback(1, "mpd-offline"); err == nil {
		t.Fatal("expected transfer to an unknown device to fail")
	}

	saved, _ := repo.GetByUserID(1)
	if saved.DeviceID != "mpd-1" {
		t.Errorf("session was bound to an unvalidated device: %q", saved.DeviceID)
	}
}

// P2-11: when the play on the target fails, the rollback restored DeviceID but
// left the old device stopped and the poller entry deleted — playback was dead
// on both devices with the session claiming to play.
func TestService_TransferPlayback_RollbackRestartsOldDevice(t *testing.T) {
	oldDevice := &mockPlaybackDevice{state: "play", songID: "1"}
	target := &mockPlaybackDevice{playErr: errors.New("mpd: connection refused")}
	resolver := &mockDeviceResolver{devices: map[string]PlaybackDevice{
		"mpd-1":   oldDevice,
		"mpd-bad": target,
	}}

	repo := &mockSessionRepository{}
	svc := NewService(&mockTrackProvider{tracks: []Track{
		{ID: 1, AlbumID: 100}, {ID: 2, AlbumID: 100},
	}}, repo)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "mpd-1"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}
	oldDevice.playCalled = false

	if _, err := svc.TransferPlayback(1, "mpd-bad"); err == nil {
		t.Fatal("expected transfer to a failing device to error")
	}

	saved, _ := repo.GetByUserID(1)
	if saved.DeviceID != "mpd-1" {
		t.Errorf("DeviceID not rolled back: %q", saved.DeviceID)
	}
	if !oldDevice.playCalled {
		t.Error("old device was stopped for the transfer and never restarted")
	}
	if _, ok := svc.mpdUsers.get(1); !ok {
		t.Error("poller entry deleted for the transfer was never restored")
	}
}

// P2-12: Previous had no playing-state guard (Next does). Pressing it while
// paused started MPD while the session stayed paused — and the poller refuses
// to track a non-playing session, so the position froze.
func TestService_Previous_WhilePausedDoesNotStartDevice(t *testing.T) {
	device := &mockPlaybackDevice{state: "pause"}
	resolver := &mockDeviceResolver{devices: map[string]PlaybackDevice{"mpd-1": device}}

	repo := &mockSessionRepository{session: &Session{
		UserID:   1,
		State:    StatePaused,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 2, Position: 50},
		Source:   Source{Type: SourceTypeAlbum, ID: 100, Remaining: []int64{3}},
		History:  []int64{1},
	}}

	svc := NewService(nil, repo)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	session, err := svc.Previous(1)
	if err != nil {
		t.Fatalf("Previous: %v", err)
	}
	if device.playCalled {
		t.Error("Previous started the device on a paused session")
	}
	if session.State != StatePaused {
		t.Errorf("expected the session to stay paused, got %s", session.State)
	}
	if session.Current.TrackID != 1 {
		t.Errorf("expected to step back to track 1, got %d", session.Current.TrackID)
	}
}

// P2-13: running off the end of the source set state=stopped but never told
// the device, so MPD kept playing the last track's stream.
func TestService_Next_EndOfSourceStopsDevice(t *testing.T) {
	device := &mockPlaybackDevice{state: "play"}
	resolver := &mockDeviceResolver{devices: map[string]PlaybackDevice{"mpd-1": device}}

	repo := &mockSessionRepository{session: &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 3, Position: 100},
		Source:   Source{Type: SourceTypeAlbum, ID: 100, Remaining: []int64{}},
		Queue:    []QueueItem{},
		History:  []int64{1, 2},
	}}

	svc := NewService(nil, repo)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	session, err := svc.Next(1)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if session.State != StateStopped {
		t.Errorf("expected stopped, got %s", session.State)
	}
	if !device.stopCalled {
		t.Error("end of source left the device playing")
	}
}

// P1-5: TransferPlayback deletes the poller entry on purpose, so the poller
// can't fire Next mid-transfer. A poller store that was already in flight must
// not bring the entry back.
func TestMPDTracker_UpdateCannotResurrectDeletedEntry(t *testing.T) {
	var tracker mpdTracker

	tracker.set(1, activeMPDSession{deviceID: "mpd-1", expectedTrack: 5})
	stale, _ := tracker.get(1)
	tracker.delete(1)

	stale.observedPlay = true
	tracker.updateIfPresent(1, stale)

	if _, ok := tracker.get(1); ok {
		t.Error("a deleted entry was resurrected by an in-flight store")
	}
}

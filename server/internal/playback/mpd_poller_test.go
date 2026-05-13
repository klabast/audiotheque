package playback

import (
	"errors"
	"testing"
)

var errStatusBoom = errors.New("status: boom")

func TestService_PollMPDPositions_UpdatesAndBroadcasts(t *testing.T) {
	tracks := &mockTrackProvider{
		tracks: []Track{{ID: 1, AlbumID: 100, Duration: 180}},
	}
	repo := &mockSessionRepository{}
	bc := &recordingBroadcaster{}

	mpd := &mockPlaybackDevice{seekPosition: 42, state: "play"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd1": mpd},
	}

	svc := NewService(tracks, repo)
	svc.SetBroadcaster(bc)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(id, _ int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "mpd1"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}
	preCount := len(bc.calls)

	svc.PollMPDPositions()

	if len(bc.calls) == preCount {
		t.Fatal("expected poll to broadcast, got no new broadcasts")
	}
	sess, _ := repo.GetByUserID(1)
	if sess.Current.Position != 42 {
		t.Errorf("position = %d, want 42", sess.Current.Position)
	}
}

func TestService_PollMPDPositions_SkipsBrowserSessions(t *testing.T) {
	tracks := &mockTrackProvider{
		tracks: []Track{{ID: 1, AlbumID: 100, Duration: 180}},
	}
	repo := &mockSessionRepository{}
	bc := &recordingBroadcaster{}
	// Resolver maps the browser client ID to the real BrowserPlaybackDevice;
	// its Status() returns an error so the poller logs and moves on without
	// broadcasting.
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"c-browser-1": &BrowserPlaybackDevice{}},
	}
	svc := NewService(tracks, repo)
	svc.SetBroadcaster(bc)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "c-browser-1"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}
	preCount := len(bc.calls)

	svc.PollMPDPositions()

	if len(bc.calls) != preCount {
		t.Errorf("browser session should not be polled; got %d new broadcasts", len(bc.calls)-preCount)
	}
}

// When MPD reports state=stop after a track has actually played (state=play
// observed at least once for the current songid), the poller must advance
// the session to the next track. This is the legitimate auto-advance path.
//
// The previous version of this test fed a single state=stop poll and asserted
// auto-advance. That tested the buggy unconditional Next() behavior. The
// honest version requires the poller to first observe state=play (mirroring
// what real MPD does) before treating a subsequent stop as track-end.
func TestService_PollMPDPositions_AdvancesOnTrackEnd(t *testing.T) {
	tracks := &mockTrackProvider{
		tracks: []Track{
			{ID: 1, AlbumID: 100, Duration: 180},
			{ID: 2, AlbumID: 100, Duration: 200},
		},
	}
	repo := &mockSessionRepository{}
	bc := &recordingBroadcaster{}

	mpd := &mockPlaybackDevice{state: "play", seekPosition: 100, songID: "1"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd1": mpd},
	}

	svc := NewService(tracks, repo)
	svc.SetBroadcaster(bc)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(id, _ int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "mpd1"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}

	// First poll: MPD is genuinely playing the loaded track. observedPlay
	// gets latched so a subsequent stop is recognized as track-end.
	svc.PollMPDPositions()

	// MPD finishes the track: state flips to stop, elapsed resets to 0.
	mpd.state = "stop"
	mpd.seekPosition = 0
	mpd.playCalled = false // reset so we can detect the next play

	svc.PollMPDPositions()

	sess, _ := repo.GetByUserID(1)

	// Saved position must NOT be clobbered to 0.
	if sess.Current == nil {
		t.Fatal("expected a current track after auto-advance")
	}
	if sess.Current.TrackID != 2 {
		t.Errorf("expected current track 2 after auto-advance, got %d", sess.Current.TrackID)
	}
	// Next track should have been pushed to MPD.
	if !mpd.playCalled {
		t.Error("expected MPD to receive play for the next track")
	}
}

// REGRESSION: this is the live audio-wz bug. After PlayAlbumOnDevice MPD spends
// a tick or two in state=stop while the URL loads (HTTP fetch, decoder spin-up).
// The old poller treated this as track-end and called Next(), which loaded the
// next track, which itself reported stop transiently → infinite Next() loop
// blasting through the album.
//
// Fix invariant: state=stop without ever observing state=play for the current
// MPD song = device never started → never auto-advance. Position is preserved.
func TestService_PollMPDPositions_DoesNotAdvanceWhenStopBeforePlay(t *testing.T) {
	tracks := &mockTrackProvider{
		tracks: []Track{
			{ID: 1, AlbumID: 100, Duration: 180},
			{ID: 2, AlbumID: 100, Duration: 200},
			{ID: 3, AlbumID: 100, Duration: 220},
		},
	}
	repo := &mockSessionRepository{}
	bc := &recordingBroadcaster{}

	// MPD reports stop on every poll — simulates a device that accepted Play
	// but is stuck loading the URL (or refusing to decode the stream).
	mpd := &mockPlaybackDevice{state: "stop", seekPosition: 0, songID: ""}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd1": mpd},
	}

	svc := NewService(tracks, repo)
	svc.SetBroadcaster(bc)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(id, _ int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "mpd1"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}
	if !mpd.playCalled {
		t.Fatalf("expected initial Play to have been called")
	}
	mpd.playCalled = false

	// Poll multiple times — the buggy behavior would call Next() each tick,
	// reloading a different track on every poll until the album drained.
	for i := 0; i < 5; i++ {
		svc.PollMPDPositions()
	}

	sess, _ := repo.GetByUserID(1)
	if sess.Current == nil || sess.Current.TrackID != 1 {
		t.Errorf("expected Current.TrackID=1 (no advance), got %+v", sess.Current)
	}
	if mpd.playCalled {
		t.Error("expected Play to NOT be called from the poller while state=stop without prior play")
	}
	if len(sess.Source.Remaining) != 2 {
		t.Errorf("expected Source.Remaining unchanged (2 tracks), got %d", len(sess.Source.Remaining))
	}
}

// After a real auto-advance, the MPD songid increments (Clear+Add semantics).
// The new song must reset observedPlay so a subsequent stop on THIS song
// can't be mistaken for a track-end if MPD lingers in stop while loading the
// new URL.
func TestService_PollMPDPositions_NewSongIDResetsObservedPlay(t *testing.T) {
	tracks := &mockTrackProvider{
		tracks: []Track{
			{ID: 1, AlbumID: 100, Duration: 180},
			{ID: 2, AlbumID: 100, Duration: 200},
			{ID: 3, AlbumID: 100, Duration: 220},
		},
	}
	repo := &mockSessionRepository{}
	bc := &recordingBroadcaster{}

	mpd := &mockPlaybackDevice{state: "play", seekPosition: 50, songID: "1"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd1": mpd},
	}

	svc := NewService(tracks, repo)
	svc.SetBroadcaster(bc)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(id, _ int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "mpd1"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}

	// First track plays then ends → expected one auto-advance.
	svc.PollMPDPositions()                                  // observedPlay latched for songid=1
	mpd.state, mpd.seekPosition, mpd.songID = "stop", 0, "" // track ends on MPD
	svc.PollMPDPositions()                                  // → Next() to track 2

	if sess, _ := repo.GetByUserID(1); sess.Current.TrackID != 2 {
		t.Fatalf("expected first auto-advance to track 2, got %d", sess.Current.TrackID)
	}

	// Now MPD has the new track loaded. New songid, but during URL-load it
	// reports stop transiently. The poller MUST NOT fire a second advance.
	mpd.songID = "2"
	mpd.state = "stop"
	mpd.seekPosition = 0
	mpd.playCalled = false

	for i := 0; i < 5; i++ {
		svc.PollMPDPositions()
	}

	sess, _ := repo.GetByUserID(1)
	if sess.Current.TrackID != 2 {
		t.Errorf("expected NO further advance (stay on track 2), got TrackID=%d", sess.Current.TrackID)
	}
}

// Edge case: MPD's Clear+Add+Play sequence after our own Next() can briefly
// report state=stop with a stale songid (the just-cleared one) before MPD
// loads the new song's metadata. observedPlay was true (we saw the previous
// track playing). Without a pre-clear, the poller would interpret this as a
// second track-end and double-skip.
func TestService_PollMPDPositions_DoesNotDoubleFireOnSlowSongLoad(t *testing.T) {
	tracks := &mockTrackProvider{
		tracks: []Track{
			{ID: 1, AlbumID: 100, Duration: 180},
			{ID: 2, AlbumID: 100, Duration: 200},
			{ID: 3, AlbumID: 100, Duration: 220},
		},
	}
	repo := &mockSessionRepository{}
	bc := &recordingBroadcaster{}

	mpd := &mockPlaybackDevice{state: "play", seekPosition: 30, songID: "1"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd1": mpd},
	}

	svc := NewService(tracks, repo)
	svc.SetBroadcaster(bc)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(id, _ int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "mpd1"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}

	// Track 1 plays, then ends.
	svc.PollMPDPositions()
	mpd.state, mpd.seekPosition = "stop", 0
	svc.PollMPDPositions() // → Next() to track 2

	if sess, _ := repo.GetByUserID(1); sess.Current.TrackID != 2 {
		t.Fatalf("expected advance to track 2, got %d", sess.Current.TrackID)
	}

	// Right after Clear+Add+Play, MPD is still in stop and the songid is
	// briefly stale (the just-cleared 1) for one tick. Old poller would
	// fire Next() again here.
	mpd.state = "stop"
	mpd.seekPosition = 0
	mpd.songID = "1" // stale — Add hasn't completed yet
	svc.PollMPDPositions()

	sess, _ := repo.GetByUserID(1)
	if sess.Current.TrackID != 2 {
		t.Errorf("double-skip detected: expected TrackID=2, got %d", sess.Current.TrackID)
	}
}

// Pre-play stop ticks (the audio-wz blast-loop trigger) must not just
// avoid Next(); they must also not touch position. Verified by holding a
// non-zero starting position via PlayAlbumOnDevice with startPosition>0
// and asserting it survives multiple stop polls.
func TestService_PollMPDPositions_PreservesPositionOnPrePlayStop(t *testing.T) {
	tracks := &mockTrackProvider{
		tracks: []Track{
			{ID: 1, AlbumID: 100, Duration: 180},
			{ID: 2, AlbumID: 100, Duration: 200},
		},
	}
	repo := &mockSessionRepository{}
	bc := &recordingBroadcaster{}

	mpd := &mockPlaybackDevice{state: "stop", seekPosition: 0, songID: ""}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd1": mpd},
	}

	svc := NewService(tracks, repo)
	svc.SetBroadcaster(bc)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(id, _ int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "mpd1"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}
	// Simulate user/handoff having seeked to position 73 mid-track.
	if _, err := svc.SeekTrack(1, 73); err != nil {
		t.Fatalf("SeekTrack: %v", err)
	}

	for i := 0; i < 5; i++ {
		svc.PollMPDPositions()
	}

	sess, _ := repo.GetByUserID(1)
	if sess.Current == nil {
		t.Fatal("expected current track to be preserved")
	}
	if sess.Current.Position != 73 {
		t.Errorf("position regressed: expected 73, got %d", sess.Current.Position)
	}
}

// Status() errors (network glitch, MPD restart, etc.) must not mutate the
// session at all. Position, state, current track all preserved.
func TestService_PollMPDPositions_PreservesSessionOnStatusError(t *testing.T) {
	tracks := &mockTrackProvider{
		tracks: []Track{
			{ID: 1, AlbumID: 100, Duration: 180},
			{ID: 2, AlbumID: 100, Duration: 200},
		},
	}
	repo := &mockSessionRepository{}
	bc := &recordingBroadcaster{}

	mpd := &mockPlaybackDevice{state: "play", seekPosition: 75, songID: "1"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd1": mpd},
	}

	svc := NewService(tracks, repo)
	svc.SetBroadcaster(bc)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(id, _ int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "mpd1"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}
	svc.PollMPDPositions()

	// Now MPD's Status starts erroring (e.g., connection dropped).
	mpd.statusErr = errStatusBoom
	for i := 0; i < 3; i++ {
		svc.PollMPDPositions()
	}

	sess, _ := repo.GetByUserID(1)
	if sess.Current == nil || sess.Current.TrackID != 1 {
		t.Errorf("expected current track unchanged, got %+v", sess.Current)
	}
	if sess.Current.Position == 0 {
		t.Errorf("expected position preserved on status error")
	}
}

func TestService_PollMPDPositions_SkipsPausedSessions(t *testing.T) {
	tracks := &mockTrackProvider{
		tracks: []Track{{ID: 1, AlbumID: 100, Duration: 180}},
	}
	repo := &mockSessionRepository{}
	bc := &recordingBroadcaster{}

	mpd := &mockPlaybackDevice{seekPosition: 99, state: "pause"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"mpd1": mpd},
	}

	svc := NewService(tracks, repo)
	svc.SetBroadcaster(bc)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(id, _ int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "mpd1"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}
	if _, err := svc.Pause(1, 10); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	preCount := len(bc.calls)

	svc.PollMPDPositions()

	if len(bc.calls) != preCount {
		t.Errorf("paused session should not be polled; got %d new broadcasts", len(bc.calls)-preCount)
	}
	sess, _ := repo.GetByUserID(1)
	if sess.Current.Position == 99 {
		t.Error("paused session position should not have been overwritten by poll")
	}
}

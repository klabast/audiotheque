package playback

import (
	"log/slog"
	"testing"

	"audiod/internal/mpd/testserver"
)

// Integration test: wire Service + RegistryDeviceResolver + a real
// mpd/testserver instance and verify the production code path absorbs the
// audio-wz transient-stop loop. Drives the testserver's state directly via
// SetState so we don't depend on real timing windows.
//
// This is the closest unit-of-test we can put under the live bug without
// running an actual MPD daemon. If this test fails, it's the same bug
// shipping to TrueNAS.
func TestIntegration_Poller_TransientStopAfterPlayDoesNotAdvance(t *testing.T) {
	srv := testserver.New()
	defer srv.Close()

	// Production resolver wiring (same as cmd/server/commands/server.go).
	registry := NewInMemoryDeviceRegistry()
	if err := registry.Register(Device{
		ID:      "audio-wz",
		Name:    "Wohnzimmer",
		Type:    DeviceTypeMPD,
		Address: srv.Addr(),
		UserID:  1,
	}); err != nil {
		t.Fatalf("registry.Register: %v", err)
	}
	resolver := NewRegistryDeviceResolver(registry, slog.Default())

	// Service wiring.
	tracks := &mockTrackProvider{
		tracks: []Track{
			{ID: 1, AlbumID: 100, Duration: 180},
			{ID: 2, AlbumID: 100, Duration: 200},
			{ID: 3, AlbumID: 100, Duration: 220},
		},
	}
	repo := &mockSessionRepository{}
	bc := &recordingBroadcaster{}

	svc := NewService(tracks, repo)
	svc.SetBroadcaster(bc)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(id, _ int64) string {
		return "http://audiod.local/stream"
	})

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "audio-wz"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}

	// Simulate the live bug: real MPD returned state=play after our cmdPlay,
	// but the audio output is still loading the URL — testserver flips back
	// to stop. The poller MUST absorb this without firing Next.
	srv.SetState("stop")
	srv.SetElapsed(0)

	for i := 0; i < 5; i++ {
		svc.PollMPDPositions()
	}

	sess, _ := repo.GetByUserID(1)
	if sess.Current == nil || sess.Current.TrackID != 1 {
		t.Errorf("expected to stay on track 1, got %+v", sess.Current)
	}
	if len(sess.Source.Remaining) != 2 {
		t.Errorf("expected Source.Remaining=2 (no auto-advance), got %d",
			len(sess.Source.Remaining))
	}
	// MPD's playlist should still have exactly the original track loaded —
	// not a different one swapped in by a runaway Next() loop.
	state := srv.State()
	if len(state.Playlist) != 1 {
		t.Errorf("expected MPD playlist to have 1 entry, got %d (%v)",
			len(state.Playlist), state.Playlist)
	}
}

// Integration test: a track that genuinely plays then stops on MPD must
// auto-advance exactly once. Validates the poller's full state-machine end
// to end against a real MPD-protocol mock.
func TestIntegration_Poller_RealTrackEndAdvancesOnce(t *testing.T) {
	srv := testserver.New()
	defer srv.Close()

	registry := NewInMemoryDeviceRegistry()
	if err := registry.Register(Device{
		ID:      "mpd-x",
		Name:    "Test",
		Type:    DeviceTypeMPD,
		Address: srv.Addr(),
		UserID:  1,
	}); err != nil {
		t.Fatalf("registry.Register: %v", err)
	}
	resolver := NewRegistryDeviceResolver(registry, slog.Default())

	tracks := &mockTrackProvider{
		tracks: []Track{
			{ID: 1, AlbumID: 100, Duration: 60},
			{ID: 2, AlbumID: 100, Duration: 60},
			{ID: 3, AlbumID: 100, Duration: 60},
		},
	}
	repo := &mockSessionRepository{}
	bc := &recordingBroadcaster{}

	svc := NewService(tracks, repo)
	svc.SetBroadcaster(bc)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(id, _ int64) string {
		return "http://audiod.local/stream"
	})

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "mpd-x"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}

	// Track plays normally for a bit.
	srv.SetState("play")
	srv.SetElapsed(45)
	svc.PollMPDPositions() // observedPlay latched

	// Track ends: MPD reports stop. Poller advances once.
	srv.SetState("stop")
	srv.SetElapsed(0)
	svc.PollMPDPositions()

	sess, _ := repo.GetByUserID(1)
	if sess.Current == nil || sess.Current.TrackID != 2 {
		t.Errorf("expected advance to track 2, got %+v", sess.Current)
	}

	// Right after the advance, MPD is briefly in stop again (Clear+Add+Play
	// sequence). Poller must NOT fire a second advance.
	srv.SetState("stop")
	srv.SetElapsed(0)
	svc.PollMPDPositions()
	svc.PollMPDPositions()

	sess, _ = repo.GetByUserID(1)
	if sess.Current.TrackID != 2 {
		t.Errorf("double-skip detected: expected TrackID=2 after extra stop polls, got %d",
			sess.Current.TrackID)
	}
}

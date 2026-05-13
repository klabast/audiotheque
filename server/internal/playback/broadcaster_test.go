package playback

import "testing"

// Every session-mutating method must broadcast the resulting session
// so all of the user's connected clients see updates in real time.
func TestService_AllMutationsBroadcast(t *testing.T) {
	tracks := &mockTrackProvider{
		tracks: []Track{
			{ID: 1, AlbumID: 100, Duration: 180},
			{ID: 2, AlbumID: 100, Duration: 200},
			{ID: 3, AlbumID: 100, Duration: 220},
		},
	}
	repo := &mockSessionRepository{}
	bc := &recordingBroadcaster{}
	// Two stub devices so we have a transfer target. Every active session
	// names a device under the unified-session invariant.
	devA := &mockPlaybackDevice{state: "play"}
	devB := &mockPlaybackDevice{state: "play"}
	resolver := &mockDeviceResolver{
		devices: map[string]PlaybackDevice{"dev-a": devA, "dev-b": devB},
	}
	svc := NewService(tracks, repo)
	svc.SetBroadcaster(bc)
	svc.SetDeviceResolver(resolver)
	svc.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	if _, err := svc.PlayAlbumOnDevice(1, 100, 0, "dev-a"); err != nil {
		t.Fatalf("PlayAlbumOnDevice: %v", err)
	}
	if _, err := svc.Pause(1, 30); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if _, err := svc.Resume(1); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if _, err := svc.Next(1); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if _, err := svc.Previous(1); err != nil {
		t.Fatalf("Previous: %v", err)
	}
	if _, err := svc.SeekTrack(1, 60); err != nil {
		t.Fatalf("SeekTrack: %v", err)
	}
	if _, err := svc.SetVolume(1, 50); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if _, err := svc.TransferPlayback(1, "dev-b"); err != nil {
		t.Fatalf("TransferPlayback: %v", err)
	}

	const want = 8
	if len(bc.calls) != want {
		t.Fatalf("expected %d broadcasts, got %d", want, len(bc.calls))
	}
	for i, c := range bc.calls {
		if c.userID != 1 {
			t.Errorf("call %d: userID = %d, want 1", i, c.userID)
		}
		if c.session == nil {
			t.Errorf("call %d: session is nil", i)
		}
	}
}

// UpdatePosition is on the WS hot path; it must also broadcast so other
// clients see the seekbar tick.
func TestService_UpdatePositionBroadcasts(t *testing.T) {
	repo := &mockSessionRepository{
		session: &Session{
			UserID:  1,
			State:   StatePlaying,
			Current: &CurrentTrack{TrackID: 5, Position: 0},
		},
	}
	bc := &recordingBroadcaster{}
	svc := NewService(nil, repo)
	svc.SetBroadcaster(bc)

	svc.UpdatePosition(1, 42)

	if len(bc.calls) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bc.calls))
	}
	if bc.calls[0].session.Current.Position != 42 {
		t.Errorf("position = %d, want 42", bc.calls[0].session.Current.Position)
	}
	if bc.calls[0].exceptCID != "" {
		t.Errorf("server-side update should broadcast to all (exceptCID = %q)", bc.calls[0].exceptCID)
	}
}

// When the position update came from a specific client (the browser pushing
// its own audio.currentTime), that client must be excluded from the broadcast
// so it doesn't receive an echo of the value it just sent.
func TestService_UpdatePositionFromClient_ExcludesSender(t *testing.T) {
	repo := &mockSessionRepository{
		session: &Session{
			UserID:  1,
			State:   StatePlaying,
			Current: &CurrentTrack{TrackID: 5, Position: 0},
		},
	}
	bc := &recordingBroadcaster{}
	svc := NewService(nil, repo)
	svc.SetBroadcaster(bc)

	svc.UpdatePositionFromClient(1, 42, "c-browser-tab-3")

	if len(bc.calls) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bc.calls))
	}
	if bc.calls[0].exceptCID != "c-browser-tab-3" {
		t.Errorf("exceptCID = %q, want c-browser-tab-3", bc.calls[0].exceptCID)
	}
}

type broadcastCall struct {
	userID    int64
	session   *Session
	exceptCID string
}

type recordingBroadcaster struct {
	calls []broadcastCall
}

func (r *recordingBroadcaster) BroadcastSession(userID int64, session *Session, exceptClientID string) {
	r.calls = append(r.calls, broadcastCall{
		userID:    userID,
		session:   session,
		exceptCID: exceptClientID,
	})
}

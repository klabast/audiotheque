package playback

import (
	"testing"
)

// TDD: First test - User can play an album
// This test drives the design: what does the service need to play an album?
func TestService_PlayAlbum(t *testing.T) {
	// Arrange - Design the dependencies through the test
	trackProvider := &mockTrackProvider{
		tracks: []Track{
			{ID: 1, Title: "Track 1", AlbumID: 100, Duration: 180},
			{ID: 2, Title: "Track 2", AlbumID: 100, Duration: 200},
			{ID: 3, Title: "Track 3", AlbumID: 100, Duration: 220},
		},
	}

	sessionRepo := &mockSessionRepository{}

	service := NewService(trackProvider, sessionRepo)
	service.SetDeviceResolver(&mockDeviceResolver{
		devices: map[string]PlaybackDevice{"dev": &mockPlaybackDevice{state: "play"}},
	})
	service.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	userID := int64(1)
	albumID := int64(100)

	// Act
	session, err := service.PlayAlbumOnDevice(userID, albumID, 0, "dev")

	// Assert
	if err != nil {
		t.Fatalf("PlayAlbumOnDevice failed: %v", err)
	}

	if session == nil {
		t.Fatal("Expected session to be created")
	}

	// Session belongs to user
	if session.UserID != userID {
		t.Errorf("Expected UserID %d, got %d", userID, session.UserID)
	}

	// Source is the album
	if session.Source.Type != SourceTypeAlbum {
		t.Errorf("Expected source type %s, got %s", SourceTypeAlbum, session.Source.Type)
	}
	if session.Source.ID != albumID {
		t.Errorf("Expected source ID %d, got %d", albumID, session.Source.ID)
	}

	// First track is current
	if session.Current == nil {
		t.Fatal("Expected current track to be set")
	}
	if session.Current.TrackID != 1 {
		t.Errorf("Expected current track ID 1, got %d", session.Current.TrackID)
	}

	// Remaining tracks are in source
	if len(session.Source.Remaining) != 2 {
		t.Errorf("Expected 2 remaining tracks, got %d", len(session.Source.Remaining))
	}

	// State is playing
	if session.State != StatePlaying {
		t.Errorf("Expected state %s, got %s", StatePlaying, session.State)
	}

	// Queue is empty (no explicit queue items)
	if len(session.Queue) != 0 {
		t.Errorf("Expected empty queue, got %d items", len(session.Queue))
	}

	// Session was persisted
	if !sessionRepo.saveCalled {
		t.Error("Expected session to be saved to repository")
	}
}

// Mock implementations - these define the interfaces we need

type mockTrackProvider struct {
	tracks []Track
}

func (m *mockTrackProvider) GetAlbumTracks(albumID int64) ([]Track, error) {
	var result []Track
	for _, t := range m.tracks {
		if t.AlbumID == albumID {
			result = append(result, t)
		}
	}
	return result, nil
}

// testDeviceID is the deviceID used by service-level tests that don't care
// about device routing but still need to satisfy the unified-session
// invariant (every active session names an addressable device).
const testDeviceID = "test-device"

// newServiceWithTestDevice returns a Service wired with the no-op test device
// + a stream URL builder. Use this for tests that exercise session logic
// (Pause/Resume/Next/etc.) where the device interaction is incidental.
func newServiceWithTestDevice(tracks TrackProvider, repo SessionRepository) *Service {
	svc := NewService(tracks, repo)
	svc.SetDeviceResolver(&mockDeviceResolver{
		devices: map[string]PlaybackDevice{testDeviceID: &mockPlaybackDevice{state: "play"}},
	})
	svc.SetStreamURLBuilder(func(int64, int64) string { return "http://test/stream" })
	return svc
}

type mockSessionRepository struct {
	saveCalled   bool
	deleteCalled bool
	session      *Session
}

func (m *mockSessionRepository) Save(session *Session) error {
	m.saveCalled = true
	m.session = session
	return nil
}

func (m *mockSessionRepository) GetByUserID(userID int64) (*Session, error) {
	return m.session, nil
}

func (m *mockSessionRepository) Delete(userID int64) error {
	m.deleteCalled = true
	m.session = nil
	return nil
}

func (m *mockSessionRepository) DeleteWithoutDevice() (int64, error) {
	if m.session != nil && m.session.DeviceID == "" {
		m.session = nil
		return 1, nil
	}
	return 0, nil
}

// TDD: A client position push that regresses position on the same track must
// be ignored. Regression is the symptom of a stale browser timeupdate event
// racing with a recent server-driven Seek (or any other path that moved the
// position forward). Trusting it would clobber the seeked position with the
// browser's pre-broadcast localCurrentTime.
//
// Regression test for the e2e failure on commit 7ced71106… on desktop CI:
// once UserIDGetter was wired up, position-from-client messages stopped
// being silently dropped (userID=0 → no session) and could clobber the
// seek. This guard restores the protection without re-introducing the
// auth bug.
func TestService_UpdatePositionFromClient_IgnoresRegression(t *testing.T) {
	existingSession := &Session{
		UserID:  1,
		State:   StatePlaying,
		Current: &CurrentTrack{TrackID: 42, Position: 90},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}
	service := NewService(&mockTrackProvider{}, sessionRepo)

	// Stale push from the browser (audio was at ~1s when the user-driven
	// seek to 90 happened). This must not overwrite session.position.
	service.UpdatePositionFromClient(1, 1, "client-A")

	if existingSession.Current.Position != 90 {
		t.Errorf("regression push clobbered position: want 90, got %d",
			existingSession.Current.Position)
	}
}

// TDD: Forward (or near-equal) progress pushes from a client are honoured.
// Without this, the position would never advance from the client side.
func TestService_UpdatePositionFromClient_AcceptsForwardProgress(t *testing.T) {
	existingSession := &Session{
		UserID:  1,
		State:   StatePlaying,
		Current: &CurrentTrack{TrackID: 42, Position: 90},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}
	service := NewService(&mockTrackProvider{}, sessionRepo)

	service.UpdatePositionFromClient(1, 92, "client-A")

	if existingSession.Current.Position != 92 {
		t.Errorf("forward push not applied: want 92, got %d",
			existingSession.Current.Position)
	}
}

// TDD: Server-side position sources (the MPD position poller) are NOT
// subject to the regression guard — the device is authoritative for its
// own elapsed time, and during track transitions or device-driven seeks
// the elapsed value can legitimately drop. Only client pushes need the
// guard, because only clients have stale events racing with server seeks.
func TestService_UpdatePosition_AllowsRegressionFromServer(t *testing.T) {
	existingSession := &Session{
		UserID:  1,
		State:   StatePlaying,
		Current: &CurrentTrack{TrackID: 42, Position: 90},
	}
	sessionRepo := &mockSessionRepository{session: existingSession}
	service := NewService(&mockTrackProvider{}, sessionRepo)

	service.UpdatePosition(1, 1)

	if existingSession.Current.Position != 1 {
		t.Errorf("server-side position update not applied: want 1, got %d",
			existingSession.Current.Position)
	}
}

// TDD: User can play an album starting from a specific track in it.
// The chosen track becomes Current; tracks after it become Remaining;
// tracks before it are skipped (Spotify-style).
func TestService_PlayAlbum_StartFromTrack(t *testing.T) {
	trackProvider := &mockTrackProvider{
		tracks: []Track{
			{ID: 1, Title: "Track 1", AlbumID: 100, Duration: 180},
			{ID: 2, Title: "Track 2", AlbumID: 100, Duration: 200},
			{ID: 3, Title: "Track 3", AlbumID: 100, Duration: 220},
		},
	}
	service := NewService(trackProvider, &mockSessionRepository{})
	service.SetDeviceResolver(&mockDeviceResolver{
		devices: map[string]PlaybackDevice{"dev": &mockPlaybackDevice{state: "play"}},
	})
	service.SetStreamURLBuilder(func(int64, int64) string { return "http://x" })

	session, err := service.PlayAlbumOnDevice(1, 100, 2, "dev")
	if err != nil {
		t.Fatalf("PlayAlbumOnDevice failed: %v", err)
	}

	if session.Current == nil || session.Current.TrackID != 2 {
		t.Fatalf("Expected current track ID 2, got %+v", session.Current)
	}
	if len(session.Source.Remaining) != 1 || session.Source.Remaining[0] != 3 {
		t.Errorf("Expected Remaining = [3], got %v", session.Source.Remaining)
	}
}

func TestService_PlayAlbum_StartTrackNotInAlbum(t *testing.T) {
	trackProvider := &mockTrackProvider{
		tracks: []Track{
			{ID: 1, Title: "Track 1", AlbumID: 100, Duration: 180},
		},
	}
	service := NewService(trackProvider, &mockSessionRepository{})
	service.SetDeviceResolver(&mockDeviceResolver{
		devices: map[string]PlaybackDevice{"dev": &mockPlaybackDevice{state: "play"}},
	})

	if _, err := service.PlayAlbumOnDevice(1, 100, 999, "dev"); err == nil {
		t.Error("Expected error when startTrackID is not in album, got nil")
	}
}

// TDD: Second test - User can get their current session
func TestService_GetSession(t *testing.T) {
	// Arrange
	existingSession := &Session{
		ID:       1,
		UserID:   1,
		State:    StatePlaying,
		DeviceID: testDeviceID,
		Current: &CurrentTrack{
			TrackID:  5,
			Position: 45,
		},
	}

	sessionRepo := &mockSessionRepository{
		session: existingSession,
	}

	service := newServiceWithTestDevice(nil, sessionRepo)

	// Act
	session, err := service.GetSession(1)

	// Assert
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if session == nil {
		t.Fatal("Expected session to be returned")
	}

	if session.Current.TrackID != 5 {
		t.Errorf("Expected track ID 5, got %d", session.Current.TrackID)
	}

	if session.Current.Position != 45 {
		t.Errorf("Expected position 45, got %d", session.Current.Position)
	}
}

// TDD: User can pause playback
func TestService_Pause(t *testing.T) {
	// Arrange - session is playing
	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: testDeviceID,
		Current: &CurrentTrack{
			TrackID:  5,
			Position: 0,
		},
	}

	sessionRepo := &mockSessionRepository{session: existingSession}
	service := newServiceWithTestDevice(nil, sessionRepo)

	// Act - pause at position 45 seconds
	session, err := service.Pause(1, 45)

	// Assert
	if err != nil {
		t.Fatalf("Pause failed: %v", err)
	}

	if session.State != StatePaused {
		t.Errorf("Expected state %s, got %s", StatePaused, session.State)
	}

	if session.Current.Position != 45 {
		t.Errorf("Expected position 45, got %d", session.Current.Position)
	}

	if !sessionRepo.saveCalled {
		t.Error("Expected session to be saved")
	}
}

// TDD: User can resume playback
func TestService_Resume(t *testing.T) {
	// Arrange - session is paused
	existingSession := &Session{
		UserID:   1,
		State:    StatePaused,
		DeviceID: testDeviceID,
		Current: &CurrentTrack{
			TrackID:  5,
			Position: 45,
		},
	}

	sessionRepo := &mockSessionRepository{session: existingSession}
	service := newServiceWithTestDevice(nil, sessionRepo)

	// Act
	session, err := service.Resume(1)

	// Assert
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}

	if session.State != StatePlaying {
		t.Errorf("Expected state %s, got %s", StatePlaying, session.State)
	}

	// Position should be preserved
	if session.Current.Position != 45 {
		t.Errorf("Expected position 45, got %d", session.Current.Position)
	}

	if !sessionRepo.saveCalled {
		t.Error("Expected session to be saved")
	}
}

// TDD: User can skip to next track (from source remaining)
func TestService_Next_FromSource(t *testing.T) {
	// Arrange - playing track 1, tracks 2 and 3 remaining
	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: testDeviceID,
		Current: &CurrentTrack{
			TrackID:  1,
			Position: 100,
		},
		Source: Source{
			Type:      SourceTypeAlbum,
			ID:        100,
			Remaining: []int64{2, 3},
		},
		Queue:   []QueueItem{},
		History: []int64{},
	}

	sessionRepo := &mockSessionRepository{session: existingSession}
	service := newServiceWithTestDevice(nil, sessionRepo)

	// Act
	session, err := service.Next(1)

	// Assert
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}

	// Current track should be track 2
	if session.Current.TrackID != 2 {
		t.Errorf("Expected current track 2, got %d", session.Current.TrackID)
	}

	// Position should reset to 0
	if session.Current.Position != 0 {
		t.Errorf("Expected position 0, got %d", session.Current.Position)
	}

	// Track 1 should be in history
	if len(session.History) != 1 || session.History[0] != 1 {
		t.Errorf("Expected history [1], got %v", session.History)
	}

	// Remaining should be [3]
	if len(session.Source.Remaining) != 1 || session.Source.Remaining[0] != 3 {
		t.Errorf("Expected remaining [3], got %v", session.Source.Remaining)
	}

	// Should still be playing
	if session.State != StatePlaying {
		t.Errorf("Expected state %s, got %s", StatePlaying, session.State)
	}
}

// TDD: User can skip to next track (from explicit queue first)
func TestService_Next_FromQueue(t *testing.T) {
	// Arrange - playing track 1, track 10 in queue, tracks 2,3 in remaining
	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: testDeviceID,
		Current: &CurrentTrack{
			TrackID:  1,
			Position: 100,
		},
		Source: Source{
			Type:      SourceTypeAlbum,
			ID:        100,
			Remaining: []int64{2, 3},
		},
		Queue: []QueueItem{
			{TrackID: 10, AddedFromID: 200, AddedFrom: SourceTypeAlbum},
		},
		History: []int64{},
	}

	sessionRepo := &mockSessionRepository{session: existingSession}
	service := newServiceWithTestDevice(nil, sessionRepo)

	// Act
	session, err := service.Next(1)

	// Assert
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}

	// Current track should be track 10 (from queue, not remaining)
	if session.Current.TrackID != 10 {
		t.Errorf("Expected current track 10, got %d", session.Current.TrackID)
	}

	// Queue should now be empty
	if len(session.Queue) != 0 {
		t.Errorf("Expected empty queue, got %v", session.Queue)
	}

	// Remaining should still have 2, 3
	if len(session.Source.Remaining) != 2 {
		t.Errorf("Expected remaining [2,3], got %v", session.Source.Remaining)
	}
}

// TDD: Next at end of album stops playback
func TestService_Next_EndOfAlbum(t *testing.T) {
	// Arrange - playing last track, no more remaining
	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: testDeviceID,
		Current: &CurrentTrack{
			TrackID:  3,
			Position: 100,
		},
		Source: Source{
			Type:      SourceTypeAlbum,
			ID:        100,
			Remaining: []int64{},
		},
		Queue:   []QueueItem{},
		History: []int64{1, 2},
	}

	sessionRepo := &mockSessionRepository{session: existingSession}
	service := newServiceWithTestDevice(nil, sessionRepo)

	// Act
	session, err := service.Next(1)

	// Assert
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}

	// Should be stopped
	if session.State != StateStopped {
		t.Errorf("Expected state %s, got %s", StateStopped, session.State)
	}

	// Current should be nil (nothing playing)
	if session.Current != nil {
		t.Errorf("Expected no current track, got %v", session.Current)
	}
}

// TDD: User can go to previous track
func TestService_Previous(t *testing.T) {
	// Arrange - playing track 2, track 1 in history
	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: testDeviceID,
		Current: &CurrentTrack{
			TrackID:  2,
			Position: 50,
		},
		Source: Source{
			Type:      SourceTypeAlbum,
			ID:        100,
			Remaining: []int64{3},
		},
		Queue:   []QueueItem{},
		History: []int64{1},
	}

	sessionRepo := &mockSessionRepository{session: existingSession}
	service := newServiceWithTestDevice(nil, sessionRepo)

	// Act
	session, err := service.Previous(1)

	// Assert
	if err != nil {
		t.Fatalf("Previous failed: %v", err)
	}

	// Current track should be track 1 (from history)
	if session.Current.TrackID != 1 {
		t.Errorf("Expected current track 1, got %d", session.Current.TrackID)
	}

	// Position should reset to 0
	if session.Current.Position != 0 {
		t.Errorf("Expected position 0, got %d", session.Current.Position)
	}

	// History should be empty now
	if len(session.History) != 0 {
		t.Errorf("Expected empty history, got %v", session.History)
	}

	// Track 2 should be added back to remaining (at front)
	if len(session.Source.Remaining) != 2 || session.Source.Remaining[0] != 2 {
		t.Errorf("Expected remaining [2,3], got %v", session.Source.Remaining)
	}
}

// TDD: Previous with no history restarts current track
func TestService_Previous_NoHistory(t *testing.T) {
	// Arrange - playing first track, no history
	existingSession := &Session{
		UserID:   1,
		State:    StatePlaying,
		DeviceID: testDeviceID,
		Current: &CurrentTrack{
			TrackID:  1,
			Position: 50,
		},
		Source: Source{
			Type:      SourceTypeAlbum,
			ID:        100,
			Remaining: []int64{2, 3},
		},
		Queue:   []QueueItem{},
		History: []int64{},
	}

	sessionRepo := &mockSessionRepository{session: existingSession}
	service := newServiceWithTestDevice(nil, sessionRepo)

	// Act
	session, err := service.Previous(1)

	// Assert
	if err != nil {
		t.Fatalf("Previous failed: %v", err)
	}

	// Current track should still be track 1
	if session.Current.TrackID != 1 {
		t.Errorf("Expected current track 1, got %d", session.Current.TrackID)
	}

	// Position should reset to 0
	if session.Current.Position != 0 {
		t.Errorf("Expected position 0, got %d", session.Current.Position)
	}
}

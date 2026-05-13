package playback

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"audiod/internal/library"
)

// TDD: User can stream a track they have access to
func TestHandleStreamTrack_Success(t *testing.T) {
	// Create a temporary audio file to stream
	tempDir := t.TempDir()
	testFilePath := filepath.Join(tempDir, "test.mp3")
	testContent := []byte("fake mp3 content for testing")
	if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Arrange
	mockTrackStore := &mockTrackStore{
		track: &library.Track{
			ID:        1,
			LibraryID: 100,
			FilePath:  testFilePath,
			FileName:  "test.mp3",
			Codec:     "mp3",
		},
		hasAccess: true,
	}

	getUserID := func(r *http.Request) (int64, error) {
		return 1, nil // User ID 1
	}

	handler := NewHandler(nil, getUserID)
	handler.SetTrackStore(mockTrackStore)

	// Create request
	req := httptest.NewRequest("GET", "/api/tracks/1/stream", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	// Act
	handler.HandleStreamTrack(w, req)

	// Assert
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "audio/mpeg" {
		t.Errorf("Expected Content-Type audio/mpeg, got %s", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != string(testContent) {
		t.Errorf("Response body doesn't match file content")
	}
}

// TDD: Unauthenticated user gets 401
func TestHandleStreamTrack_Unauthorized(t *testing.T) {
	getUserID := func(r *http.Request) (int64, error) {
		return 0, errors.New("unauthorized")
	}

	handler := NewHandler(nil, getUserID)

	req := httptest.NewRequest("GET", "/api/tracks/1/stream", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	handler.HandleStreamTrack(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

// TDD: Track not found returns 404
func TestHandleStreamTrack_TrackNotFound(t *testing.T) {
	mockTrackStore := &mockTrackStore{
		track:     nil,
		hasAccess: false,
		err:       ErrTrackNotFound,
	}

	getUserID := func(r *http.Request) (int64, error) {
		return 1, nil
	}

	handler := NewHandler(nil, getUserID)
	handler.SetTrackStore(mockTrackStore)

	req := httptest.NewRequest("GET", "/api/tracks/999/stream", nil)
	req.SetPathValue("id", "999")
	w := httptest.NewRecorder()

	handler.HandleStreamTrack(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

// TDD: User without access to library gets 404 (don't leak existence)
func TestHandleStreamTrack_NoAccess(t *testing.T) {
	mockTrackStore := &mockTrackStore{
		track: &library.Track{
			ID:        1,
			LibraryID: 100,
			FilePath:  "/some/path.mp3",
		},
		hasAccess: false,
	}

	getUserID := func(r *http.Request) (int64, error) {
		return 1, nil
	}

	handler := NewHandler(nil, getUserID)
	handler.SetTrackStore(mockTrackStore)

	req := httptest.NewRequest("GET", "/api/tracks/1/stream", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	handler.HandleStreamTrack(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 (for security), got %d", resp.StatusCode)
	}
}

// TDD: Invalid track ID returns 400
func TestHandleStreamTrack_InvalidID(t *testing.T) {
	getUserID := func(r *http.Request) (int64, error) {
		return 1, nil
	}

	handler := NewHandler(nil, getUserID)

	req := httptest.NewRequest("GET", "/api/tracks/invalid/stream", nil)
	req.SetPathValue("id", "invalid")
	w := httptest.NewRecorder()

	handler.HandleStreamTrack(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

// TDD: MPD device fetches stream using a signed query-string token (no cookie).
// This is the path used by MPD devices on the LAN: audiod mints the URL with
// ?token=... and the device fetches it without any session.
func TestHandleStreamTrack_SignedTokenSucceedsWithoutCookie(t *testing.T) {
	tempDir := t.TempDir()
	testFilePath := filepath.Join(tempDir, "test.mp3")
	testContent := []byte("fake mp3 content for testing")
	if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mockTrackStore := &mockTrackStore{
		track: &library.Track{
			ID: 1, LibraryID: 100, FilePath: testFilePath,
			FileName: "test.mp3", Codec: "mp3",
		},
		hasAccess: true,
	}

	// Cookie auth fails — simulating an MPD device with no session.
	getUserID := func(r *http.Request) (int64, error) {
		return 0, errors.New("unauthorized")
	}

	// Token validator accepts "valid-token-for-track-1" only when trackID == 1.
	validator := func(token string, trackID int64) (int64, error) {
		if token == "valid-token-for-track-1" && trackID == 1 {
			return 42, nil // userID encoded in the token
		}
		return 0, errors.New("invalid token")
	}

	handler := NewHandler(nil, getUserID)
	handler.SetTrackStore(mockTrackStore)
	handler.SetStreamTokenValidator(validator)

	req := httptest.NewRequest("GET", "/api/tracks/1/stream?token=valid-token-for-track-1", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	handler.HandleStreamTrack(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 with valid signed token, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != string(testContent) {
		t.Errorf("Response body doesn't match file content")
	}
}

// TDD: A token whose trackID doesn't match the requested URL is rejected.
// Prevents using a token minted for track 1 to fetch track 2.
func TestHandleStreamTrack_SignedTokenWrongTrackRejected(t *testing.T) {
	mockTrackStore := &mockTrackStore{
		track:     &library.Track{ID: 2, LibraryID: 100, FilePath: "/x.mp3"},
		hasAccess: true,
	}

	getUserID := func(r *http.Request) (int64, error) {
		return 0, errors.New("unauthorized")
	}

	// Validator only OKs trackID 1 — request asks for trackID 2.
	validator := func(token string, trackID int64) (int64, error) {
		if trackID == 1 {
			return 42, nil
		}
		return 0, errors.New("token trackID mismatch")
	}

	handler := NewHandler(nil, getUserID)
	handler.SetTrackStore(mockTrackStore)
	handler.SetStreamTokenValidator(validator)

	req := httptest.NewRequest("GET", "/api/tracks/2/stream?token=valid-token-for-track-1", nil)
	req.SetPathValue("id", "2")
	w := httptest.NewRecorder()

	handler.HandleStreamTrack(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for token/track mismatch, got %d", resp.StatusCode)
	}
}

// TDD: An invalid/expired token still falls through to 401.
func TestHandleStreamTrack_SignedTokenInvalidRejected(t *testing.T) {
	getUserID := func(r *http.Request) (int64, error) {
		return 0, errors.New("unauthorized")
	}
	validator := func(token string, trackID int64) (int64, error) {
		return 0, errors.New("invalid token")
	}

	handler := NewHandler(nil, getUserID)
	handler.SetStreamTokenValidator(validator)

	req := httptest.NewRequest("GET", "/api/tracks/1/stream?token=garbage", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	handler.HandleStreamTrack(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for invalid token, got %d", resp.StatusCode)
	}
}

// Mock implementations

type mockTrackStore struct {
	track     *library.Track
	hasAccess bool
	err       error
}

func (m *mockTrackStore) GetTrackByID(trackID int64) (*library.Track, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.track, nil
}

func (m *mockTrackStore) UserHasLibraryAccess(userID, libraryID int64) (bool, error) {
	return m.hasAccess, nil
}

// SessionToResponse should surface a deviceCapabilities hint when a
// CapabilitiesFn is supplied — used by clients to grey out volume controls
// on mixerless devices (e.g. HiFiBerry's `mixer_type "none"`) instead of
// hitting an error every time the slider moves.
func TestSessionToResponse_PopulatesDeviceCapabilities(t *testing.T) {
	session := &Session{
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 0},
	}

	caps := func(deviceID string) *DeviceCapabilities {
		if deviceID == "mpd-1" {
			return &DeviceCapabilities{Volume: false}
		}
		return nil
	}

	resp := SessionToResponse(session, caps)
	if resp.DeviceCapabilities == nil {
		t.Fatal("expected DeviceCapabilities to be populated")
	}
	if resp.DeviceCapabilities.Volume {
		t.Error("expected Volume=false for mixerless device")
	}
}

// Without a capability resolver, SessionToResponse must still produce a
// valid response (older callers, browser-only sessions, etc.).
func TestSessionToResponse_NilCapabilitiesFn_OmitsField(t *testing.T) {
	session := &Session{
		State:    StatePlaying,
		DeviceID: "mpd-1",
		Current:  &CurrentTrack{TrackID: 5, Position: 0},
	}
	resp := SessionToResponse(session, nil)
	if resp.DeviceCapabilities != nil {
		t.Error("expected DeviceCapabilities to be nil when no resolver supplied")
	}
}

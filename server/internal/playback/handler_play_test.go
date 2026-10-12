package playback

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubService satisfies ServiceInterface and records calls. By default every
// method returns an error so tests can detect unintended invocations.
type stubService struct {
	playAlbumCalled         bool
	playAlbumOnDeviceCalled bool
	lastDeviceID            string
	lastTransferDeviceID    string
	// err, when set, is returned by every session-mutating method so tests can
	// exercise the handler's error mapping.
	err          error
	caps         *DeviceCapabilities
	capsDeviceID string

	queueCall     string
	queueTrackIDs []int64
	queueAlbumID  int64
	queuePosition QueuePosition
	queueItemID   string
	queueIndex    int
}

func (s *stubService) PlayAlbum(userID, albumID, startTrackID int64) (*Session, error) {
	s.playAlbumCalled = true
	return nil, errors.New("stub: PlayAlbum should not be reached")
}
func (s *stubService) PlayAlbumOnDevice(userID, albumID, startTrackID int64, deviceID string) (*Session, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.playAlbumOnDeviceCalled = true
	s.lastDeviceID = deviceID
	return &Session{UserID: userID, State: StatePlaying, DeviceID: deviceID,
		Current: &CurrentTrack{TrackID: 1, Position: 0}}, nil
}
func (s *stubService) GetSession(userID int64) (*Session, error) { return nil, s.err }
func (s *stubService) Pause(userID int64, p int) (*Session, error) {
	return s.emptySession(userID)
}
func (s *stubService) Resume(userID int64) (*Session, error)   { return s.emptySession(userID) }
func (s *stubService) Next(userID int64) (*Session, error)     { return s.emptySession(userID) }
func (s *stubService) Previous(userID int64) (*Session, error) { return s.emptySession(userID) }
func (s *stubService) TransferPlayback(u int64, d string) (*Session, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.lastTransferDeviceID = d
	return &Session{UserID: u, DeviceID: d}, nil
}
func (s *stubService) SeekTrack(userID int64, p int) (*Session, error) {
	return s.emptySession(userID)
}
func (s *stubService) SetVolume(userID int64, v int) (*Session, error) {
	return s.emptySession(userID)
}

func (s *stubService) DeviceCapabilities(deviceID string) *DeviceCapabilities {
	s.capsDeviceID = deviceID
	return s.caps
}

func (s *stubService) emptySession(userID int64) (*Session, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &Session{UserID: userID, State: StatePlaying, DeviceID: "stub"}, nil
}

// TDD: When the request body omits `deviceId` but the requesting tab carries
// its hub client ID in `X-Audiod-Client-Id`, the handler must default the play
// target to that tab. This is what makes "play here" work without the legacy
// empty-string sentinel: the server captures the requesting client's identity
// and the session is born with a real DeviceID.
func TestHandlePlay_NoDeviceButHeader_UsesClientIDFromHeader(t *testing.T) {
	registry := NewBrowserDeviceRegistry()
	registry.Register("c1", 1, "Chrome on Linux")

	svc := &stubService{}
	getUserID := func(r *http.Request) (int64, error) { return 1, nil }
	handler := NewHandler(svc, getUserID)
	handler.SetBrowserRegistry(registry)

	body := bytes.NewBufferString(`{"albumId": 1}`)
	req := httptest.NewRequest("POST", "/api/playback/play", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Audiod-Client-Id", "c1")
	w := httptest.NewRecorder()

	handler.HandlePlay(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	if !svc.playAlbumOnDeviceCalled {
		t.Errorf("expected PlayAlbumOnDevice to be called with header-derived deviceID, got PlayAlbum=%v",
			svc.playAlbumCalled)
	}
	if svc.lastDeviceID != "c1" {
		t.Errorf("session deviceID: want %q (from X-Audiod-Client-Id), got %q", "c1", svc.lastDeviceID)
	}
}

// TDD: An X-Audiod-Client-Id that doesn't match any registered browser tab is
// not a valid play target either — the server must not let a stale or forged
// client ID through. Reject with 400.
func TestHandlePlay_HeaderClientIDNotRegistered_Returns400(t *testing.T) {
	registry := NewBrowserDeviceRegistry()
	// Note: no registration for "c1".

	svc := &stubService{}
	getUserID := func(r *http.Request) (int64, error) { return 1, nil }
	handler := NewHandler(svc, getUserID)
	handler.SetBrowserRegistry(registry)

	body := bytes.NewBufferString(`{"albumId": 1}`)
	req := httptest.NewRequest("POST", "/api/playback/play", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Audiod-Client-Id", "c1")
	w := httptest.NewRecorder()

	handler.HandlePlay(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", resp.StatusCode)
	}
	if svc.playAlbumCalled || svc.playAlbumOnDeviceCalled {
		t.Errorf("service must not be called when client ID is unknown")
	}
}

// TDD: Sessions must name a real playback device. A play request with no
// `deviceId` in the body AND no `X-Audiod-Client-Id` header has no way to know
// where to play — the handler must reject with 400 before the service is
// touched. This is the invariant that lets the unified-session model work
// across clients: every session is owned by an addressable device.
func TestHandlePlay_NoDeviceAndNoClientID_Returns400(t *testing.T) {
	svc := &stubService{}
	getUserID := func(r *http.Request) (int64, error) { return 1, nil }
	handler := NewHandler(svc, getUserID)
	// No browser registry needed — request will be rejected before lookup.

	body := bytes.NewBufferString(`{"albumId": 1}`)
	req := httptest.NewRequest("POST", "/api/playback/play", body)
	req.Header.Set("Content-Type", "application/json")
	// Deliberately NO X-Audiod-Client-Id header.
	w := httptest.NewRecorder()

	handler.HandlePlay(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", resp.StatusCode)
	}
	if svc.playAlbumCalled || svc.playAlbumOnDeviceCalled {
		t.Errorf("service must not be called when no device can be inferred (PlayAlbum=%v, PlayAlbumOnDevice=%v)",
			svc.playAlbumCalled, svc.playAlbumOnDeviceCalled)
	}
}

func (s *stubService) AddTracksToQueue(userID int64, ids []int64, pos QueuePosition) (*Session, error) {
	s.queueCall, s.queueTrackIDs, s.queuePosition = "addTracks", ids, pos
	return s.queuedSession(userID)
}
func (s *stubService) AddAlbumToQueue(userID, albumID int64, pos QueuePosition) (*Session, error) {
	s.queueCall, s.queueAlbumID, s.queuePosition = "addAlbum", albumID, pos
	return s.queuedSession(userID)
}
func (s *stubService) RemoveFromQueue(userID int64, itemID string) (*Session, error) {
	s.queueCall, s.queueItemID = "remove", itemID
	return s.queuedSession(userID)
}
func (s *stubService) ClearQueue(userID int64) (*Session, error) {
	s.queueCall = "clear"
	return s.queuedSession(userID)
}
func (s *stubService) MoveQueueItem(userID int64, itemID string, to int) (*Session, error) {
	s.queueCall, s.queueItemID, s.queueIndex = "move", itemID, to
	return s.queuedSession(userID)
}
func (s *stubService) PlayFromQueue(userID int64, itemID string) (*Session, error) {
	s.queueCall, s.queueItemID = "play", itemID
	return s.queuedSession(userID)
}

func (s *stubService) queuedSession(userID int64) (*Session, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &Session{UserID: userID, State: StatePlaying, DeviceID: "stub",
		Queue: []QueueItem{{ID: "q1", TrackID: 7, AddedFrom: SourceTypeAlbum, AddedFromID: 3}}}, nil
}

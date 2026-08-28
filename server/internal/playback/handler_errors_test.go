package playback

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// P3-15: pause/resume/next/previous/transfer/seek/volume mapped EVERY service
// error to 404, so "MPD is down" reached the client as "you have no session".
// The raw wrapped error was echoed too, leaking MPD host addresses.
func TestHandlers_MapServiceErrorsToAccurateCodes(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"no session", ErrNoSession, http.StatusNotFound},
		{"no device on session", ErrNoDevice, http.StatusConflict},
		{"device gone", fmt.Errorf("resolve device: %w", ErrDeviceNotFound), http.StatusNotFound},
		{
			"device unreachable",
			fmt.Errorf("device pause: %w: dial tcp 192.168.1.50:6600: connect: connection refused", ErrDeviceUnreachable),
			http.StatusBadGateway,
		},
	}

	calls := map[string]func(*Handler, *http.Request, http.ResponseWriter){
		"pause":    func(h *Handler, r *http.Request, w http.ResponseWriter) { h.HandlePause(w, r) },
		"resume":   func(h *Handler, r *http.Request, w http.ResponseWriter) { h.HandleResume(w, r) },
		"next":     func(h *Handler, r *http.Request, w http.ResponseWriter) { h.HandleNext(w, r) },
		"previous": func(h *Handler, r *http.Request, w http.ResponseWriter) { h.HandlePrevious(w, r) },
		"seek":     func(h *Handler, r *http.Request, w http.ResponseWriter) { h.HandleSeek(w, r) },
		"volume":   func(h *Handler, r *http.Request, w http.ResponseWriter) { h.HandleVolume(w, r) },
	}

	for _, tc := range tests {
		for name, call := range calls {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				svc := &stubService{err: tc.err}
				handler := NewHandler(svc, func(*http.Request) (int64, error) { return 1, nil })

				req := httptest.NewRequest("POST", "/api/playback/"+name, bytes.NewBufferString(`{}`))
				w := httptest.NewRecorder()
				call(handler, req, w)

				resp := w.Result()
				defer resp.Body.Close()
				if resp.StatusCode != tc.wantCode {
					t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantCode)
				}
				body, _ := io.ReadAll(resp.Body)
				if strings.Contains(string(body), "192.168.1.50") {
					t.Errorf("response leaks internal detail to the client: %q", body)
				}
			})
		}
	}
}

// P3-15: ErrNoDevice is documented as 409 on play; the handler returned 500.
func TestHandlePlay_NoDeviceReturns409(t *testing.T) {
	registry := NewBrowserDeviceRegistry()
	registry.Register("c1", 1, "Chrome")

	svc := &stubService{err: ErrNoDevice}
	handler := NewHandler(svc, func(*http.Request) (int64, error) { return 1, nil })
	handler.SetBrowserRegistry(registry)

	req := httptest.NewRequest("POST", "/api/playback/play", bytes.NewBufferString(`{"albumId":1}`))
	req.Header.Set("X-Audiod-Client-Id", "c1")
	w := httptest.NewRecorder()
	handler.HandlePlay(w, req)

	if got := w.Result().StatusCode; got != http.StatusConflict {
		t.Errorf("status = %d, want 409", got)
	}
}

// P3-17: resolvePlayTarget only validated registry membership for the INFERRED
// client ID. An explicit deviceId went straight through, so a user could bind
// their session to somebody else's browser tab and drive audio on it.
func TestResolvePlayTarget_RejectsAnotherUsersBrowserTab(t *testing.T) {
	registry := NewBrowserDeviceRegistry()
	registry.Register("c-victim", 99, "Victim's Chrome")
	registry.Register("c-mine", 1, "My Chrome")

	svc := &stubService{}
	handler := NewHandler(svc, func(*http.Request) (int64, error) { return 1, nil })
	handler.SetBrowserRegistry(registry)

	body := bytes.NewBufferString(`{"albumId":1,"deviceId":"c-victim"}`)
	req := httptest.NewRequest("POST", "/api/playback/play", body)
	req.Header.Set("X-Audiod-Client-Id", "c-mine")
	w := httptest.NewRecorder()
	handler.HandlePlay(w, req)

	if got := w.Result().StatusCode; got != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", got)
	}
	if svc.playAlbumOnDeviceCalled {
		t.Error("service was called with another user's browser tab as the play target")
	}
}

// P3-17: MPD devices are global by design — an explicit device ID that isn't a
// browser tab must still pass through.
func TestResolvePlayTarget_AllowsMPDDeviceID(t *testing.T) {
	registry := NewBrowserDeviceRegistry()
	registry.Register("c-mine", 1, "My Chrome")

	svc := &stubService{}
	handler := NewHandler(svc, func(*http.Request) (int64, error) { return 1, nil })
	handler.SetBrowserRegistry(registry)

	body := bytes.NewBufferString(`{"albumId":1,"deviceId":"audio-wz"}`)
	req := httptest.NewRequest("POST", "/api/playback/play", body)
	req.Header.Set("X-Audiod-Client-Id", "c-mine")
	w := httptest.NewRecorder()
	handler.HandlePlay(w, req)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want 200", got)
	}
	if svc.lastDeviceID != "audio-wz" {
		t.Errorf("play target = %q, want audio-wz", svc.lastDeviceID)
	}
}

// P3-16: the handler did device I/O itself to build the capability hint.
// Device access belongs to the service, which owns the resolver.
func TestHandler_CapabilitiesComeFromService(t *testing.T) {
	svc := &stubService{caps: &DeviceCapabilities{Volume: false}}
	handler := NewHandler(svc, func(*http.Request) (int64, error) { return 1, nil })

	fn := handler.capabilitiesFor()
	if fn == nil {
		t.Fatal("expected a capabilities function backed by the service")
	}
	got := fn("mpd-1")
	if got == nil || got.Volume {
		t.Errorf("capabilities = %+v, want Volume=false from the service", got)
	}
	if svc.capsDeviceID != "mpd-1" {
		t.Errorf("service was asked about %q, want mpd-1", svc.capsDeviceID)
	}
}

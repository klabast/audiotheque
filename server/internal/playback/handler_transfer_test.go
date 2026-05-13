package playback

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TDD: Transfer without a body deviceId and without X-Audiod-Client-Id has no
// target — reject with 400.
func TestHandleTransfer_NoDeviceAndNoClientID_Returns400(t *testing.T) {
	svc := &stubService{}
	getUserID := func(r *http.Request) (int64, error) { return 1, nil }
	handler := NewHandler(svc, getUserID)

	body := bytes.NewBufferString(`{"deviceId": ""}`)
	req := httptest.NewRequest("POST", "/api/playback/transfer", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleTransfer(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", resp.StatusCode)
	}
}

// TDD: Empty body deviceId with X-Audiod-Client-Id means "transfer to me".
// The service must be called with the header-derived deviceID, not "".
func TestHandleTransfer_NoDeviceButHeader_TransfersToRequester(t *testing.T) {
	registry := NewBrowserDeviceRegistry()
	registry.Register("c1", 1, "Chrome on macOS")

	svc := &stubService{}
	getUserID := func(r *http.Request) (int64, error) { return 1, nil }
	handler := NewHandler(svc, getUserID)
	handler.SetBrowserRegistry(registry)

	body := bytes.NewBufferString(`{"deviceId": ""}`)
	req := httptest.NewRequest("POST", "/api/playback/transfer", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Audiod-Client-Id", "c1")
	w := httptest.NewRecorder()

	handler.HandleTransfer(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	// stubService records the last TransferPlayback target.
	if svc.lastTransferDeviceID != "c1" {
		t.Errorf("transfer target: want %q (from header), got %q", "c1", svc.lastTransferDeviceID)
	}
}

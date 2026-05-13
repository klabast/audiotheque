package playback

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TDD: Two browser tabs share the same user. The list endpoint must mark only
// the requesting tab as current (via IsCurrent), and must NOT rewrite either
// browser's Name — server stays out of i18n. The UI will localize "This
// Device" for the row whose IsCurrent is true.
func TestHandleListDevices_TwoBrowsers_MarksOnlyRequesterAsCurrent(t *testing.T) {
	registry := NewBrowserDeviceRegistry()
	registry.Register("c1", 1, "Chrome on macOS")
	registry.Register("c2", 1, "Firefox on Linux")

	getUserID := func(r *http.Request) (int64, error) { return 1, nil }
	handler := NewHandler(nil, getUserID)
	handler.SetBrowserRegistry(registry)

	req := httptest.NewRequest("GET", "/api/devices", nil)
	req.Header.Set("X-Audiod-Client-Id", "c1")
	w := httptest.NewRecorder()

	handler.HandleListDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}

	var got []DeviceResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	byID := map[string]DeviceResponse{}
	for _, d := range got {
		byID[d.ID] = d
	}

	if len(byID) != 2 {
		t.Fatalf("want exactly 2 devices keyed by id, got %d: %+v", len(byID), got)
	}

	c1, ok := byID["c1"]
	if !ok {
		t.Fatalf("missing c1 in response: %+v", got)
	}
	if c1.Name != "Chrome on macOS" {
		t.Errorf("c1.Name: server must not rewrite it. want %q, got %q", "Chrome on macOS", c1.Name)
	}
	if !c1.IsCurrent {
		t.Errorf("c1.IsCurrent: requester must be marked current, got false")
	}

	c2, ok := byID["c2"]
	if !ok {
		t.Fatalf("missing c2 in response: %+v", got)
	}
	if c2.Name != "Firefox on Linux" {
		t.Errorf("c2.Name: want %q, got %q", "Firefox on Linux", c2.Name)
	}
	if c2.IsCurrent {
		t.Errorf("c2.IsCurrent: only the requester is current, got true")
	}

	// No synthesized empty-ID row should sneak in.
	if _, hasEmpty := byID[""]; hasEmpty {
		t.Errorf("response must not include a synthetic empty-ID device row: %+v", got)
	}
}

// TDD: Under the unified-session invariant there is no synthetic empty-ID
// stand-in row. When the requesting tab isn't in the registry (no header, or
// WS welcome hasn't landed yet), the endpoint returns the registered
// browsers as-is, none flagged current. The UI surfaces that honestly
// (loading state) rather than receiving a phantom "This Device" entry whose
// ID can't be addressed.
func TestHandleListDevices_UnknownRequester_NoSyntheticRow(t *testing.T) {
	registry := NewBrowserDeviceRegistry()
	registry.Register("c1", 1, "Chrome on macOS")

	getUserID := func(r *http.Request) (int64, error) { return 1, nil }
	handler := NewHandler(nil, getUserID)
	handler.SetBrowserRegistry(registry)

	// No X-Audiod-Client-Id header.
	req := httptest.NewRequest("GET", "/api/devices", nil)
	w := httptest.NewRecorder()

	handler.HandleListDevices(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}

	var got []DeviceResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("want exactly 1 device (just c1), got %d: %+v", len(got), got)
	}
	if got[0].ID != "c1" {
		t.Errorf("want id=c1, got %q (no synthetic empty-id fallback allowed)", got[0].ID)
	}
	if got[0].Name != "Chrome on macOS" {
		t.Errorf("want name=Chrome on macOS, got %q", got[0].Name)
	}
	if got[0].IsCurrent {
		t.Errorf("no registered requester → nothing is current, got IsCurrent=true")
	}
}

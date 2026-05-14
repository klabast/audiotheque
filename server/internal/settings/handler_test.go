package settings

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"audiod/internal/auth"
)

// --- Auth mocks (same pattern as library/handler_test.go) ---

type mockAuthRepository struct {
	users map[int64]*auth.User
}

func (m *mockAuthRepository) GetByID(id int64) (*auth.User, error) {
	if user, ok := m.users[id]; ok {
		return user, nil
	}
	return nil, auth.ErrUserNotFound
}
func (m *mockAuthRepository) GetByUsername(string) (*auth.User, error)   { return nil, auth.ErrUserNotFound }
func (m *mockAuthRepository) GetUserCount() (int, error)                 { return len(m.users), nil }
func (m *mockAuthRepository) GetAdminCount() (int, error)                { return 1, nil }
func (m *mockAuthRepository) Create(string, string, bool) (*auth.User, error) { return nil, nil }
func (m *mockAuthRepository) UpdatePassword(int64, string) error         { return nil }
func (m *mockAuthRepository) StoreResetCode(string, int64, time.Time) error { return nil }
func (m *mockAuthRepository) GetResetCode(string) (*auth.ResetCode, error) {
	return nil, auth.ErrInvalidResetCode
}
func (m *mockAuthRepository) DeleteResetCode(string) error        { return nil }
func (m *mockAuthRepository) DeleteExpiredResetCodes() error      { return nil }
func (m *mockAuthRepository) DeleteResetCodesByUserID(int64) error { return nil }

// testSessionRepo is shared between createTestAuthService and addAuthCookie
// so the handler's session lookup finds the row addAuthCookie inserted.
var testSessionRepo auth.SessionRepository

func createTestAuthService() *auth.Service {
	repo := &mockAuthRepository{
		users: map[int64]*auth.User{
			1: {ID: 1, Username: "admin", IsAdmin: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	testSessionRepo = auth.NewInMemorySessionRepository()
	return auth.NewService(repo, testSessionRepo)
}

func addAuthCookie(req *http.Request, userID int64) error {
	if testSessionRepo == nil {
		testSessionRepo = auth.NewInMemorySessionRepository()
	}
	id := fmt.Sprintf("test-session-%d-%d", userID, time.Now().UnixNano())
	now := time.Now()
	err := testSessionRepo.Create(&auth.Session{
		ID:         id,
		UserID:     userID,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(time.Hour),
	})
	if err != nil {
		return err
	}
	req.AddCookie(&http.Cookie{Name: "audiod_token", Value: id})
	return nil
}

// --- Settings service mock ---

type mockSettingsService struct {
	devices           []Device
	createdDevice     *Device
	updatedDevice     *Device
	createErr         error
	updateErr         error
	deleteErr         error
	streamingHostname string
	streamingErr      error
}

func (m *mockSettingsService) CreateDevice(name, deviceType, address string) (*Device, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return m.createdDevice, nil
}

func (m *mockSettingsService) ListDevices() ([]Device, error) {
	return m.devices, nil
}

func (m *mockSettingsService) UpdateDevice(id, name, address string) (*Device, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	return m.updatedDevice, nil
}

func (m *mockSettingsService) DeleteDevice(id string) error {
	return m.deleteErr
}

func (m *mockSettingsService) GetStreamingHostname() (string, error) {
	if m.streamingErr != nil {
		return "", m.streamingErr
	}
	if m.streamingHostname == "" {
		return "localhost:8080", nil
	}
	return m.streamingHostname, nil
}

func (m *mockSettingsService) SetStreamingHostname(hostname string) error {
	if m.streamingErr != nil {
		return m.streamingErr
	}
	m.streamingHostname = hostname
	return nil
}

// --- Tests ---

func TestHandleListDevices(t *testing.T) {
	svc := &mockSettingsService{
		devices: []Device{
			{ID: "d1", Name: "Speaker", Type: "mpd", Address: "10.0.0.1:6600"},
		},
	}
	h := NewHandler(svc, createTestAuthService())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/devices", nil)
	if err := addAuthCookie(req, 1); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var devices []Device
	if err := json.Unmarshal(w.Body.Bytes(), &devices); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(devices))
	}
}

func TestHandleCreateDevice(t *testing.T) {
	svc := &mockSettingsService{
		createdDevice: &Device{ID: "new-id", Name: "Speaker", Type: "mpd", Address: "10.0.0.1:6600"},
	}
	h := NewHandler(svc, createTestAuthService())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"name":"Speaker","type":"mpd","address":"10.0.0.1:6600"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/devices", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if err := addAuthCookie(req, 1); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestHandleUpdateDevice(t *testing.T) {
	svc := &mockSettingsService{
		updatedDevice: &Device{ID: "d1", Name: "New Name", Type: "mpd", Address: "10.0.0.2:6600"},
	}
	h := NewHandler(svc, createTestAuthService())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"name":"New Name","address":"10.0.0.2:6600"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/devices/d1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if err := addAuthCookie(req, 1); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandleDeleteDevice(t *testing.T) {
	svc := &mockSettingsService{}
	h := NewHandler(svc, createTestAuthService())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/devices/d1", nil)
	if err := addAuthCookie(req, 1); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestHandleGetStreamingHostname(t *testing.T) {
	svc := &mockSettingsService{streamingHostname: "192.168.1.5:8080"}
	h := NewHandler(svc, createTestAuthService())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/streaming", nil)
	if err := addAuthCookie(req, 1); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["hostname"] != "192.168.1.5:8080" {
		t.Errorf("hostname = %q, want %q", resp["hostname"], "192.168.1.5:8080")
	}
}

func TestHandleSetStreamingHostname(t *testing.T) {
	svc := &mockSettingsService{}
	h := NewHandler(svc, createTestAuthService())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"hostname":"192.168.1.5:8080"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/streaming", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if err := addAuthCookie(req, 1); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	if svc.streamingHostname != "192.168.1.5:8080" {
		t.Errorf("hostname not set; got %q", svc.streamingHostname)
	}
}

func TestHandleDevices_Unauthenticated(t *testing.T) {
	svc := &mockSettingsService{}
	h := NewHandler(svc, createTestAuthService())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/devices", nil)
	// No auth cookie
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

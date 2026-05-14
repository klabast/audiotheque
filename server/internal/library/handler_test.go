package library

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"audiod/internal/auth"
)

// mockService implements ServiceInterface for handler tests
type mockService struct {
	startScanError        error
	getScanProgressError  error
	scanProgress          *ScanProgress
	libraries             []*Library
	createLibraryResult   *Library
	createLibraryError    error
	deleteLibraryError    error
	updateLibraryResult   *Library
	updateLibraryError    error
	albumCoverPath        string
	getAlbumCoverPathErr  error
}

func (m *mockService) ListLibraries(userID int64) ([]*Library, error) {
	return m.libraries, nil
}

func (m *mockService) CreateLibrary(userID int64, name string, paths []string) (*Library, error) {
	return m.createLibraryResult, m.createLibraryError
}

func (m *mockService) StartScan(libraryID int64) error {
	return m.startScanError
}

func (m *mockService) GetScanProgress(libraryID int64) (*ScanProgress, error) {
	return m.scanProgress, m.getScanProgressError
}

func (m *mockService) DeleteLibrary(libraryID int64) error {
	return m.deleteLibraryError
}

func (m *mockService) UpdateLibrary(libraryID int64, name string, paths []string) (*Library, error) {
	return m.updateLibraryResult, m.updateLibraryError
}

func (m *mockService) ListAlbums(libraryID int64, opts ListAlbumsOptions) ([]*AlbumWithArtist, error) {
	return nil, nil
}

func (m *mockService) GetAlbumCoverPath(albumID int64) (string, error) {
	return m.albumCoverPath, m.getAlbumCoverPathErr
}

func (m *mockService) GetAlbum(albumID int64) (*AlbumWithArtist, error) {
	return nil, nil
}

func (m *mockService) ListTracksByAlbum(albumID int64) ([]*TrackWithArtist, error) {
	return nil, nil
}

func (m *mockService) Search(libraryID int64, query string) (*SearchResult, error) {
	return &SearchResult{}, nil
}

// mockAuthRepository implements auth repository for testing
type mockAuthRepository struct {
	users map[int64]*auth.User
}

func (m *mockAuthRepository) GetByID(id int64) (*auth.User, error) {
	if user, ok := m.users[id]; ok {
		return user, nil
	}
	return nil, auth.ErrUserNotFound
}

func (m *mockAuthRepository) GetByUsername(username string) (*auth.User, error) {
	return nil, auth.ErrUserNotFound
}

func (m *mockAuthRepository) GetUserCount() (int, error) {
	return len(m.users), nil
}

func (m *mockAuthRepository) GetAdminCount() (int, error) {
	count := 0
	for _, user := range m.users {
		if user.IsAdmin {
			count++
		}
	}
	return count, nil
}

func (m *mockAuthRepository) Create(username, passwordHash string, isAdmin bool) (*auth.User, error) {
	return nil, nil
}

func (m *mockAuthRepository) UpdatePassword(userID int64, passwordHash string) error {
	return nil
}

func (m *mockAuthRepository) StoreResetCode(code string, userID int64, expiresAt time.Time) error {
	return nil
}

func (m *mockAuthRepository) GetResetCode(code string) (*auth.ResetCode, error) {
	return nil, auth.ErrInvalidResetCode
}

func (m *mockAuthRepository) DeleteResetCode(code string) error {
	return nil
}

func (m *mockAuthRepository) DeleteExpiredResetCodes() error {
	return nil
}

func (m *mockAuthRepository) DeleteResetCodesByUserID(userID int64) error {
	return nil
}

// createAdminUser returns a mock admin user for testing
func createAdminUser() *auth.User {
	return &auth.User{
		ID:        1,
		Username:  "admin",
		IsAdmin:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// createTestAuthService creates an auth service with a mock admin user
func createTestAuthService() *auth.Service {
	repo := &mockAuthRepository{
		users: map[int64]*auth.User{
			1: createAdminUser(),
		},
	}
	testSessionRepo = auth.NewInMemorySessionRepository()
	return auth.NewService(repo, testSessionRepo)
}

// testSessionRepo is the in-memory session store shared by addAuthCookie
// and the auth service constructed in newAuthService. Tests that bypass
// newAuthService (constructing auth.NewService inline) must seed this from
// their own setup.
var testSessionRepo auth.SessionRepository

// addAuthCookie creates a session row for userID in testSessionRepo and
// attaches its opaque ID to req as the audiod_token cookie. Mirrors how a
// real login wires the cookie, so the handler under test resolves the user
// via the same path production does.
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

// TestHandleScanLibrary_Success tests successful scan initiation
func TestHandleScanLibrary_Success(t *testing.T) {
	// Arrange
	mockSvc := &mockService{
		startScanError: nil,
	}
	handler := &Handler{
		service:     mockSvc,
		authService: createTestAuthService(),
	}

	// Use ServeMux to get proper path value support
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/libraries/{id}/scan", handler.HandleScanLibrary)

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/1/scan", nil)
	if err := addAuthCookie(req, 1); err != nil {
		t.Fatalf("failed to add auth cookie: %v", err)
	}
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202 Accepted, got %d", w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["message"] != "Scan started" {
		t.Errorf("expected message 'Scan started', got %q", response["message"])
	}
}

// TestHandleScanLibrary_AlreadyInProgress tests 409 Conflict response
func TestHandleScanLibrary_AlreadyInProgress(t *testing.T) {
	// Arrange
	mockSvc := &mockService{
		startScanError: ErrScanAlreadyInProgress,
	}
	handler := &Handler{
		service:     mockSvc,
		authService: createTestAuthService(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/libraries/{id}/scan", handler.HandleScanLibrary)

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/1/scan", nil)
	if err := addAuthCookie(req, 1); err != nil {
		t.Fatalf("failed to add auth cookie: %v", err)
	}
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409 Conflict, got %d", w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] != "Scan already in progress" {
		t.Errorf("expected error 'Scan already in progress', got %q", response["error"])
	}
}

// TestHandleScanLibrary_LibraryNotFound tests 500 response for library not found
func TestHandleScanLibrary_LibraryNotFound(t *testing.T) {
	// Arrange
	mockSvc := &mockService{
		startScanError: ErrLibraryNotFound,
	}
	handler := &Handler{
		service:     mockSvc,
		authService: createTestAuthService(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/libraries/{id}/scan", handler.HandleScanLibrary)

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/999/scan", nil)
	if err := addAuthCookie(req, 1); err != nil {
		t.Fatalf("failed to add auth cookie: %v", err)
	}
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 Internal Server Error, got %d", w.Code)
	}
}

// TestHandleScanLibrary_MethodNotAllowed tests that non-POST requests are rejected
func TestHandleScanLibrary_MethodNotAllowed(t *testing.T) {
	// Arrange
	mockSvc := &mockService{}
	handler := &Handler{service: mockSvc}

	req := httptest.NewRequest(http.MethodGet, "/api/libraries/1/scan", nil)
	w := httptest.NewRecorder()

	// Act
	handler.HandleScanLibrary(w, req)

	// Assert
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405 Method Not Allowed, got %d", w.Code)
	}
}

// TestHandleGetScanStatus_Success tests successful progress retrieval
func TestHandleGetScanStatus_Success(t *testing.T) {
	// Arrange
	progress := &ScanProgress{
		LibraryID:      1,
		Status:         "running",
		TotalFiles:     100,
		ProcessedFiles: 50,
		TracksAdded:    25,
		TracksUpdated:  10,
		Errors:         0,
		CurrentFile:    "/music/album/track.flac",
	}
	mockSvc := &mockService{
		scanProgress:         progress,
		getScanProgressError: nil,
	}
	handler := &Handler{service: mockSvc}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/libraries/{id}/scan-status", handler.HandleGetScanStatus)

	req := httptest.NewRequest(http.MethodGet, "/api/libraries/1/scan-status", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", w.Code)
	}

	var response ScanProgress
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.LibraryID != progress.LibraryID {
		t.Errorf("expected libraryID %d, got %d", progress.LibraryID, response.LibraryID)
	}

	if response.Status != "running" {
		t.Errorf("expected status 'running', got %q", response.Status)
	}

	if response.ProcessedFiles != 50 {
		t.Errorf("expected processedFiles 50, got %d", response.ProcessedFiles)
	}
}

// TestHandleGetScanStatus_NoScanInProgress tests 204 No Content response
func TestHandleGetScanStatus_NoScanInProgress(t *testing.T) {
	// Arrange
	mockSvc := &mockService{
		scanProgress:         nil,
		getScanProgressError: ErrNoScanInProgress,
	}
	handler := &Handler{service: mockSvc}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/libraries/{id}/scan-status", handler.HandleGetScanStatus)

	req := httptest.NewRequest(http.MethodGet, "/api/libraries/1/scan-status", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204 No Content, got %d", w.Code)
	}

	if w.Body.Len() != 0 {
		t.Error("expected empty response body for 204 status")
	}
}

// TestHandleGetScanStatus_InternalError tests 500 response for unexpected errors
func TestHandleGetScanStatus_InternalError(t *testing.T) {
	// Arrange
	mockSvc := &mockService{
		scanProgress:         nil,
		getScanProgressError: ErrLibraryNotFound,
	}
	handler := &Handler{service: mockSvc}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/libraries/{id}/scan-status", handler.HandleGetScanStatus)

	req := httptest.NewRequest(http.MethodGet, "/api/libraries/999/scan-status", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 Internal Server Error, got %d", w.Code)
	}
}

// TestHandleGetScanStatus_MethodNotAllowed tests that non-GET requests are rejected
func TestHandleGetScanStatus_MethodNotAllowed(t *testing.T) {
	// Arrange
	mockSvc := &mockService{}
	handler := &Handler{service: mockSvc}

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/1/scan-status", nil)
	w := httptest.NewRecorder()

	// Act
	handler.HandleGetScanStatus(w, req)

	// Assert
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405 Method Not Allowed, got %d", w.Code)
	}
}

// ==============================================================================
// Delete Library Handler Tests
// ==============================================================================

// TestHandleDeleteLibrary_Success tests successful library deletion
func TestHandleDeleteLibrary_Success(t *testing.T) {
	// Arrange
	mockSvc := &mockService{
		deleteLibraryError: nil,
	}
	handler := &Handler{
		service:     mockSvc,
		authService: createTestAuthService(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/libraries/{id}", handler.HandleDeleteLibrary)

	req := httptest.NewRequest(http.MethodDelete, "/api/libraries/1", nil)
	if err := addAuthCookie(req, 1); err != nil {
		t.Fatalf("failed to add auth cookie: %v", err)
	}
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204 No Content, got %d", w.Code)
	}
}

// TestHandleDeleteLibrary_NotFound tests 404 response for non-existent library
func TestHandleDeleteLibrary_NotFound(t *testing.T) {
	// Arrange
	mockSvc := &mockService{
		deleteLibraryError: ErrLibraryNotFound,
	}
	handler := &Handler{
		service:     mockSvc,
		authService: createTestAuthService(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/libraries/{id}", handler.HandleDeleteLibrary)

	req := httptest.NewRequest(http.MethodDelete, "/api/libraries/999", nil)
	if err := addAuthCookie(req, 1); err != nil {
		t.Fatalf("failed to add auth cookie: %v", err)
	}
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 Not Found, got %d", w.Code)
	}
}

// TestHandleDeleteLibrary_Forbidden tests that non-admin users cannot delete libraries
func TestHandleDeleteLibrary_Forbidden(t *testing.T) {
	// Arrange
	mockSvc := &mockService{}
	// Create auth service with non-admin user
	repo := &mockAuthRepository{
		users: map[int64]*auth.User{
			2: {
				ID:        2,
				Username:  "regularuser",
				IsAdmin:   false,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}
	testSessionRepo = auth.NewInMemorySessionRepository()
	handler := &Handler{
		service:     mockSvc,
		authService: auth.NewService(repo, testSessionRepo),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/libraries/{id}", handler.HandleDeleteLibrary)

	req := httptest.NewRequest(http.MethodDelete, "/api/libraries/1", nil)
	if err := addAuthCookie(req, 2); err != nil {
		t.Fatalf("failed to add auth cookie: %v", err)
	}
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden, got %d", w.Code)
	}
}

// ==============================================================================
// Album Cover Handler Tests
// ==============================================================================

// TestHandleGetAlbumCover_Success tests serving album cover art
func TestHandleGetAlbumCover_Success(t *testing.T) {
	// Arrange - create a temp cover file
	tempDir := t.TempDir()
	coverContent := []byte{0xFF, 0xD8, 0xFF, 0xE0} // JPEG magic bytes
	coverPath := filepath.Join(tempDir, "covers", "album_1.jpg")
	if err := os.MkdirAll(filepath.Dir(coverPath), 0755); err != nil {
		t.Fatalf("failed to create covers dir: %v", err)
	}
	if err := os.WriteFile(coverPath, coverContent, 0644); err != nil {
		t.Fatalf("failed to write cover file: %v", err)
	}

	mockSvc := &mockService{
		albumCoverPath: coverPath,
	}
	handler := &Handler{
		service: mockSvc,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/albums/{id}/cover", handler.HandleGetAlbumCover)

	req := httptest.NewRequest(http.MethodGet, "/api/albums/1/cover", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %q", w.Header().Get("Content-Type"))
	}

	if len(w.Body.Bytes()) != len(coverContent) {
		t.Errorf("expected body length %d, got %d", len(coverContent), len(w.Body.Bytes()))
	}
}

// TestHandleGetAlbumCover_NotFound tests 404 when album has no cover
func TestHandleGetAlbumCover_NotFound(t *testing.T) {
	// Arrange
	mockSvc := &mockService{
		getAlbumCoverPathErr: ErrAlbumNotFound,
	}
	handler := &Handler{
		service: mockSvc,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/albums/{id}/cover", handler.HandleGetAlbumCover)

	req := httptest.NewRequest(http.MethodGet, "/api/albums/999/cover", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 Not Found, got %d", w.Code)
	}
}

// TestHandleGetAlbumCover_NoCover tests 404 when album exists but has no cover
func TestHandleGetAlbumCover_NoCover(t *testing.T) {
	// Arrange
	mockSvc := &mockService{
		albumCoverPath: "", // Empty path means no cover
	}
	handler := &Handler{
		service: mockSvc,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/albums/{id}/cover", handler.HandleGetAlbumCover)

	req := httptest.NewRequest(http.MethodGet, "/api/albums/1/cover", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 Not Found, got %d", w.Code)
	}
}

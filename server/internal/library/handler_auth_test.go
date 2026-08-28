package library

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// libraryRoutes is every route the library handler registers, paired with a
// concrete request path. Kept as one table so a newly registered route that
// forgets authentication fails here rather than shipping open.
var libraryRoutes = []struct {
	name   string
	method string
	path   string
}{
	{"list libraries", http.MethodGet, "/api/libraries"},
	{"create library", http.MethodPost, "/api/libraries"},
	{"update library", http.MethodPut, "/api/libraries/1"},
	{"delete library", http.MethodDelete, "/api/libraries/1"},
	{"scan library", http.MethodPost, "/api/libraries/1/scan"},
	{"scan status", http.MethodGet, "/api/libraries/1/scan-status"},
	{"list albums", http.MethodGet, "/api/libraries/1/albums"},
	{"search", http.MethodGet, "/api/libraries/1/search?q=x"},
	{"get album", http.MethodGet, "/api/albums/1"},
	{"album cover", http.MethodGet, "/api/albums/1/cover"},
	{"album tracks", http.MethodGet, "/api/albums/1/tracks"},
}

func newAuthTestMux() *http.ServeMux {
	handler := &Handler{
		service:     &mockService{},
		authService: createTestAuthService(),
	}
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

// TestAllLibraryRoutesRequireAuthentication is the regression guard for the
// six read routes that shipped with no authentication at all: the whole
// catalog — album titles, artists, track listings and cover art — was
// readable by anyone who could reach the port.
func TestAllLibraryRoutesRequireAuthentication(t *testing.T) {
	mux := newAuthTestMux()

	for _, route := range libraryRoutes {
		t.Run(route.name, func(t *testing.T) {
			// Given a request carrying no session cookie
			req := httptest.NewRequest(route.method, route.path, nil)
			w := httptest.NewRecorder()

			// When it reaches the router
			mux.ServeHTTP(w, req)

			// Then it is rejected as unauthenticated
			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s %s: got status %d, want %d",
					route.method, route.path, w.Code, http.StatusUnauthorized)
			}
		})
	}
}

// TestReadRoutesRejectUserWithoutLibraryAccess covers the ACL half of the same
// gap: authentication alone let any logged-in user read any library by
// changing the path id, bypassing library_access entirely.
func TestReadRoutesRejectUserWithoutLibraryAccess(t *testing.T) {
	readRoutes := []struct {
		name string
		path string
	}{
		{"scan status", "/api/libraries/1/scan-status"},
		{"list albums", "/api/libraries/1/albums"},
		{"search", "/api/libraries/1/search?q=x"},
		{"get album", "/api/albums/1"},
		{"album cover", "/api/albums/1/cover"},
		{"album tracks", "/api/albums/1/tracks"},
	}

	for _, route := range readRoutes {
		t.Run(route.name, func(t *testing.T) {
			// Given a service that grants the caller no library access
			handler := &Handler{
				service:     &mockService{accessErr: ErrLibraryAccessDenied},
				authService: createTestAuthService(),
			}
			mux := http.NewServeMux()
			handler.RegisterRoutes(mux)

			req := httptest.NewRequest(http.MethodGet, route.path, nil)
			if err := addAuthCookie(req, 1); err != nil {
				t.Fatalf("addAuthCookie() failed: %v", err)
			}
			w := httptest.NewRecorder()

			// When an authenticated user without access requests it
			mux.ServeHTTP(w, req)

			// Then it is forbidden, not served
			if w.Code != http.StatusForbidden {
				t.Errorf("GET %s: got status %d, want %d",
					route.path, w.Code, http.StatusForbidden)
			}
		})
	}
}

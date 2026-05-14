package auth

import (
	"net/http/httptest"
	"testing"
)

// When auth is disabled, GetAuthenticatedUser should resolve every request
// to the canonical admin without inspecting cookies or sessions. This is the
// core behavioural change behind the "auth-disabled" mode: handlers don't
// need to special-case anything — they keep calling the middleware and
// receive a valid *User regardless of cookie state.
func TestGetAuthenticatedUser_AuthDisabled_ReturnsCanonicalAdmin(t *testing.T) {
	repo := newMockRepository()
	hash, _ := HashPassword("pw")
	admin := &User{ID: 1, Username: "alice", PasswordHash: hash, IsAdmin: true}
	repo.users["alice"] = admin
	repo.usersID[1] = admin

	service := NewService(repo, NewInMemorySessionRepository())
	service.SetAuthEnabledFn(func() (bool, error) { return false, nil })

	// No cookie attached on purpose — auth-disabled must not require one.
	r := httptest.NewRequest("GET", "/api/anything", nil)
	user, err := GetAuthenticatedUser(r, service)
	if err != nil {
		t.Fatalf("GetAuthenticatedUser failed: %v", err)
	}
	if user == nil || user.Username != "alice" {
		t.Errorf("got %+v, want alice (canonical admin)", user)
	}
}

// AuthenticateRequest is the cookie-writing variant — when auth is disabled
// it should still return the canonical admin without writing a cookie (there
// is no session to renew).
func TestAuthenticateRequest_AuthDisabled_NoCookieWritten(t *testing.T) {
	repo := newMockRepository()
	hash, _ := HashPassword("pw")
	admin := &User{ID: 2, Username: "alice", PasswordHash: hash, IsAdmin: true}
	repo.users["alice"] = admin
	repo.usersID[2] = admin

	service := NewService(repo, NewInMemorySessionRepository())
	service.SetAuthEnabledFn(func() (bool, error) { return false, nil })

	r := httptest.NewRequest("GET", "/api/anything", nil)
	w := httptest.NewRecorder()
	user, err := AuthenticateRequest(w, r, service)
	if err != nil {
		t.Fatalf("AuthenticateRequest failed: %v", err)
	}
	if user.Username != "alice" {
		t.Errorf("got %q, want alice", user.Username)
	}
	if got := w.Result().Cookies(); len(got) != 0 {
		t.Errorf("expected no cookies to be set in auth-disabled mode, got %d", len(got))
	}
}

// Sanity check: with auth enabled and no cookie, the middleware still
// returns ErrUnauthorized — i.e. we haven't broken the normal path.
func TestGetAuthenticatedUser_AuthEnabled_NoCookie_ReturnsUnauthorized(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())
	// authEnabledFn left nil → AuthEnabled() returns true.

	r := httptest.NewRequest("GET", "/api/anything", nil)
	if _, err := GetAuthenticatedUser(r, service); err != ErrUnauthorized {
		t.Errorf("got err=%v, want ErrUnauthorized", err)
	}
}

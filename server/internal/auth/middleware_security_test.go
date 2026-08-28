package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newAuthDisabledService returns a service with auth switched off and one
// admin — the shape an attacker finds on a server whose owner turned auth off
// so the household doesn't have to type a password.
func newAuthDisabledService(t *testing.T) (*Service, *mockRepository) {
	t.Helper()
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())
	service.SetAuthEnabledFn(func() (bool, error) { return false, nil })
	seedUserWithPassword(t, repo, 1, "owner", "ownerpassword", true)
	return service, repo
}

// Auth-disabled means "no password to listen to music", not "anyone on the
// network may reprovision this server". Privilege-granting routes must still
// demand a genuine session.
func TestRequireRealAdminUser_AuthDisabled_RejectsAnonymousCaller(t *testing.T) {
	service, _ := newAuthDisabledService(t)

	reached := false
	handler := RequireRealAdminUser(service, func(http.ResponseWriter, *http.Request, *User) {
		reached = true
	})

	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodPost, "/api/users", nil))

	if reached {
		t.Error("anonymous caller reached an admin-only handler with auth disabled")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestRequireRealUser_AuthDisabled_RejectsAnonymousCaller(t *testing.T) {
	service, _ := newAuthDisabledService(t)

	handler := RequireRealUser(service, func(http.ResponseWriter, *http.Request, *User) {})

	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodPost, "/api/auth/verify-password", nil))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

// The real admin keeps working with auth disabled — they have a session.
func TestRequireRealAdminUser_AuthDisabled_AcceptsSignedInAdmin(t *testing.T) {
	service, _ := newAuthDisabledService(t)

	sessionID, err := service.CreateSession(1, SessionContext{})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	var got *User
	handler := RequireRealAdminUser(service, func(_ http.ResponseWriter, _ *http.Request, user *User) {
		got = user
	})

	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}
	if got == nil || got.Username != "owner" {
		t.Errorf("got %+v, want the owner", got)
	}
}

// A signed-in non-admin is still forbidden from admin surfaces.
func TestRequireRealAdminUser_RejectsNonAdmin(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, NewInMemorySessionRepository())
	seedUserWithPassword(t, repo, 2, "bob", "bobpassword", false)

	sessionID, err := service.CreateSession(2, SessionContext{})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	handler := RequireRealAdminUser(service, func(http.ResponseWriter, *http.Request, *User) {})
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", w.Code)
	}
}

// Revoking sessions grants nothing to the caller but takes access away from
// the owner, so it stays behind a real session even with auth disabled.
func TestRevokeSessions_AuthDisabled_RejectsAnonymousCaller(t *testing.T) {
	service, _ := newAuthDisabledService(t)
	handler := NewHandler(service)

	routes := map[string]http.HandlerFunc{
		"revoke-others": RequireRealUser(service, handler.HandleRevokeOtherSessions),
		"revoke-all":    RequireRealUser(service, handler.HandleRevokeAllSessions),
		"revoke-one":    RequireRealUser(service, handler.HandleRevokeSession),
	}

	for name, route := range routes {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			route(w, httptest.NewRequest(http.MethodPost, "/api/auth/sessions/"+name, nil))
			if w.Code != http.StatusUnauthorized {
				t.Errorf("got %d, want 401", w.Code)
			}
		})
	}
}

// The one credential check that must survive auth-disabled mode: it is how
// the owner proves themselves in order to turn login back on. It carries its
// own knowledge factor and is rate limited.
func TestVerifyPassword_AuthDisabled_StillChecksTheOwnersPassword(t *testing.T) {
	service, _ := newAuthDisabledService(t)
	handler := NewHandler(service)
	route := RequireUser(service, handler.HandleVerifyPassword)

	wrong := httptest.NewRecorder()
	route(wrong, httptest.NewRequest(http.MethodPost, "/api/auth/verify-password",
		strings.NewReader(`{"password":"guess"}`)))
	if wrong.Code != http.StatusUnauthorized {
		t.Errorf("wrong password got %d, want 401", wrong.Code)
	}

	right := httptest.NewRecorder()
	route(right, httptest.NewRequest(http.MethodPost, "/api/auth/verify-password",
		strings.NewReader(`{"password":"ownerpassword"}`)))
	if right.Code != http.StatusNoContent {
		t.Errorf("correct password got %d, want 204", right.Code)
	}
}

// X-Forwarded-For is attacker-controlled unless a proxy is actually in front,
// and one implementation must serve both login and renewal so a session's
// recorded IP doesn't change format mid-life.
func TestClientIP(t *testing.T) {
	tests := []struct {
		name         string
		remoteAddr   string
		forwardedFor string
		trustProxy   bool
		want         string
	}{
		{"strips the port", "10.0.0.7:54321", "", false, "10.0.0.7"},
		{"ignores the header by default", "10.0.0.7:54321", "1.2.3.4", false, "10.0.0.7"},
		{"honours the header behind a trusted proxy", "10.0.0.7:54321", "1.2.3.4", true, "1.2.3.4"},
		{"takes the first hop", "10.0.0.7:54321", "1.2.3.4, 5.6.7.8", true, "1.2.3.4"},
		{"falls back on an unparsable RemoteAddr", "unix-socket", "", false, "unix-socket"},
		{"ipv6 loses the port", "[::1]:8080", "", false, "::1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.trustProxy {
				t.Setenv(TrustedProxyEnv, "true")
			} else {
				t.Setenv(TrustedProxyEnv, "")
			}
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.remoteAddr
			if tc.forwardedFor != "" {
				r.Header.Set("X-Forwarded-For", tc.forwardedFor)
			}
			if got := clientIP(r); got != tc.want {
				t.Errorf("clientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}

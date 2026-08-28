package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// adminSession seeds an admin and returns a cookie for a live session.
func adminSession(t *testing.T, handler *Handler, repo *fakeRepository) *http.Cookie {
	t.Helper()
	repo.seedUser(1, "owner", "ownerpassword", true)
	return loginCookie(t, handler, "owner", "ownerpassword")
}

func TestHandleCreateUser_DoesNotLeakDriverErrors(t *testing.T) {
	handler, repo := newTestHandler()
	cookie := adminSession(t, handler, repo)

	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"username":"owner","password":"secret123"}`))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	RequireRealAdminUser(handler.service, handler.HandleCreateUser)(w, req)

	if strings.Contains(w.Body.String(), "UNIQUE constraint") {
		t.Errorf("driver error text reached the client: %s", w.Body.String())
	}
}

// Admin-facing endpoints must go through the same validation rules as the
// self-service ones — one source of truth, enforced in the service.
func TestHandleCreateUser_AppliesCentralValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"username too short", `{"username":"a","password":"secret123"}`},
		{"password too long", `{"username":"carol","password":"` + strings.Repeat("p", MaxPasswordLength+1) + `"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler, repo := newTestHandler()
			cookie := adminSession(t, handler, repo)

			req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(tc.body))
			req.AddCookie(cookie)
			w := httptest.NewRecorder()
			RequireRealAdminUser(handler.service, handler.HandleCreateUser)(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("got %d, want 400: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandleResetUserPassword_AppliesCentralValidation(t *testing.T) {
	handler, repo := newTestHandler()
	cookie := adminSession(t, handler, repo)
	repo.seedUser(2, "bob", "bobpassword", false)

	body := `{"new_password":"` + strings.Repeat("p", MaxPasswordLength+1) + `"}`
	req := httptest.NewRequest(http.MethodPut, "/api/users/2/password", strings.NewReader(body))
	req.SetPathValue("id", "2")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	RequireRealAdminUser(handler.service, handler.HandleResetUserPassword)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400: %s", w.Code, w.Body.String())
	}
}

// Changing the password logs every other device out but keeps the caller's
// tab signed in, on a brand-new session id.
func TestHandleUpdatePassword_RotatesTheCallersSessionAndRevokesTheRest(t *testing.T) {
	handler, repo := newTestHandler()
	repo.seedUser(1, "alice", "alicepass123", false)
	cookie := loginCookie(t, handler, "alice", "alicepass123")
	otherDevice := loginCookie(t, handler, "alice", "alicepass123")

	req := httptest.NewRequest(http.MethodPut, "/api/auth/password", strings.NewReader(
		`{"currentPassword":"alicepass123","newPassword":"alicepass456"}`))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	RequireUser(handler.service, handler.HandleUpdatePassword)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}

	var replacement *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == SessionCookieName {
			replacement = c
		}
	}
	if replacement == nil || replacement.Value == "" {
		t.Fatal("expected a replacement session cookie")
	}
	if replacement.Value == cookie.Value {
		t.Error("expected a new session id, got the old one back")
	}

	// The other device is out.
	otherReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	otherReq.AddCookie(otherDevice)
	otherW := httptest.NewRecorder()
	RequireUser(handler.service, handler.HandleMe)(otherW, otherReq)
	if otherW.Code != http.StatusUnauthorized {
		t.Errorf("other device still authenticated: got %d, want 401", otherW.Code)
	}

	// The caller is still in.
	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(replacement)
	meW := httptest.NewRecorder()
	RequireUser(handler.service, handler.HandleMe)(meW, meReq)
	if meW.Code != http.StatusOK {
		t.Errorf("caller was logged out of their own tab: got %d, want 200", meW.Code)
	}
}

func TestHandleLogin_RateLimitsRepeatedFailures(t *testing.T) {
	handler, repo := newTestHandler()
	repo.seedUser(1, "alice", "alicepass123", false)

	for i := 0; i < CredentialFailureLimit; i++ {
		w := doJSON(t, handler.HandleLogin, http.MethodPost, "/api/auth/login", LoginRequest{
			Username: "alice",
			Password: "wrongpassword",
		})
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d, want 401", i, w.Code)
		}
	}

	blocked := doJSON(t, handler.HandleLogin, http.MethodPost, "/api/auth/login", LoginRequest{
		Username: "alice",
		Password: "wrongpassword",
	})
	if blocked.Code != http.StatusTooManyRequests {
		t.Errorf("got %d, want 429 after %d failures", blocked.Code, CredentialFailureLimit)
	}

	// Even the right password stays blocked while the window holds.
	correct := doJSON(t, handler.HandleLogin, http.MethodPost, "/api/auth/login", LoginRequest{
		Username: "alice",
		Password: "alicepass123",
	})
	if correct.Code != http.StatusTooManyRequests {
		t.Errorf("got %d, want 429 while rate limited", correct.Code)
	}
}

func TestHandleLogin_SuccessClearsTheRateLimit(t *testing.T) {
	handler, repo := newTestHandler()
	repo.seedUser(1, "alice", "alicepass123", false)

	for i := 0; i < CredentialFailureLimit-1; i++ {
		doJSON(t, handler.HandleLogin, http.MethodPost, "/api/auth/login", LoginRequest{
			Username: "alice",
			Password: "wrongpassword",
		})
	}
	if w := doJSON(t, handler.HandleLogin, http.MethodPost, "/api/auth/login", LoginRequest{
		Username: "alice",
		Password: "alicepass123",
	}); w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}

	// A fresh typo must not be the one that trips the limit.
	if w := doJSON(t, handler.HandleLogin, http.MethodPost, "/api/auth/login", LoginRequest{
		Username: "alice",
		Password: "wrongpassword",
	}); w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401 after a successful login reset the counter", w.Code)
	}
}

func TestHandleConfirmPasswordReset_RateLimitsGuessing(t *testing.T) {
	handler, repo := newTestHandler()
	user := repo.seedUser(1, "alice", "alicepass123", false)
	if err := repo.StoreResetCode("ABCD1234", user.ID, time.Now().Add(ResetCodeTTL)); err != nil {
		t.Fatalf("seed reset code: %v", err)
	}

	for i := 0; i < CredentialFailureLimit; i++ {
		doJSON(t, handler.HandleConfirmPasswordReset, http.MethodPost, "/api/auth/password/reset/confirm", ConfirmPasswordResetRequest{
			Code:        "WRONG123",
			NewPassword: "newpassword",
		})
	}

	w := doJSON(t, handler.HandleConfirmPasswordReset, http.MethodPost, "/api/auth/password/reset/confirm", ConfirmPasswordResetRequest{
		Code:        "ABCD1234",
		NewPassword: "newpassword",
	})
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("got %d, want 429 after %d guesses", w.Code, CredentialFailureLimit)
	}
}

// Flooding one credential endpoint must not lock an account out of another.
func TestRateLimitScopes_ResetRequestsDoNotBlockLogin(t *testing.T) {
	t.Setenv("AUDIOD_DATA_DIR", t.TempDir())
	handler, repo := newTestHandler()
	repo.seedUser(1, "alice", "alicepass123", false)

	for i := 0; i <= CredentialFailureLimit; i++ {
		doJSON(t, handler.HandleRequestPasswordReset, http.MethodPost, "/api/auth/password/reset/request", RequestPasswordResetRequest{
			Username: "alice",
		})
	}

	w := doJSON(t, handler.HandleLogin, http.MethodPost, "/api/auth/login", LoginRequest{
		Username: "alice",
		Password: "alicepass123",
	})
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200 — reset-request floods must not block login", w.Code)
	}
}

// The 200 body used to hand an anonymous caller the server's absolute data
// directory.
func TestHandleRequestPasswordReset_DoesNotDiscloseTheDataDir(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("AUDIOD_DATA_DIR", dataDir)
	handler, repo := newTestHandler()
	repo.seedUser(1, "alice", "alicepass123", false)

	w := doJSON(t, handler.HandleRequestPasswordReset, http.MethodPost, "/api/auth/password/reset/request", RequestPasswordResetRequest{
		Username: "alice",
	})

	if strings.Contains(w.Body.String(), dataDir) {
		t.Errorf("response disclosed the data directory: %s", w.Body.String())
	}
}

// Expired rows must not show up as active devices even before the cleanup job
// gets to them.
func TestHandleListSessions_HidesExpiredSessions(t *testing.T) {
	repo := newFakeRepository()
	repo.seedUser(1, "alice", "alicepass123", false)
	sessions := NewInMemorySessionRepository()
	service := NewService(repo, sessions)
	handler := NewHandler(service)

	live, err := service.CreateSession(1, SessionContext{})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	stale := &Session{
		ID:         "stale-session",
		UserID:     1,
		CreatedAt:  time.Now().Add(-100 * 24 * time.Hour),
		LastSeenAt: time.Now().Add(-100 * 24 * time.Hour),
		ExpiresAt:  time.Now().Add(-24 * time.Hour),
	}
	if err := sessions.Create(stale); err != nil {
		t.Fatalf("seed stale session: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/sessions", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: live})
	w := httptest.NewRecorder()
	RequireUser(handler.service, handler.HandleListSessions)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}
	var got []SessionInfo
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only the live session, got %d", len(got))
	}
	if got[0].PublicID == SessionPublicID(stale.ID) {
		t.Error("an expired session was listed as an active device")
	}
}

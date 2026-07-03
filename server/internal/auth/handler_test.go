package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeRepository is an in-memory Repository fake for handler tests. It's
// deliberately minimal: enough behavior to drive the handlers, with error
// injection knobs for the paths that need to fail.
type fakeRepository struct {
	usersByUsername map[string]*User
	usersByID       map[int64]*User
	resetCodes      map[string]*ResetCode
	nextID          int64
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		usersByUsername: map[string]*User{},
		usersByID:       map[int64]*User{},
		resetCodes:      map[string]*ResetCode{},
	}
}

// seedUser inserts a user with an already-hashed password, bypassing
// Create's ID assignment so tests can pick predictable IDs.
func (r *fakeRepository) seedUser(id int64, username, password string, isAdmin bool) *User {
	hash, err := HashPassword(password)
	if err != nil {
		panic(err)
	}
	u := &User{ID: id, Username: username, PasswordHash: hash, IsAdmin: isAdmin, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	r.usersByUsername[username] = u
	r.usersByID[id] = u
	if id >= r.nextID {
		r.nextID = id + 1
	}
	return u
}

func (r *fakeRepository) GetByUsername(username string) (*User, error) {
	if u, ok := r.usersByUsername[username]; ok {
		return u, nil
	}
	return nil, ErrUserNotFound
}

func (r *fakeRepository) GetByID(id int64) (*User, error) {
	if u, ok := r.usersByID[id]; ok {
		return u, nil
	}
	return nil, ErrUserNotFound
}

func (r *fakeRepository) GetUserCount() (int, error) {
	return len(r.usersByID), nil
}

func (r *fakeRepository) GetAdminCount() (int, error) {
	n := 0
	for _, u := range r.usersByID {
		if u.IsAdmin {
			n++
		}
	}
	return n, nil
}

func (r *fakeRepository) GetFirstAdmin() (*User, error) {
	var lowest *User
	for _, u := range r.usersByID {
		if !u.IsAdmin {
			continue
		}
		if lowest == nil || u.ID < lowest.ID {
			lowest = u
		}
	}
	if lowest == nil {
		return nil, ErrUserNotFound
	}
	return lowest, nil
}

func (r *fakeRepository) ListUsers() ([]*User, error) {
	out := make([]*User, 0, len(r.usersByID))
	for _, u := range r.usersByID {
		out = append(out, u)
	}
	return out, nil
}

func (r *fakeRepository) Create(username, passwordHash string, isAdmin bool) (*User, error) {
	if _, exists := r.usersByUsername[username]; exists {
		return nil, fmt.Errorf("username already exists")
	}
	id := r.nextID
	r.nextID++
	u := &User{ID: id, Username: username, PasswordHash: passwordHash, IsAdmin: isAdmin, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	r.usersByUsername[username] = u
	r.usersByID[id] = u
	return u, nil
}

func (r *fakeRepository) Delete(userID int64) error {
	u, ok := r.usersByID[userID]
	if !ok {
		return ErrUserNotFound
	}
	delete(r.usersByID, userID)
	delete(r.usersByUsername, u.Username)
	return nil
}

func (r *fakeRepository) UpdatePassword(userID int64, newPasswordHash string) error {
	u, ok := r.usersByID[userID]
	if !ok {
		return ErrUserNotFound
	}
	u.PasswordHash = newPasswordHash
	return nil
}

func (r *fakeRepository) StoreResetCode(code string, userID int64, expiresAt time.Time) error {
	r.resetCodes[code] = &ResetCode{Code: code, UserID: userID, ExpiresAt: expiresAt, CreatedAt: time.Now()}
	return nil
}

func (r *fakeRepository) GetResetCode(code string) (*ResetCode, error) {
	rc, ok := r.resetCodes[code]
	if !ok {
		return nil, ErrInvalidResetCode
	}
	return rc, nil
}

func (r *fakeRepository) DeleteResetCode(code string) error {
	delete(r.resetCodes, code)
	return nil
}

func (r *fakeRepository) DeleteResetCodesByUserID(userID int64) error {
	for code, rc := range r.resetCodes {
		if rc.UserID == userID {
			delete(r.resetCodes, code)
		}
	}
	return nil
}

func (r *fakeRepository) DeleteExpiredResetCodes() error {
	now := time.Now()
	for code, rc := range r.resetCodes {
		if now.After(rc.ExpiresAt) {
			delete(r.resetCodes, code)
		}
	}
	return nil
}

// newTestHandler wires a Handler against a fresh fake repo + in-memory
// session store. Returns the handler and the repo so tests can seed users
// and inspect post-conditions.
func newTestHandler() (*Handler, *fakeRepository) {
	repo := newFakeRepository()
	service := NewService(repo, NewInMemorySessionRepository())
	return NewHandler(service), repo
}

func doJSON(t *testing.T, handler http.HandlerFunc, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

// loginCookie logs in via the handler and returns the audiod_token cookie
// so subsequent authenticated requests can attach it.
func loginCookie(t *testing.T, handler *Handler, username, password string) *http.Cookie {
	t.Helper()
	w := doJSON(t, handler.HandleLogin, http.MethodPost, "/api/auth/login", LoginRequest{
		Username: username,
		Password: password,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("login setup failed: status %d, body %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "audiod_token" {
			return c
		}
	}
	t.Fatal("login response did not set audiod_token cookie")
	return nil
}

func TestHandleLogin(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		handler, repo := newTestHandler()
		repo.seedUser(1, "alice", "alicepass123", false)

		w := doJSON(t, handler.HandleLogin, http.MethodPost, "/api/auth/login", LoginRequest{
			Username: "alice",
			Password: "alicepass123",
		})

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp LoginResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.User.Username != "alice" {
			t.Errorf("expected username alice, got %q", resp.User.Username)
		}
		if resp.Token == "" {
			t.Error("expected a non-empty session token")
		}
		found := false
		for _, c := range w.Result().Cookies() {
			if c.Name == "audiod_token" {
				found = true
			}
		}
		if !found {
			t.Error("expected audiod_token cookie to be set")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		handler, repo := newTestHandler()
		repo.seedUser(1, "alice", "alicepass123", false)

		w := doJSON(t, handler.HandleLogin, http.MethodPost, "/api/auth/login", LoginRequest{
			Username: "alice",
			Password: "wrongpassword",
		})

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("unknown user", func(t *testing.T) {
		handler, _ := newTestHandler()

		w := doJSON(t, handler.HandleLogin, http.MethodPost, "/api/auth/login", LoginRequest{
			Username: "nosuchuser",
			Password: "anypassword",
		})

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("legacy password shorter than current minimum still logs in", func(t *testing.T) {
		// Given a user whose password predates the 8-char minimum
		handler, repo := newTestHandler()
		repo.seedUser(1, "alice", "short", false)

		w := doJSON(t, handler.HandleLogin, http.MethodPost, "/api/auth/login", LoginRequest{
			Username: "alice",
			Password: "short",
		})

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		handler, _ := newTestHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
		w := httptest.NewRecorder()
		handler.HandleLogin(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestHandleSetup(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		handler, repo := newTestHandler()

		w := doJSON(t, handler.HandleSetup, http.MethodPost, "/api/auth/setup", SetupRequest{
			Username: "alice",
			Password: "alicepass123",
		})

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp SetupResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if !resp.User.IsAdmin {
			t.Error("expected first user to be admin")
		}
		if _, err := repo.GetByUsername("alice"); err != nil {
			t.Errorf("expected user to be persisted: %v", err)
		}
	})

	t.Run("validation error per rule: username too short", func(t *testing.T) {
		handler, _ := newTestHandler()

		w := doJSON(t, handler.HandleSetup, http.MethodPost, "/api/auth/setup", SetupRequest{
			Username: "a",
			Password: "alicepass123",
		})

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("validation error per rule: password too long", func(t *testing.T) {
		handler, _ := newTestHandler()

		w := doJSON(t, handler.HandleSetup, http.MethodPost, "/api/auth/setup", SetupRequest{
			Username: "alice",
			Password: makeString(MaxPasswordLength + 1),
		})

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("short password is accepted (warn, don't block)", func(t *testing.T) {
		handler, _ := newTestHandler()

		w := doJSON(t, handler.HandleSetup, http.MethodPost, "/api/auth/setup", SetupRequest{
			Username: "alice",
			Password: "p",
		})

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("duplicate setup is rejected once a user exists", func(t *testing.T) {
		handler, repo := newTestHandler()
		repo.seedUser(1, "existing", "existingpass1", true)

		w := doJSON(t, handler.HandleSetup, http.MethodPost, "/api/auth/setup", SetupRequest{
			Username: "alice",
			Password: "alicepass123",
		})

		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestHandleConfirmPasswordReset(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		handler, repo := newTestHandler()
		user := repo.seedUser(1, "alice", "alicepass123", false)
		if err := repo.StoreResetCode("ABCD1234", user.ID, time.Now().Add(30*time.Minute)); err != nil {
			t.Fatalf("seed reset code: %v", err)
		}

		w := doJSON(t, handler.HandleConfirmPasswordReset, http.MethodPost, "/api/auth/password/reset/confirm", ConfirmPasswordResetRequest{
			Code:        "ABCD1234",
			NewPassword: "newpassalice123",
		})

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		updated, err := repo.GetByID(user.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		valid, err := VerifyPassword("newpassalice123", updated.PasswordHash)
		if err != nil || !valid {
			t.Errorf("expected password to be updated, verify: valid=%v err=%v", valid, err)
		}
	})

	t.Run("invalid or expired code", func(t *testing.T) {
		handler, _ := newTestHandler()

		w := doJSON(t, handler.HandleConfirmPasswordReset, http.MethodPost, "/api/auth/password/reset/confirm", ConfirmPasswordResetRequest{
			Code:        "NOTFOUND",
			NewPassword: "newpassalice123",
		})

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("new password above maximum length", func(t *testing.T) {
		handler, repo := newTestHandler()
		user := repo.seedUser(1, "alice", "alicepass123", false)
		if err := repo.StoreResetCode("ABCD1234", user.ID, time.Now().Add(30*time.Minute)); err != nil {
			t.Fatalf("seed reset code: %v", err)
		}

		w := doJSON(t, handler.HandleConfirmPasswordReset, http.MethodPost, "/api/auth/password/reset/confirm", ConfirmPasswordResetRequest{
			Code:        "ABCD1234",
			NewPassword: makeString(MaxPasswordLength + 1),
		})

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("short new password is accepted (warn, don't block)", func(t *testing.T) {
		handler, repo := newTestHandler()
		user := repo.seedUser(1, "alice", "alicepass123", false)
		if err := repo.StoreResetCode("ABCD1234", user.ID, time.Now().Add(30*time.Minute)); err != nil {
			t.Fatalf("seed reset code: %v", err)
		}

		w := doJSON(t, handler.HandleConfirmPasswordReset, http.MethodPost, "/api/auth/password/reset/confirm", ConfirmPasswordResetRequest{
			Code:        "ABCD1234",
			NewPassword: "p",
		})

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestHandleConfirmPasswordReset_CodeLengthValidation(t *testing.T) {
	handler, _ := newTestHandler()

	w := doJSON(t, handler.HandleConfirmPasswordReset, http.MethodPost, "/api/auth/password/reset/confirm", ConfirmPasswordResetRequest{
		Code:        "TOOLONGCODE",
		NewPassword: "newpassalice123",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for wrong-length reset code, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleRequestPasswordReset(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		t.Setenv("AUDIOD_DATA_DIR", t.TempDir())
		handler, repo := newTestHandler()
		repo.seedUser(1, "alice", "alicepass123", false)

		w := doJSON(t, handler.HandleRequestPasswordReset, http.MethodPost, "/api/auth/password/reset/request", RequestPasswordResetRequest{
			Username: "alice",
		})

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp RequestPasswordResetResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Username != "alice" {
			t.Errorf("expected username alice, got %q", resp.Username)
		}
		if len(repo.resetCodes) != 1 {
			t.Errorf("expected exactly one reset code to be stored, got %d", len(repo.resetCodes))
		}
	})

	t.Run("missing username", func(t *testing.T) {
		handler, _ := newTestHandler()

		w := doJSON(t, handler.HandleRequestPasswordReset, http.MethodPost, "/api/auth/password/reset/request", RequestPasswordResetRequest{
			Username: "",
		})

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestHandleLogout(t *testing.T) {
	t.Run("deletes the server-side session", func(t *testing.T) {
		handler, repo := newTestHandler()
		repo.seedUser(1, "alice", "alicepass123", false)
		cookie := loginCookie(t, handler, "alice", "alicepass123")

		req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		handler.HandleLogout(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		// The session should now be gone: HandleMe with the same cookie 401s.
		meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		meReq.AddCookie(cookie)
		meW := httptest.NewRecorder()
		handler.HandleMe(meW, meReq)
		if meW.Code != http.StatusUnauthorized {
			t.Errorf("expected session to be invalidated after logout, /me returned %d", meW.Code)
		}
	})

	t.Run("is idempotent without a cookie", func(t *testing.T) {
		handler, _ := newTestHandler()

		req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		w := httptest.NewRecorder()
		handler.HandleLogout(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})
}

func TestHandleMe(t *testing.T) {
	t.Run("authenticated", func(t *testing.T) {
		handler, repo := newTestHandler()
		repo.seedUser(1, "alice", "alicepass123", false)
		cookie := loginCookie(t, handler, "alice", "alicepass123")

		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		handler.HandleMe(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp MeResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.User.Username != "alice" {
			t.Errorf("expected username alice, got %q", resp.User.Username)
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		handler, _ := newTestHandler()

		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		w := httptest.NewRecorder()
		handler.HandleMe(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})
}

func TestHandleListSessions(t *testing.T) {
	t.Run("authenticated user sees their own session", func(t *testing.T) {
		handler, repo := newTestHandler()
		repo.seedUser(1, "alice", "alicepass123", false)
		cookie := loginCookie(t, handler, "alice", "alicepass123")

		req := httptest.NewRequest(http.MethodGet, "/api/auth/sessions", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		handler.HandleListSessions(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var sessions []SessionInfo
		if err := json.Unmarshal(w.Body.Bytes(), &sessions); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(sessions))
		}
		if !sessions[0].IsCurrent {
			t.Error("expected the session backing the request cookie to be flagged current")
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		handler, _ := newTestHandler()

		req := httptest.NewRequest(http.MethodGet, "/api/auth/sessions", nil)
		w := httptest.NewRecorder()
		handler.HandleListSessions(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})
}

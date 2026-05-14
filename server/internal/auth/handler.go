package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// Password validation. MinPasswordLength is intentionally 1 (non-empty).
	// Policy: warn, don't block — any non-empty password is accepted, and the
	// UI surfaces a non-blocking warning for short / equals-username / etc.
	// Login rate limiting (deferred, see roadmap) is the proper defense
	// against brute force, not a length minimum.
	MinPasswordLength = 1
	MaxPasswordLength = 64

	// Username validation
	MinUsernameLength = 2
	MaxUsernameLength = 32
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all auth routes on the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/setup/required", h.HandleSetupRequired)
	mux.HandleFunc("POST /api/auth/setup", h.HandleSetup)
	mux.HandleFunc("POST /api/auth/login", h.HandleLogin)
	mux.HandleFunc("POST /api/auth/logout", h.HandleLogout)
	mux.HandleFunc("GET /api/auth/me", h.HandleMe)
	mux.HandleFunc("PUT /api/auth/password", h.HandleUpdatePassword)
	mux.HandleFunc("POST /api/auth/password/reset/request", h.HandleRequestPasswordReset)
	mux.HandleFunc("POST /api/auth/password/reset/confirm", h.HandleConfirmPasswordReset)
	mux.HandleFunc("POST /api/auth/verify-password", h.HandleVerifyPassword)

	// Active sessions (Settings → Security tab).
	mux.HandleFunc("GET /api/auth/sessions", h.HandleListSessions)
	mux.HandleFunc("DELETE /api/auth/sessions/{publicId}", h.HandleRevokeSession)
	mux.HandleFunc("POST /api/auth/sessions/revoke-others", h.HandleRevokeOtherSessions)
	mux.HandleFunc("POST /api/auth/sessions/revoke-all", h.HandleRevokeAllSessions)

	// User management (Settings → Users tab). Admin-only across the board.
	mux.HandleFunc("GET /api/users", h.HandleListUsers)
	mux.HandleFunc("POST /api/users", h.HandleCreateUser)
	mux.HandleFunc("DELETE /api/users/{id}", h.HandleDeleteUser)
	mux.HandleFunc("PUT /api/users/{id}/password", h.HandleResetUserPassword)
}

type LoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"rememberMe"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// sessionContextFromRequest captures user-agent + best-effort client IP for
// recording on the session row. X-Forwarded-For (first hop) wins if present,
// so reverse-proxied deployments record the real client.
func sessionContextFromRequest(r *http.Request, rememberMe bool) SessionContext {
	ip := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if comma := strings.Index(forwarded, ","); comma >= 0 {
			ip = strings.TrimSpace(forwarded[:comma])
		} else {
			ip = strings.TrimSpace(forwarded)
		}
	}
	return SessionContext{
		RememberMe: rememberMe,
		UserAgent:  r.Header.Get("User-Agent"),
		IP:         ip,
	}
}

// setSessionCookie writes the audiod_token cookie with the correct lifetime
// for the session's RememberMe flag. Centralised so login + setup stay in
// sync. Secure flag stays false here — auto-detection from request transport
// lands in a separate slice.
func setSessionCookie(w http.ResponseWriter, value string, rememberMe bool) {
	window := SessionWindowDefault
	if rememberMe {
		window = SessionWindowRemember
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "audiod_token",
		Value:    value,
		Path:     "/",
		MaxAge:   int(window.Seconds()),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie expires the audiod_token cookie immediately. Used on
// logout once the server-side session row has been deleted.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "audiod_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})
}

// HandleLogin handles POST /api/auth/login
// @Summary User login
// @Description Authenticate user and return JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Router /auth/login [post]
// @ID login
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	if len(req.Username) < MinUsernameLength || len(req.Username) > MaxUsernameLength {
		http.Error(w, "Username must be between 2 and 32 characters", http.StatusBadRequest)
		return
	}

	if len(req.Password) < MinPasswordLength || len(req.Password) > MaxPasswordLength {
		http.Error(w, "Password must be at most 64 characters", http.StatusBadRequest)
		return
	}

	// Attempt login — opens a DB-backed session and returns the opaque cookie value.
	sessionID, user, err := h.service.Login(req.Username, req.Password, sessionContextFromRequest(r, req.RememberMe))
	if err != nil {
		log.Printf("Login failed for user %s: %v", req.Username, err)
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	setSessionCookie(w, sessionID, req.RememberMe)

	response := LoginResponse{
		Token: sessionID,
		User:  user.ToResponse(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// HandleLogout handles POST /api/auth/logout
// @Summary User logout
// @Description Clear authentication cookie
// @Tags auth
// @Success 200 {string} string "OK"
// @Router /auth/logout [post]
// @ID logout
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Delete the server-side session row before clearing the cookie. Missing
	// or unknown cookies are not an error — logout is idempotent.
	if cookie, err := r.Cookie("audiod_token"); err == nil {
		if err := h.service.DeleteSession(cookie.Value); err != nil {
			log.Printf("Failed to delete session on logout: %v", err)
		}
	}

	clearSessionCookie(w)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

type MeResponse struct {
	User UserResponse `json:"user"`
}

// HandleMe handles GET /api/auth/me
// @Summary Get current user
// @Description Returns the currently authenticated user from JWT cookie
// @Tags auth
// @Produce json
// @Success 200 {object} MeResponse
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/me [get]
// @ID getMe
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Resolve the session cookie to a user. Unknown / expired → 401.
	user, err := userFromSessionCookie(w, r, h.service)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	response := MeResponse{
		User: user.ToResponse(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// HandleUpdatePassword handles PUT /api/auth/password
// @Summary Update user password
// @Description Update the current user's password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body UpdatePasswordRequest true "Password update request"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/password [put]
// @ID updatePassword
func (h *Handler) HandleUpdatePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Resolve the session cookie to a user.
	user, err := userFromSessionCookie(w, r, h.service)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var req UpdatePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, "Current password and new password are required", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < MinPasswordLength || len(req.NewPassword) > MaxPasswordLength {
		http.Error(w, "Password must be at most 64 characters", http.StatusBadRequest)
		return
	}

	// Update password
	err = h.service.UpdatePassword(user.ID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		if err == ErrInvalidPassword {
			http.Error(w, "Current password is incorrect", http.StatusUnauthorized)
			return
		}
		log.Printf("Failed to update password for user %d: %v", user.ID, err)
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

type RequestPasswordResetRequest struct {
	Username string `json:"username"`
}

type RequestPasswordResetResponse struct {
	FilePath string `json:"filePath"`
	Username string `json:"username"`
}

// HandleRequestPasswordReset handles POST /api/auth/password/reset/request
// @Summary Request password reset
// @Description Generates a reset code and writes it to a file on the server
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RequestPasswordResetRequest true "Username"
// @Success 200 {object} RequestPasswordResetResponse
// @Failure 400 {string} string "Bad Request"
// @Failure 500 {string} string "Internal Server Error"
// @Router /auth/password/reset/request [post]
// @ID requestPasswordReset
func (h *Handler) HandleRequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req RequestPasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate username
	if req.Username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	// Generate reset code and write to file
	filePath, username, err := h.service.RequestPasswordResetWithFile(req.Username)
	if err != nil {
		log.Printf("Failed to generate reset code for user %s: %v", req.Username, err)
		http.Error(w, "Failed to generate reset code", http.StatusInternalServerError)
		return
	}

	response := RequestPasswordResetResponse{
		FilePath: filePath,
		Username: username,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

type ConfirmPasswordResetRequest struct {
	Code        string `json:"code"`
	NewPassword string `json:"newPassword"`
}

type ConfirmPasswordResetResponse struct {
	User UserResponse `json:"user"`
}

// HandleConfirmPasswordReset handles POST /api/auth/password/reset/confirm
// @Summary Confirm password reset
// @Description Validates reset code and resets user to default credentials
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ConfirmPasswordResetRequest true "Reset code"
// @Success 200 {object} ConfirmPasswordResetResponse
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized - Invalid or expired code"
// @Failure 500 {string} string "Internal Server Error"
// @Router /auth/password/reset/confirm [post]
// @ID confirmPasswordReset
func (h *Handler) HandleConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req ConfirmPasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate code format
	if req.Code == "" {
		http.Error(w, "Reset code is required", http.StatusBadRequest)
		return
	}

	if len(req.Code) != 8 {
		http.Error(w, "Invalid reset code format", http.StatusBadRequest)
		return
	}

	// Validate new password
	if req.NewPassword == "" {
		http.Error(w, "New password is required", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < MinPasswordLength || len(req.NewPassword) > MaxPasswordLength {
		http.Error(w, "Password must be at most 64 characters", http.StatusBadRequest)
		return
	}

	// Confirm reset
	user, err := h.service.ConfirmPasswordReset(req.Code, req.NewPassword)
	if err != nil {
		if err == ErrInvalidResetCode {
			http.Error(w, "Invalid or expired reset code", http.StatusUnauthorized)
			return
		}
		log.Printf("Failed to confirm password reset: %v", err)
		http.Error(w, "Failed to reset password", http.StatusInternalServerError)
		return
	}

	response := ConfirmPasswordResetResponse{
		User: user.ToResponse(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

type SetupRequiredResponse struct {
	Required bool `json:"required"`
}

// HandleSetupRequired handles GET /api/auth/setup/required
// @Summary Check if initial setup is required
// @Description Returns whether the system requires initial setup (no users exist)
// @Tags auth
// @Produce json
// @Success 200 {object} SetupRequiredResponse
// @Router /auth/setup/required [get]
// @ID isSetupRequired
func (h *Handler) HandleSetupRequired(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	count, err := h.service.GetUserCount()
	if err != nil {
		log.Printf("Failed to get user count: %v", err)
		http.Error(w, "Failed to check setup status", http.StatusInternalServerError)
		return
	}

	response := SetupRequiredResponse{
		Required: count == 0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// userFromSessionCookie wraps AuthenticateRequest so HandleMe + HandleUpdate
// Password keep their concise inline auth-check call. Sliding renewal +
// cookie refresh happen inside AuthenticateRequest.
func userFromSessionCookie(w http.ResponseWriter, r *http.Request, service *Service) (*User, error) {
	return AuthenticateRequest(w, r, service)
}

type VerifyPasswordRequest struct {
	Password string `json:"password"`
}

// HandleVerifyPassword handles POST /api/auth/verify-password
// @Summary Verify the caller's password (sudo re-confirmation)
// @Description Re-checks the authenticated user's password without rotating
// @Description the session. Used by the sudo modal to gate sensitive ops:
// @Description on 204 the caller proceeds with the wrapped action; on 401
// @Description the modal stays open and shows an error.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body VerifyPasswordRequest true "Password to verify"
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/verify-password [post]
// @ID verifyPassword
func (h *Handler) HandleVerifyPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Auth check is the standard cookie path. Critically we DON'T issue a
	// new cookie or open a session here — verify is a "prove who you are
	// right now" probe, not a re-login.
	user, err := userFromSessionCookie(w, r, h.service)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req VerifyPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "Password is required", http.StatusBadRequest)
		return
	}

	// Match the same error mapping as Login: collapse "no such user" and
	// "wrong password" into a single 401 so the response shape doesn't leak
	// which one was incorrect (even though "the user is the cookie-holder"
	// is already known to the caller, keeping the contract uniform).
	if _, err := h.service.Authenticate(user.Username, req.Password); err != nil {
		http.Error(w, "Incorrect password", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// currentSessionID returns the raw session id from the request cookie, or
// "" if no cookie is present. Used by the session-management endpoints to
// flag the caller's "current" row and to spare it from bulk revoke.
func currentSessionID(r *http.Request) string {
	cookie, err := r.Cookie("audiod_token")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// SessionInfo is the JSON shape returned for each row in
// GET /api/auth/sessions. The raw session id is never exposed — only its
// public hash, so XSS cannot turn a list-sessions response into a stolen
// cookie even though the cookie itself is HttpOnly.
type SessionInfo struct {
	PublicID   string    `json:"publicId"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	UserAgent  string    `json:"userAgent"`
	LastIP     string    `json:"lastIp"`
	IsCurrent  bool      `json:"isCurrent"`
}

// HandleListSessions handles GET /api/auth/sessions
// @Summary List active sessions
// @Description Returns the caller's active browser sessions, with isCurrent flagging the row backing the request cookie.
// @Tags auth
// @Produce json
// @Success 200 {array} SessionInfo
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/sessions [get]
// @ID listSessions
func (h *Handler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	user, err := userFromSessionCookie(w, r, h.service)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessions, err := h.service.ListUserSessions(user.ID)
	if err != nil {
		log.Printf("List sessions failed for user %d: %v", user.ID, err)
		http.Error(w, "Failed to list sessions", http.StatusInternalServerError)
		return
	}

	cookieID := currentSessionID(r)
	out := make([]SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, SessionInfo{
			PublicID:   SessionPublicID(s.ID),
			CreatedAt:  s.CreatedAt,
			LastSeenAt: s.LastSeenAt,
			ExpiresAt:  s.ExpiresAt,
			UserAgent:  s.UserAgent,
			LastIP:     s.LastIP,
			IsCurrent:  s.ID == cookieID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Printf("Failed to encode sessions response: %v", err)
	}
}

// HandleRevokeSession handles DELETE /api/auth/sessions/{publicId}
// @Summary Revoke a single session
// @Description Deletes one of the caller's sessions by its public id. Used by the Active Devices "✕" button.
// @Tags auth
// @Param publicId path string true "Session public id"
// @Success 204 {string} string "No Content"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Not Found"
// @Router /auth/sessions/{publicId} [delete]
// @ID revokeSession
func (h *Handler) HandleRevokeSession(w http.ResponseWriter, r *http.Request) {
	user, err := userFromSessionCookie(w, r, h.service)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	publicID := r.PathValue("publicId")
	if publicID == "" {
		http.Error(w, "publicId is required", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteUserSessionByPublicID(user.ID, publicID); err != nil {
		if err == ErrSessionNotFound {
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}
		log.Printf("Revoke session failed for user %d: %v", user.ID, err)
		http.Error(w, "Failed to revoke session", http.StatusInternalServerError)
		return
	}

	// If the caller revoked their own current session, clear the cookie so
	// the browser doesn't keep sending a now-orphan value. The next request
	// would 401 either way; this just keeps the wire tidy.
	if SessionPublicID(currentSessionID(r)) == publicID {
		clearSessionCookie(w)
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleRevokeOtherSessions handles POST /api/auth/sessions/revoke-others
// @Summary Log out of all other devices
// @Description Deletes every session of the caller except the one backing the request cookie.
// @Tags auth
// @Success 204 {string} string "No Content"
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/sessions/revoke-others [post]
// @ID revokeOtherSessions
func (h *Handler) HandleRevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	user, err := userFromSessionCookie(w, r, h.service)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.service.DeleteOtherUserSessions(user.ID, currentSessionID(r)); err != nil {
		log.Printf("Revoke other sessions failed for user %d: %v", user.ID, err)
		http.Error(w, "Failed to revoke sessions", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleRevokeAllSessions handles POST /api/auth/sessions/revoke-all
// @Summary Log out of all devices
// @Description Deletes every session of the caller, including the current one. The response clears the auth cookie.
// @Tags auth
// @Success 204 {string} string "No Content"
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/sessions/revoke-all [post]
// @ID revokeAllSessions
func (h *Handler) HandleRevokeAllSessions(w http.ResponseWriter, r *http.Request) {
	user, err := userFromSessionCookie(w, r, h.service)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.service.DeleteAllUserSessions(user.ID); err != nil {
		log.Printf("Revoke all sessions failed for user %d: %v", user.ID, err)
		http.Error(w, "Failed to revoke sessions", http.StatusInternalServerError)
		return
	}

	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

type SetupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SetupResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// HandleSetup handles POST /api/auth/setup
// @Summary Create first user account
// @Description Creates the first user (admin) account and returns auth token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body SetupRequest true "First user credentials"
// @Success 200 {object} SetupResponse
// @Failure 409 {string} string "Setup already completed"
// @Router /auth/setup [post]
// @ID createFirstUser
func (h *Handler) HandleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	if len(req.Username) < MinUsernameLength || len(req.Username) > MaxUsernameLength {
		http.Error(w, "Username must be between 2 and 32 characters", http.StatusBadRequest)
		return
	}

	if len(req.Password) < MinPasswordLength || len(req.Password) > MaxPasswordLength {
		http.Error(w, "Password must be at most 64 characters", http.StatusBadRequest)
		return
	}

	// Create first user — also opens an initial session so the wizard's
	// "you're in" page works without a redirect through /login.
	sessionID, user, err := h.service.CreateFirstUser(req.Username, req.Password, sessionContextFromRequest(r, false))
	if err != nil {
		if err.Error() == "setup already completed: users already exist" {
			http.Error(w, "Setup already completed", http.StatusConflict)
			return
		}
		log.Printf("Failed to create first user: %v", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, sessionID, false)

	response := SetupResponse{
		Token: sessionID,
		User:  user.ToResponse(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// --- User management (admin actions on other users) ---

// requireAdmin checks the cookie + admin flag and surfaces the right HTTP
// status. Mirrors the pattern in settings.Handler but inline here to keep the
// auth handlers self-contained.
func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (*User, bool) {
	user, err := userFromSessionCookie(w, r, h.service)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	if err := RequireAdmin(user); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return nil, false
	}
	return user, true
}

type UsersListResponse struct {
	Users []UserResponse `json:"users"`
}

// HandleListUsers handles GET /api/users
// @Summary List all users
// @Description Lists every user in the system. Admin-only.
// @Tags users
// @Produce json
// @Success 200 {object} UsersListResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Router /users [get]
// @ID listUsers
func (h *Handler) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	users, err := h.service.ListUsers()
	if err != nil {
		log.Printf("ListUsers failed: %v", err)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}
	out := make([]UserResponse, 0, len(users))
	for _, u := range users {
		out = append(out, u.ToResponse())
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(UsersListResponse{Users: out}); err != nil {
		log.Printf("Encode users list failed: %v", err)
	}
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"is_admin"`
}

// HandleCreateUser handles POST /api/users
// @Summary Create a new user
// @Description Creates a new user account. Admin-only. Once the first admin
// @Description exists, every subsequent user is created through this endpoint
// @Description (the /auth/setup endpoint stops working after first-run).
// @Tags users
// @Accept json
// @Produce json
// @Param request body CreateUserRequest true "New user credentials"
// @Success 201 {object} UserResponse
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Router /users [post]
// @ID createUser
func (h *Handler) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}
	user, err := h.service.CreateUser(req.Username, req.Password, req.IsAdmin)
	if err != nil {
		log.Printf("CreateUser failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(user.ToResponse()); err != nil {
		log.Printf("Encode created user failed: %v", err)
	}
}

// HandleDeleteUser handles DELETE /api/users/{id}
// @Summary Delete a user
// @Description Removes a user row. Admin-only. Refuses to delete the
// @Description calling user or the system's last admin.
// @Tags users
// @Param id path int true "User id"
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Router /users/{id} [delete]
// @ID deleteUser
func (h *Handler) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
	if err := h.service.DeleteUser(actor.ID, id); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		// Self-delete and last-admin protection both surface here.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type ResetUserPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// HandleResetUserPassword handles PUT /api/users/{id}/password
// @Summary Reset another user's password
// @Description Admin-only password reset that bypasses the current-password
// @Description check (which the self-service /auth/password endpoint enforces).
// @Tags users
// @Accept json
// @Param id path int true "User id"
// @Param request body ResetUserPasswordRequest true "New password"
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Router /users/{id}/password [put]
// @ID resetUserPassword
func (h *Handler) HandleResetUserPassword(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
	var req ResetUserPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.NewPassword == "" {
		http.Error(w, "new_password is required", http.StatusBadRequest)
		return
	}
	if err := h.service.AdminResetUserPassword(id, req.NewPassword); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		log.Printf("AdminResetUserPassword failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

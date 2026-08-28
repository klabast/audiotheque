package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

type LoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"rememberMe"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type MeResponse struct {
	User UserResponse `json:"user"`
}

type SetupRequiredResponse struct {
	Required bool `json:"required"`
}

type SetupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SetupResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// HandleLogin handles POST /api/auth/login
// @Summary User login
// @Description Authenticate user and open a session
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 429 {string} string "Too Many Requests"
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

	// Login only checks presence: length rules apply where passwords are
	// set, and must never lock out accounts created under older rules.
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	keys := rateLimitKeys(scopeCredential, r, req.Username)
	if !h.allowCredentialAttempt(w, keys) {
		return
	}

	sessionID, user, err := h.service.Login(req.Username, req.Password, sessionContextFromRequest(r, req.RememberMe))
	if err != nil {
		h.limiter.RecordFailure(time.Now(), keys...)
		log.Printf("Login failed for user %s: %v", req.Username, err)
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	h.limiter.Reset(keys...)

	setSessionCookie(w, r, sessionID, req.RememberMe)

	writeJSON(w, http.StatusOK, LoginResponse{
		Token: sessionID,
		User:  user.ToResponse(),
	})
}

// HandleLogout handles POST /api/auth/logout
// @Summary User logout
// @Description Delete the session row and clear the auth cookie
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
	if id := currentSessionID(r); id != "" {
		if err := h.service.DeleteSession(id); err != nil {
			log.Printf("Failed to delete session on logout: %v", err)
		}
	}

	clearSessionCookie(w, r)

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Printf("Failed to write logout response: %v", err)
	}
}

// HandleMe handles GET /api/auth/me
// @Summary Get current user
// @Description Returns the currently authenticated user
// @Tags auth
// @Produce json
// @Success 200 {object} MeResponse
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/me [get]
// @ID getMe
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request, user *User) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, MeResponse{User: user.ToResponse()})
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

	writeJSON(w, http.StatusOK, SetupRequiredResponse{Required: count == 0})
}

// HandleSetup handles POST /api/auth/setup
// @Summary Create first user account
// @Description Creates the first user (admin) account and opens a session
// @Tags auth
// @Accept json
// @Produce json
// @Param request body SetupRequest true "First user credentials"
// @Success 200 {object} SetupResponse
// @Failure 400 {string} string "Bad Request"
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

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	if err := ValidateUsername(req.Username); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := ValidatePassword(req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create first user — also opens an initial session so the wizard's
	// "you're in" page works without a redirect through /login.
	sessionID, user, err := h.service.CreateFirstUser(req.Username, req.Password, sessionContextFromRequest(r, false))
	if err != nil {
		if errors.Is(err, ErrSetupAlreadyCompleted) || errors.Is(err, ErrUsernameTaken) {
			http.Error(w, "Setup already completed", http.StatusConflict)
			return
		}
		log.Printf("Failed to create first user: %v", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, r, sessionID, false)

	writeJSON(w, http.StatusOK, SetupResponse{
		Token: sessionID,
		User:  user.ToResponse(),
	})
}

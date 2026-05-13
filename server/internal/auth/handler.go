package auth

import (
	"encoding/json"
	"log"
	"net/http"
)

const (
	// Password validation
	MinPasswordLength = 8
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
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token             string `json:"token"`
	User              UserResponse `json:"user"`
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
		http.Error(w, "Password must be between 8 and 64 characters", http.StatusBadRequest)
		return
	}

	// Attempt login
	token, user, err := h.service.Login(req.Username, req.Password)
	if err != nil {
		log.Printf("Login failed for user %s: %v", req.Username, err)
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// Set httpOnly cookie with JWT token
	http.SetCookie(w, &http.Cookie{
		Name:     "audiod_token",
		Value:    token,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	})

	response := LoginResponse{
		Token: token,
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

	// Clear the cookie by setting MaxAge to -1
	http.SetCookie(w, &http.Cookie{
		Name:     "audiod_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

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

	// Get token from cookie
	cookie, err := r.Cookie("audiod_token")
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate token
	claims, err := ValidateToken(cookie.Value)
	if err != nil {
		log.Printf("Invalid token: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user from database
	user, err := h.service.GetUserByID(claims.UserID)
	if err != nil {
		log.Printf("Failed to get user %d: %v", claims.UserID, err)
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

	// Get token from cookie
	cookie, err := r.Cookie("audiod_token")
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate token and get user ID
	claims, err := ValidateToken(cookie.Value)
	if err != nil {
		log.Printf("Invalid token: %v", err)
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
		http.Error(w, "Password must be between 8 and 64 characters", http.StatusBadRequest)
		return
	}

	// Update password
	err = h.service.UpdatePassword(claims.UserID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		if err == ErrInvalidPassword {
			http.Error(w, "Current password is incorrect", http.StatusUnauthorized)
			return
		}
		log.Printf("Failed to update password for user %d: %v", claims.UserID, err)
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
		http.Error(w, "Password must be between 8 and 64 characters", http.StatusBadRequest)
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
		http.Error(w, "Password must be between 8 and 64 characters", http.StatusBadRequest)
		return
	}

	// Create first user
	token, user, err := h.service.CreateFirstUser(req.Username, req.Password)
	if err != nil {
		if err.Error() == "setup already completed: users already exist" {
			http.Error(w, "Setup already completed", http.StatusConflict)
			return
		}
		log.Printf("Failed to create first user: %v", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Set httpOnly cookie with JWT token (same as HandleLogin)
	http.SetCookie(w, &http.Cookie{
		Name:     "audiod_token",
		Value:    token,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	})

	response := SetupResponse{
		Token: token,
		User:  user.ToResponse(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

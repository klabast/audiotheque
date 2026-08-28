package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type RequestPasswordResetRequest struct {
	Username string `json:"username"`
}

type ConfirmPasswordResetRequest struct {
	Code        string `json:"code"`
	NewPassword string `json:"newPassword"`
}

type ConfirmPasswordResetResponse struct {
	User UserResponse `json:"user"`
}

type VerifyPasswordRequest struct {
	Password string `json:"password"`
}

// HandleUpdatePassword handles PUT /api/auth/password
// @Summary Update user password
// @Description Updates the caller's password. Every other session is revoked;
// @Description the calling browser is issued a fresh session cookie.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body UpdatePasswordRequest true "Password update request"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/password [put]
// @ID updatePassword
func (h *Handler) HandleUpdatePassword(w http.ResponseWriter, r *http.Request, user *User) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdatePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, "Current password and new password are required", http.StatusBadRequest)
		return
	}

	if err := ValidatePassword(req.NewPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// This endpoint verifies the current password, so it is a guessing oracle
	// like login is.
	keys := rateLimitKeys(scopeCredential, r, user.Username)
	if !h.allowCredentialAttempt(w, keys) {
		return
	}

	// Read the caller's "keep me logged in" choice before the update wipes
	// the session it's recorded on, so the replacement session keeps it.
	rememberMe := h.service.SessionRememberMe(currentSessionID(r))

	sessionID, err := h.service.UpdatePassword(user.ID, req.CurrentPassword, req.NewPassword, sessionContextFromRequest(r, rememberMe))
	if err != nil {
		if errors.Is(err, ErrInvalidPassword) {
			h.limiter.RecordFailure(time.Now(), keys...)
			http.Error(w, "Current password is incorrect", http.StatusUnauthorized)
			return
		}
		log.Printf("Failed to update password for user %d: %v", user.ID, err)
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}
	h.limiter.Reset(keys...)

	// The old session went with the old password; hand the caller the new one
	// so the tab they changed it from stays signed in.
	if sessionID != "" {
		setSessionCookie(w, r, sessionID, rememberMe)
	} else {
		clearSessionCookie(w, r)
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Printf("Failed to write update-password response: %v", err)
	}
}

// HandleRequestPasswordReset handles POST /api/auth/password/reset/request
// @Summary Request password reset
// @Description Generates a reset code and writes it to a file on the server.
// @Description Always answers 204, whether or not the account exists — the
// @Description endpoint is unauthenticated and must not confirm usernames.
// @Tags auth
// @Accept json
// @Param request body RequestPasswordResetRequest true "Username"
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Bad Request"
// @Failure 429 {string} string "Too Many Requests"
// @Router /auth/password/reset/request [post]
// @ID requestPasswordReset
func (h *Handler) HandleRequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RequestPasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	// Each request writes a file and rotates the account's live code, so an
	// anonymous caller gets a rate limit here too — in its own scope, so that
	// flooding it can't lock the account out of logging in.
	keys := rateLimitKeys(scopeResetRequest, r, req.Username)
	if !h.allowCredentialAttempt(w, keys) {
		return
	}
	h.limiter.RecordFailure(time.Now(), keys...)

	// The outcome is deliberately not reflected in the response: an unknown
	// username used to 500 while a known one 200'd, which told an anonymous
	// caller exactly which accounts exist. The old 200 body also disclosed the
	// server's absolute data directory.
	if err := h.service.RequestPasswordResetWithFile(req.Username); err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			log.Printf("Failed to generate reset code: %v", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleConfirmPasswordReset handles POST /api/auth/password/reset/confirm
// @Summary Confirm password reset
// @Description Validates a reset code and sets the new password. Every
// @Description session of the account is revoked.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ConfirmPasswordResetRequest true "Reset code"
// @Success 200 {object} ConfirmPasswordResetResponse
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized - Invalid or expired code"
// @Failure 429 {string} string "Too Many Requests"
// @Router /auth/password/reset/confirm [post]
// @ID confirmPasswordReset
func (h *Handler) HandleConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ConfirmPasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Code == "" {
		http.Error(w, "Reset code is required", http.StatusBadRequest)
		return
	}

	if err := ValidateResetCode(req.Code); err != nil {
		http.Error(w, "Invalid reset code format", http.StatusBadRequest)
		return
	}

	if req.NewPassword == "" {
		http.Error(w, "New password is required", http.StatusBadRequest)
		return
	}

	if err := ValidatePassword(req.NewPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// A reset code is 8 Base32 characters — guessable in bulk without a limit.
	keys := rateLimitKeys(scopeCredential, r, "")
	if !h.allowCredentialAttempt(w, keys) {
		return
	}

	user, err := h.service.ConfirmPasswordReset(req.Code, req.NewPassword)
	if err != nil {
		h.limiter.RecordFailure(time.Now(), keys...)
		if errors.Is(err, ErrInvalidResetCode) {
			http.Error(w, "Invalid or expired reset code", http.StatusUnauthorized)
			return
		}
		log.Printf("Failed to confirm password reset: %v", err)
		http.Error(w, "Failed to reset password", http.StatusInternalServerError)
		return
	}
	h.limiter.Reset(keys...)

	writeJSON(w, http.StatusOK, ConfirmPasswordResetResponse{User: user.ToResponse()})
}

// HandleVerifyPassword handles POST /api/auth/verify-password
// @Summary Verify the caller's password (sudo re-confirmation)
// @Description Re-checks the authenticated user's password without rotating
// @Description the session. Used by the sudo modal to gate sensitive ops:
// @Description on 204 the caller proceeds with the wrapped action; on 401
// @Description the modal stays open and shows an error.
// @Tags auth
// @Accept json
// @Param request body VerifyPasswordRequest true "Password to verify"
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 429 {string} string "Too Many Requests"
// @Router /auth/verify-password [post]
// @ID verifyPassword
func (h *Handler) HandleVerifyPassword(w http.ResponseWriter, r *http.Request, user *User) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	keys := rateLimitKeys(scopeCredential, r, user.Username)
	if !h.allowCredentialAttempt(w, keys) {
		return
	}

	// Match the same error mapping as Login: collapse "no such user" and
	// "wrong password" into a single 401 so the response shape doesn't leak
	// which one was incorrect.
	if _, err := h.service.Authenticate(user.Username, req.Password); err != nil {
		h.limiter.RecordFailure(time.Now(), keys...)
		http.Error(w, "Incorrect password", http.StatusUnauthorized)
		return
	}
	h.limiter.Reset(keys...)

	w.WriteHeader(http.StatusNoContent)
}

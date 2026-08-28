package auth

import (
	"errors"
	"log"
	"net/http"
	"time"
)

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
func (h *Handler) HandleListSessions(w http.ResponseWriter, r *http.Request, user *User) {
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

	writeJSON(w, http.StatusOK, out)
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
func (h *Handler) HandleRevokeSession(w http.ResponseWriter, r *http.Request, user *User) {
	publicID := r.PathValue("publicId")
	if publicID == "" {
		http.Error(w, "publicId is required", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteUserSessionByPublicID(user.ID, publicID); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
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
		clearSessionCookie(w, r)
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
func (h *Handler) HandleRevokeOtherSessions(w http.ResponseWriter, r *http.Request, user *User) {
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
func (h *Handler) HandleRevokeAllSessions(w http.ResponseWriter, r *http.Request, user *User) {
	if err := h.service.DeleteAllUserSessions(user.ID); err != nil {
		log.Printf("Revoke all sessions failed for user %d: %v", user.ID, err)
		http.Error(w, "Failed to revoke sessions", http.StatusInternalServerError)
		return
	}

	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

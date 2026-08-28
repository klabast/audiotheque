package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Handler struct {
	service *Service
	limiter *rateLimiter
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
		limiter: newRateLimiter(CredentialFailureLimit, CredentialIPFailureLimit, CredentialFailureWindow),
	}
}

// RegisterRoutes registers all auth routes on the given mux.
//
// Anything that grants privilege by itself — user management, revoking
// sessions — is registered behind RequireRealUser / RequireRealAdminUser,
// which ignore the auth-disabled toggle: turning auth off means nobody has to
// type a password to listen to music, not that any caller on the network may
// reprovision the server.
//
// The endpoints that carry their own knowledge factor (the caller must supply
// the account's current password) stay on RequireUser, so an instance with
// auth disabled can still change its password and re-enable auth. Those are
// rate limited instead — see CredentialFailureLimit.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/setup/required", h.HandleSetupRequired)
	mux.HandleFunc("POST /api/auth/setup", h.HandleSetup)
	mux.HandleFunc("POST /api/auth/login", h.HandleLogin)
	mux.HandleFunc("POST /api/auth/logout", h.HandleLogout)
	mux.HandleFunc("GET /api/auth/me", RequireUser(h.service, h.HandleMe))

	mux.HandleFunc("PUT /api/auth/password", RequireUser(h.service, h.HandleUpdatePassword))
	mux.HandleFunc("POST /api/auth/password/reset/request", h.HandleRequestPasswordReset)
	mux.HandleFunc("POST /api/auth/password/reset/confirm", h.HandleConfirmPasswordReset)
	mux.HandleFunc("POST /api/auth/verify-password", RequireUser(h.service, h.HandleVerifyPassword))

	// Active sessions (Settings → Security tab). Listing is read-only;
	// revoking is not.
	mux.HandleFunc("GET /api/auth/sessions", RequireUser(h.service, h.HandleListSessions))
	mux.HandleFunc("DELETE /api/auth/sessions/{publicId}", RequireRealUser(h.service, h.HandleRevokeSession))
	mux.HandleFunc("POST /api/auth/sessions/revoke-others", RequireRealUser(h.service, h.HandleRevokeOtherSessions))
	mux.HandleFunc("POST /api/auth/sessions/revoke-all", RequireRealUser(h.service, h.HandleRevokeAllSessions))

	// User management (Settings → Users tab). Real-admin-only across the board.
	mux.HandleFunc("GET /api/users", RequireRealAdminUser(h.service, h.HandleListUsers))
	mux.HandleFunc("POST /api/users", RequireRealAdminUser(h.service, h.HandleCreateUser))
	mux.HandleFunc("DELETE /api/users/{id}", RequireRealAdminUser(h.service, h.HandleDeleteUser))
	mux.HandleFunc("PUT /api/users/{id}/password", RequireRealAdminUser(h.service, h.HandleResetUserPassword))
}

// Rate-limit scopes. Buckets are namespaced per scope so that flooding one
// endpoint can't lock a user out of another — filling the reset-request
// bucket must not block that account's login.
const (
	scopeCredential   = "credential"
	scopeResetRequest = "reset-request"
	// scopeResetConfirm is separate from scopeCredential because it has no
	// username to key on, so its IP bucket is the only bound there is — it
	// keeps the tight limit rather than the widened shared-address one.
	scopeResetConfirm = "reset-confirm"
)

// rateLimitKeys builds the buckets an attempt is counted against: the
// caller's IP and, when known, the account being targeted.
func rateLimitKeys(scope string, r *http.Request, username string) []string {
	keys := []string{scope + "|ip:" + clientIP(r)}
	if username != "" {
		keys = append(keys, scope+"|user:"+username)
	}
	return keys
}

// allowCredentialAttempt reports whether a credential attempt may proceed,
// writing 429 when it may not.
func (h *Handler) allowCredentialAttempt(w http.ResponseWriter, keys []string) bool {
	if h.limiter.Allow(time.Now(), keys...) {
		return true
	}
	http.Error(w, "Too many attempts, try again later", http.StatusTooManyRequests)
	return false
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

package auth

import (
	"errors"
	"log"
	"net/http"
	"strings"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden: admin access required")
)

// clientIP extracts the best-effort caller IP from an HTTP request. Trusts
// the first hop of X-Forwarded-For when present (reverse-proxy deployments),
// falling back to r.RemoteAddr otherwise. Returned as a bare host string
// where possible (port stripped for direct connections).
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if comma := strings.Index(forwarded, ","); comma >= 0 {
			return strings.TrimSpace(forwarded[:comma])
		}
		return strings.TrimSpace(forwarded)
	}
	// r.RemoteAddr is "ip:port" for TCP; strip the port for storage parity
	// with the X-Forwarded-For path above.
	if colon := strings.LastIndex(r.RemoteAddr, ":"); colon > 0 {
		return r.RemoteAddr[:colon]
	}
	return r.RemoteAddr
}

// GetAuthenticatedUser resolves the audiod_token cookie to a user via the
// DB-backed session table and returns the user. Sliding renewal fires
// internally — see Service.ValidateSession — but no Set-Cookie is written,
// so the browser's cookie expiry can drift from the bumped DB row.
//
// HTTP handlers that own a ResponseWriter should call AuthenticateRequest
// instead, which re-issues the cookie so renewal is visible client-side.
// This shorter form stays for the WebSocket upgrade path (which only owns
// the request) and any caller that explicitly doesn't want cookie writes.
//
// When auth is disabled (Service.AuthEnabled() == false) the cookie is
// ignored entirely and the canonical admin is returned — see auth-disabled
// mode in §7 of the auth-rework roadmap.
func GetAuthenticatedUser(r *http.Request, service *Service) (*User, error) {
	if !service.AuthEnabled() {
		return service.GetCanonicalAdmin()
	}
	user, _, err := getAuthenticatedUserAndSession(r, service)
	return user, err
}

// AuthenticateRequest is the same as GetAuthenticatedUser but also re-issues
// the audiod_token cookie on success, so the browser's Max-Age tracks the
// session's (possibly bumped) expires_at. HTTP handlers should prefer this
// over GetAuthenticatedUser.
//
// In auth-disabled mode there is no session to renew, so no Set-Cookie is
// written — the canonical admin is returned as-is.
func AuthenticateRequest(w http.ResponseWriter, r *http.Request, service *Service) (*User, error) {
	if !service.AuthEnabled() {
		return service.GetCanonicalAdmin()
	}
	user, sess, err := getAuthenticatedUserAndSession(r, service)
	if err != nil {
		return nil, err
	}
	setSessionCookie(w, sess.ID, sess.RememberMe)
	return user, nil
}

func getAuthenticatedUserAndSession(r *http.Request, service *Service) (*User, *Session, error) {
	cookie, err := r.Cookie("audiod_token")
	if err != nil {
		return nil, nil, ErrUnauthorized
	}

	user, sess, err := service.ValidateSession(cookie.Value, clientIP(r))
	if err != nil {
		// Unknown/expired sessions are the common case; only log the noisy
		// ones (e.g. DB errors) and return a clean Unauthorized.
		if !errors.Is(err, ErrSessionNotFound) {
			log.Printf("Session validation error: %v", err)
		}
		return nil, nil, ErrUnauthorized
	}

	return user, sess, nil
}

// RequireAdmin checks if the authenticated user is an admin.
// Returns ErrForbidden if the user is not an admin.
func RequireAdmin(user *User) error {
	if !user.IsAdmin {
		return ErrForbidden
	}
	return nil
}
